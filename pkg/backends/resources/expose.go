/*
Copyright (C) GRyCAP - I3M - UPV

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	htpasswd "github.com/foomo/htpasswd"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	apps "k8s.io/api/apps/v1"
	autos "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	net "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	typeNodePort       = "NodePort"
	typeClusterIP      = "ClusterIP"
	prefixLabelApp     = "oscar-svc-exp-"
	KeyLabelApp        = "app"
	podPortName        = "podport"
	servicePortName    = "serviceport"
	servicePortNumber  = 80
	routeKindIngress   = "ingress"
	routeKindHTTPRoute = "httproute"
)

var httpRouteGVR = schema.GroupVersionResource{
	Group:    "gateway.networking.k8s.io",
	Version:  "v1",
	Resource: "httproutes",
}

var traefikMiddlewareGVR = schema.GroupVersionResource{
	Group:    "traefik.io",
	Version:  "v1alpha1",
	Resource: "middlewares",
}

var gatewayClientsetProvider = getGatewayClientset

// Custom logger
var ExposeLogger = log.New(os.Stdout, "[EXPOSED-SERVICE] ", log.Flags())

/* Exposed service is composed by a deployment and a service.
An exposed service can be of to types:
- NodePort
- Ingress */

// CreateExpose creates all the kubernetes components
func CreateExpose(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	//ExposeLogger.Printf("Creating exposed service: \n%v\n", service)
	targetNamespace := namespace
	if targetNamespace == "" {
		targetNamespace = cfg.ServicesNamespace
	}

	err := createDeployment(service, targetNamespace, kubeClientset, cfg)
	if err != nil {
		return fmt.Errorf("error creating deployment for exposed service '%s': %v", service.Name, err)
	}
	err = createService(service, targetNamespace, kubeClientset, cfg)
	if err != nil {
		return fmt.Errorf("error creating svc for exposed service '%s': %v", service.Name, err)
	}
	if service.Expose.NodePort == 0 {
		err = createRoute(service, targetNamespace, kubeClientset, cfg)
		if err != nil {
			return fmt.Errorf("error creating route for exposed service '%s': %v", service.Name, err)
		}
	}
	return nil
}

// DeleteExpose removes all the components of the exposed service from the cluster
func DeleteExpose(name string, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	targetNamespace := namespace
	if targetNamespace == "" {
		targetNamespace = cfg.ServicesNamespace
	}

	err := deleteDeployment(name, targetNamespace, kubeClientset, cfg)
	if err = ignoreNotFound(err); err != nil {
		return fmt.Errorf("error deleting deployment for exposed service '%s': %v", name, err)
	}
	err = deleteService(name, targetNamespace, kubeClientset, cfg)
	if err = ignoreNotFound(err); err != nil {
		return fmt.Errorf("error deleting service for exposed service '%s': %v", name, err)
	}

	if err := deleteRouteResources(name, targetNamespace, kubeClientset, cfg); err != nil {
		return fmt.Errorf("error deleting route for exposed service '%s': %v", name, err)
	}
	termination := int64(0)
	foreground := metav1.DeletePropagationForeground
	delete := metav1.DeleteOptions{
		GracePeriodSeconds: &termination,
		PropagationPolicy:  &foreground,
	}
	listOpts := metav1.ListOptions{
		LabelSelector: KeyLabelApp + "=" + GetKeyLabelApp(name),
	}
	err = kubeClientset.CoreV1().Pods(targetNamespace).DeleteCollection(context.TODO(), delete, listOpts)
	if err != nil {
		return fmt.Errorf("error deleting pods of exposed service '%s': %v", name, err)
	}
	utils.DeleteWorkload(name, targetNamespace, cfg)

	return nil
}

func ignoreNotFound(err error) error {
	if err == nil || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// UpdateExpose updates all the components of the exposed service on the cluster
func UpdateExpose(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	targetNamespace := namespace
	if targetNamespace == "" {
		targetNamespace = cfg.ServicesNamespace
	}

	// If the deployment exist, and keep continue it will upload.
	err := updateDeployment(service, targetNamespace, kubeClientset, cfg)
	if err != nil {
		ExposeLogger.Printf("error updating exposed service deployment: %v\n", err)
		return err
	}
	err = updateService(service, targetNamespace, kubeClientset, cfg)
	if err != nil {
		ExposeLogger.Printf("error updating exposed service: %v\n", err)
		return err
	}

	if service.Expose.NodePort != 0 {
		if err = deleteRouteResources(service.Name, targetNamespace, kubeClientset, cfg); err != nil {
			log.Printf("error deleting route service: %v\n", err)
			return err
		}
	} else {
		if err = upsertRoute(service, targetNamespace, kubeClientset, cfg); err != nil {
			log.Printf("error updating route service: %v\n", err)
			return err
		}
	}

	utils.UpdateWorkload(service, targetNamespace, cfg, getPodTemplateSpec)
	if cfg.KueueEnable {
		err = utils.CheckWorkloadAdmited(service, namespace, cfg, kubeClientset, getDeploymentSpec)
		if err != nil {
			return fmt.Errorf("Invalid workload after update: Error checking workload admission: change the cpu/memory requests")
		}
	}
	return nil
}

func getRouteKind(cfg *types.Config) string {
	if cfg == nil {
		return routeKindIngress
	}

	routeKind := strings.ToLower(strings.TrimSpace(cfg.ExposedServicesRouteKind))
	if routeKind == routeKindHTTPRoute {
		return routeKindHTTPRoute
	}

	return routeKindIngress
}

func createRoute(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	if getRouteKind(cfg) == routeKindHTTPRoute {
		return createHTTPRoute(service, namespace, kubeClientset, cfg)
	}

	return createIngress(service, namespace, kubeClientset, cfg)
}

func upsertRoute(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	ingressExists := existsIngress(service.Name, namespace, kubeClientset)
	httpRouteExists := existsHTTPRoute(service.Name, namespace)

	if getRouteKind(cfg) == routeKindHTTPRoute {
		if ingressExists {
			if err := deleteIngress(getIngressName(service.Name), namespace, kubeClientset, cfg); err != nil {
				return err
			}
		}
		if existsSecret(service.Name, namespace, kubeClientset, cfg) {
			if err := deleteSecret(service.Name, namespace, kubeClientset, cfg); err != nil {
				return err
			}
		}

		if httpRouteExists {
			return updateHTTPRoute(service, namespace, kubeClientset, cfg)
		}
		return createHTTPRoute(service, namespace, kubeClientset, cfg)
	}

	if httpRouteExists {
		if err := deleteHTTPRoute(getHTTPRouteName(service.Name), namespace); err != nil {
			return err
		}
	}

	if existsTraefikCORSMiddleware(service.Name, namespace) {
		if err := deleteTraefikCORSMiddleware(getTraefikCORSMiddlewareName(service.Name), namespace); err != nil {
			return err
		}
	}

	if ingressExists {
		return updateIngress(service, namespace, kubeClientset, cfg)
	}

	return createIngress(service, namespace, kubeClientset, cfg)
}

func deleteRouteResources(serviceName string, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	if existsIngress(serviceName, namespace, kubeClientset) {
		if err := deleteIngress(getIngressName(serviceName), namespace, kubeClientset, cfg); err != nil {
			return err
		}
	}

	if existsHTTPRoute(serviceName, namespace) {
		if err := deleteHTTPRoute(getHTTPRouteName(serviceName), namespace); err != nil {
			return err
		}
	}

	if existsTraefikCORSMiddleware(serviceName, namespace) {
		if err := deleteTraefikCORSMiddleware(getTraefikCORSMiddlewareName(serviceName), namespace); err != nil {
			return err
		}
	}

	if existsTraefikAuthMiddleware(serviceName, namespace) {
		if err := deleteTraefikAuthMiddleware(getTraefikAuthMiddlewareName(serviceName), namespace); err != nil {
			return err
		}
	}

	if existsTraefikAuthSecret(serviceName, namespace, kubeClientset) {
		if err := deleteTraefikAuthSecret(getTraefikAuthSecretName(serviceName), namespace, kubeClientset); err != nil {
			return err
		}
	}

	if existsSecret(serviceName, namespace, kubeClientset, cfg) {
		err := deleteSecret(serviceName, namespace, kubeClientset, cfg)
		if err = ignoreNotFound(err); err != nil {
			return err
		}
	}

	return nil
}

func getGatewayClientset() (dynamic.Interface, error) {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	return dynamic.NewForConfig(restCfg)
}

func validateHTTPRouteConfig(service types.Service, cfg *types.Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if strings.TrimSpace(cfg.HTTPRouteGatewayName) == "" {
		return fmt.Errorf("HTTPROUTE_GATEWAY_NAME must be defined when EXPOSED_SERVICES_ROUTE_KIND=httproute")
	}

	return nil
}

// TODO check and refactor
// Main function that list all the kubernetes components
// This function is not used, in the future could be useful
/*func ListExpose(kubeClientset kubernetes.Interface, cfg *types.Config) error {
	targetNamespace := cfg.ServicesNamespace
	deploy, hpa, err := listDeployments(targetNamespace, kubeClientset, cfg)

	services, err2 := listServices(targetNamespace, kubeClientset, cfg)
	ingress, err3 := listIngress(targetNamespace, kubeClientset, cfg)
	if err != nil {
		ExposeLogger.Printf("WARNING: %v\n", err)
		return err
	}
	if err2 != nil {
		ExposeLogger.Printf("WARNING: %v\n", err2)
		return err2
	}
	if err3 != nil {
		ExposeLogger.Printf("WARNING: %v\n", err3)
		return err3
	}
	fmt.Println(deploy, hpa, services, ingress)
	return nil

}*/

//////////// Deployment

/// Create deployment and horizontal autoscale

func createDeployment(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	utils.CreateWorkload(service, namespace, cfg, getPodTemplateSpec)
	deployment := getDeploymentSpec(service, namespace, cfg)
	if utils.SecretExists(service.Name, namespace, kubeClientset) {
		deployment.Spec.Template.Spec.Containers[0].EnvFrom = []v1.EnvFromSource{
			{
				SecretRef: &v1.SecretEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: service.Name,
					},
				},
			},
		}
	}
	_, err := kubeClientset.AppsV1().Deployments(namespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	hpa := getHortizontalAutoScaleSpec(service, namespace, cfg)
	_, err = kubeClientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).Create(context.TODO(), hpa, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	if cfg.KueueEnable {
		err = utils.CheckWorkloadAdmited(service, namespace, cfg, kubeClientset, getDeploymentSpec)
		if err != nil {
			if err := utils.DeleteKueueLocalQueue(context.TODO(), cfg, service.Namespace, service.Name); err != nil {
				ExposeLogger.Printf("Error deleting Kueue local queue: %v", err)
			}
			ExposeLogger.Printf("Error checking workload admission: change the cpu/memory requests\n")
			return err
		}
	}

	return nil
}

// Return the component deployment, ready to create or update
func getDeploymentSpec(service types.Service, namespace string, cfg *types.Config) *apps.Deployment {
	deployName := GetDeploymentName(service.Name)
	minScale := int32(0)
	if service.Owner == types.DefaultOwner || !cfg.KueueEnable {
		minScale = int32(service.Expose.MinScale)
	}
	uid := auth.FormatUID(service.Owner)
	if len(uid) > 62 {
		uid = uid[:62]
	}
	deployment := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployName,
			Namespace: namespace,
			Labels: map[string]string{
				types.KueueOwnerLabel: uid,
			},
		},
		Spec: apps.DeploymentSpec{
			Replicas: &minScale,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					KeyLabelApp: GetKeyLabelApp(service.Name),
				},
			},
			Template: getPodTemplateSpec(service, namespace, cfg),
		},
		Status: apps.DeploymentStatus{},
	}
	if service.Owner != types.DefaultOwner && cfg.KueueEnable {
		deployment.Spec.Template.ObjectMeta.Labels[types.KueueOwnerLabel] = uid
		deployment.Spec.Template.ObjectMeta.Labels["kueue.x-k8s.io/queue-name"] = utils.BuildLocalQueueName(service.Name)
		deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{
			"kueue.x-k8s.io/queue-name": utils.BuildLocalQueueName(service.Name),
		}
		deployment.ObjectMeta.Labels["kueue.x-k8s.io/queue-name"] = utils.BuildLocalQueueName(service.Name)
		deployment.ObjectMeta.Annotations = map[string]string{
			"kueue.x-k8s.io/queue-name": utils.BuildLocalQueueName(service.Name),
		}
	}

	return deployment
}

// Return the component HorizontalAutoScale, ready to create or update
func getHortizontalAutoScaleSpec(service types.Service, namespace string, cfg *types.Config) *autos.HorizontalPodAutoscaler {
	hpaName := getHPAName(service.Name)
	deployName := GetDeploymentName(service.Name)
	hpa := &autos.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hpaName,
			Namespace: namespace,
		},
		Spec: autos.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autos.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       deployName,
				APIVersion: "apps/v1",
			},
			MinReplicas:                    &service.Expose.MinScale,
			MaxReplicas:                    service.Expose.MaxScale,
			TargetCPUUtilizationPercentage: &service.Expose.CpuThreshold,
		},
		Status: autos.HorizontalPodAutoscalerStatus{},
	}
	return hpa
}

// Return the Pod spec inside of deployment, ready to create or update

func getPodTemplateSpec(service types.Service, namespace string, cfg *types.Config) v1.PodTemplateSpec {
	podSpec, _ := service.ToPodSpec(cfg)

	for i := range podSpec.Containers {
		podSpec.Containers[i].Ports = []v1.ContainerPort{
			{
				Name:          podPortName,
				ContainerPort: int32(service.Expose.APIPort), // #nosec G115
			},
		}
		podSpec.Containers[i].VolumeMounts[0].ReadOnly = false
		if service.Expose.DefaultCommand {
			podSpec.Containers[i].Command = nil
			podSpec.Containers[i].Args = nil
		} else {
			podSpec.Containers[i].Command = []string{"/bin/sh"}
			podSpec.Containers[i].Args = []string{"-c", fmt.Sprintf("%s/%s", types.ConfigPath, types.ScriptFileName)}
		}

		probePath := service.Expose.HealthPath
		probePath = getProbePath(service)

		probeHandler := v1.ProbeHandler{
			HTTPGet: &v1.HTTPGetAction{
				Path: probePath,
				Port: intstr.FromString(podPortName),
			},
		}

		podSpec.Containers[i].LivenessProbe = &v1.Probe{
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
			ProbeHandler:        probeHandler,
			TimeoutSeconds:      2,
		}
		podSpec.Containers[i].ReadinessProbe = &v1.Probe{
			InitialDelaySeconds: 10,
			PeriodSeconds:       5,
			ProbeHandler:        probeHandler,
			TimeoutSeconds:      2,
		}
	}
	var num int32 = 0777
	podSpec.Volumes[0].VolumeSource.ConfigMap.DefaultMode = &num
	if service.Mount.Provider != "" {
		SetMount(podSpec, service, cfg)
	}
	template := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: namespace,
			Labels: map[string]string{
				types.OscarUserServiceLabel: "true",
				KeyLabelApp:                 GetKeyLabelApp(service.Name),
			},
		},
		Spec: *podSpec,
	}
	return template
}

// / List deployment and the horizontal auto scale
func listDeployments(namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) (*apps.DeploymentList, *autos.HorizontalPodAutoscalerList, error) {
	deployment, err := kubeClientset.AppsV1().Deployments(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	hpa, err2 := kubeClientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(context.TODO(), metav1.ListOptions{})
	if err2 != nil {
		return nil, nil, err2
	}
	return deployment, hpa, nil
}

// Delete Deployment and HPA
func deleteDeployment(name string, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	name_hpa := getHPAName(name)
	err := kubeClientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).Delete(context.TODO(), name_hpa, metav1.DeleteOptions{})
	if err = ignoreNotFound(err); err != nil {
		return err
	}
	deployment := GetDeploymentName(name)
	err = kubeClientset.AppsV1().Deployments(namespace).Delete(context.TODO(), deployment, metav1.DeleteOptions{})
	if err = ignoreNotFound(err); err != nil {
		return err
	}
	return nil
}

///Update Deployment and HPA

func updateDeployment(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	_, err := kubeClientset.AppsV1().Deployments(namespace).Get(context.TODO(), GetDeploymentName(service.Name), metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment := getDeploymentSpec(service, namespace, cfg)
	if utils.SecretExists(service.Name, namespace, kubeClientset) {
		deployment.Spec.Template.Spec.Containers[0].EnvFrom = []v1.EnvFromSource{
			{
				SecretRef: &v1.SecretEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: service.Name,
					},
				},
			},
		}
	}
	_, err = kubeClientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	_, err = kubeClientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).Get(context.TODO(), getHPAName(service.Name), metav1.GetOptions{})
	if err != nil {
		return err
	}
	hpa := getHortizontalAutoScaleSpec(service, namespace, cfg)
	_, err = kubeClientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).Update(context.TODO(), hpa, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

/////////// Service

// Create a kubernetes service component
func createService(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	service_spec := getServiceSpec(service, namespace, cfg)
	_, err := kubeClientset.CoreV1().Services(namespace).Create(context.TODO(), service_spec, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Return a kubernetes service component, ready to deploy or update
func getServiceSpec(service types.Service, namespace string, cfg *types.Config) *v1.Service {
	name_service := getServiceName(service.Name)
	var port v1.ServicePort = v1.ServicePort{
		Name: servicePortName,
		Port: servicePortNumber,
		TargetPort: intstr.IntOrString{
			Type:   0,
			IntVal: int32(service.Expose.APIPort), // #nosec G115
		},
	}
	service_type := v1.ServiceType(typeClusterIP)
	if service.Expose.NodePort != 0 {
		service_type = typeNodePort
		port.NodePort = service.Expose.NodePort
	}
	service_spec := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name_service,
			Namespace: namespace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{port},
			Type:  service_type,
			Selector: map[string]string{
				KeyLabelApp: GetKeyLabelApp(service.Name),
			},
		},
		Status: v1.ServiceStatus{},
	}
	return service_spec
}

/// List services in a certain namespace

func listServices(namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) (*v1.ServiceList, error) {
	services, err := kubeClientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return services, nil
}

// / Update a kubernete service
func updateService(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	kube_service := getServiceSpec(service, namespace, cfg)
	_, err := kubeClientset.CoreV1().Services(namespace).Update(context.TODO(), kube_service, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

/// Delete kubernetes service

func deleteService(name string, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	kube_service := getServiceName(name)
	err := kubeClientset.CoreV1().Services(namespace).Delete(context.TODO(), kube_service, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

/////////// Ingress

// / Create an ingress component
func createIngress(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	// Create Secret

	ingress := getIngressSpec(service, namespace, cfg)
	_, err := kubeClientset.NetworkingV1().Ingresses(namespace).Create(context.TODO(), ingress, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	if service.Expose.SetAuth {
		cerr := createSecret(service, namespace, kubeClientset, cfg)
		if cerr != nil {
			return cerr
		}
	}
	return nil
}

// / Update a kubernete service
func updateIngress(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {

	serviceName := service.Name
	//if exist continue and need -> Update
	//if exist and not need -> delete
	//if not  exist create
	kube_ingress := getIngressSpec(service, namespace, cfg)
	_, err := kubeClientset.NetworkingV1().Ingresses(namespace).Update(context.TODO(), kube_ingress, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	secret := existsSecret(serviceName, namespace, kubeClientset, cfg)
	if secret {
		if service.Expose.SetAuth {
			uerr := updateSecret(service, namespace, kubeClientset, cfg)
			if uerr != nil {
				return uerr
			}
		} else {
			derr := deleteSecret(service.Name, namespace, kubeClientset, cfg)
			if derr != nil {
				return derr
			}
		}
	} else {
		if service.Expose.SetAuth {
			cerr := createSecret(service, namespace, kubeClientset, cfg)
			if cerr != nil {
				return cerr
			}
		}
	}

	return nil
}

func createHTTPRoute(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	if err := validateHTTPRouteConfig(service, cfg); err != nil {
		return err
	}

	if err := createTraefikCORSMiddleware(service, namespace, cfg); err != nil {
		return err
	}

	if service.Expose.SetAuth {
		if err := createTraefikAuthSecret(service, namespace, kubeClientset); err != nil {
			return err
		}
		if err := createTraefikAuthMiddleware(service, namespace); err != nil {
			return err
		}
	}

	gatewayClientset, err := gatewayClientsetProvider()
	if err != nil {
		return err
	}

	httpRoute := getHTTPRouteSpec(service, namespace, cfg)
	_, err = gatewayClientset.Resource(httpRouteGVR).Namespace(namespace).Create(context.TODO(), httpRoute, metav1.CreateOptions{})
	return err
}

func updateHTTPRoute(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	if err := validateHTTPRouteConfig(service, cfg); err != nil {
		return err
	}

	if err := upsertTraefikCORSMiddleware(service, namespace, cfg); err != nil {
		return err
	}

	if service.Expose.SetAuth {
		if err := upsertTraefikAuthSecret(service, namespace, kubeClientset); err != nil {
			return err
		}
		if err := upsertTraefikAuthMiddleware(service, namespace); err != nil {
			return err
		}
	} else {
		if existsTraefikAuthMiddleware(service.Name, namespace) {
			if err := deleteTraefikAuthMiddleware(getTraefikAuthMiddlewareName(service.Name), namespace); err != nil {
				return err
			}
		}
		if existsTraefikAuthSecret(service.Name, namespace, kubeClientset) {
			if err := deleteTraefikAuthSecret(getTraefikAuthSecretName(service.Name), namespace, kubeClientset); err != nil {
				return err
			}
		}
	}

	gatewayClientset, err := gatewayClientsetProvider()
	if err != nil {
		return err
	}

	httpRoute := getHTTPRouteSpec(service, namespace, cfg)
	_, err = gatewayClientset.Resource(httpRouteGVR).Namespace(namespace).Update(context.TODO(), httpRoute, metav1.UpdateOptions{})
	return err
}

func getHTTPRouteSpec(service types.Service, namespace string, cfg *types.Config) *unstructured.Unstructured {
	nameHTTPRoute := getHTTPRouteName(service.Name)
	nameService := getServiceName(service.Name)
	pathAPI := getAPIPath(service.Name)

	rule := map[string]any{
		"matches": []any{
			map[string]any{
				"path": map[string]any{
					"type":  "PathPrefix",
					"value": pathAPI,
				},
			},
		},
		"backendRefs": []any{
			map[string]any{
				"name": nameService,
				"port": int64(servicePortNumber),
			},
		},
	}

	if !service.Expose.RewriteTarget {
		rule["filters"] = []any{
			map[string]any{
				"type": "URLRewrite",
				"urlRewrite": map[string]any{
					"path": map[string]any{
						"type":               "ReplacePrefixMatch",
						"replacePrefixMatch": "/",
					},
				},
			},
			map[string]any{
				"type": "ExtensionRef",
				"extensionRef": map[string]any{
					"group": "traefik.io",
					"kind":  "Middleware",
					"name":  getTraefikCORSMiddlewareName(service.Name),
				},
			},
		}
	} else {
		rule["filters"] = []any{
			map[string]any{
				"type": "ExtensionRef",
				"extensionRef": map[string]any{
					"group": "traefik.io",
					"kind":  "Middleware",
					"name":  getTraefikCORSMiddlewareName(service.Name),
				},
			},
		}
	}

	if service.Expose.SetAuth {
		filters, _ := rule["filters"].([]any)
		filters = append(filters, map[string]any{
			"type": "ExtensionRef",
			"extensionRef": map[string]any{
				"group": "traefik.io",
				"kind":  "Middleware",
				"name":  getTraefikAuthMiddlewareName(service.Name),
			},
		})
		rule["filters"] = filters
	}

	spec := map[string]any{
		"rules": []any{rule},
	}

	host := strings.TrimSpace(cfg.IngressHost)
	if host != "" {
		spec["hostnames"] = []any{host}
	}

	parentRef := map[string]any{
		"group": "gateway.networking.k8s.io",
		"kind":  "Gateway",
		"name":  strings.TrimSpace(cfg.HTTPRouteGatewayName),
	}

	if gatewayNamespace := strings.TrimSpace(cfg.HTTPRouteGatewayNamespace); gatewayNamespace != "" {
		parentRef["namespace"] = gatewayNamespace
	}

	spec["parentRefs"] = []any{parentRef}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]any{
			"name":      nameHTTPRoute,
			"namespace": namespace,
		},
		"spec": spec,
	}}
}

// Return a kubernetes ingress component, ready to deploy or update
func getIngressSpec(service types.Service, namespace string, cfg *types.Config) *net.Ingress {
	name_ingress := getIngressName(service.Name)
	pathofapi := getAPIPath(service.Name)
	name_service := getServiceName(service.Name)
	var ptype net.PathType = "ImplementationSpecific"
	var ingresspath net.HTTPIngressPath = net.HTTPIngressPath{
		Path:     pathofapi + "/?(.*)",
		PathType: &ptype,
		Backend: net.IngressBackend{
			Service: &net.IngressServiceBackend{
				Name: name_service,
				Port: net.ServiceBackendPort{
					Number: servicePortNumber,
				},
			},
		},
	}
	var ingresssrulevalue net.HTTPIngressRuleValue = net.HTTPIngressRuleValue{
		Paths: []net.HTTPIngressPath{ingresspath},
	}
	var rule net.IngressRule
	var tls net.IngressTLS
	var specification net.IngressSpec

	var host string = cfg.IngressHost

	if host == "" {
		rule = net.IngressRule{
			IngressRuleValue: net.IngressRuleValue{HTTP: &ingresssrulevalue},
		}
		specification = net.IngressSpec{
			TLS:   []net.IngressTLS{},
			Rules: []net.IngressRule{rule}, //IngressClassName:
		}
	} else {
		rule = net.IngressRule{
			Host:             host,
			IngressRuleValue: net.IngressRuleValue{HTTP: &ingresssrulevalue},
		}
		tls = net.IngressTLS{
			Hosts:      []string{host},
			SecretName: host,
		}
		ingressClassName := "nginx"
		specification = net.IngressSpec{
			IngressClassName: &ingressClassName,
			TLS:              []net.IngressTLS{tls},
			Rules:            []net.IngressRule{rule}, //IngressClassName:
		}
	}

	rewriteOption := "/$1"
	if service.Expose.RewriteTarget {
		rewriteOption = pathofapi + "/$1"
	}
	annotation := map[string]string{
		"nginx.ingress.kubernetes.io/rewrite-target":     rewriteOption,
		"spec.ingressClassName":                          "nginx",
		"nginx.ingress.kubernetes.io/use-regex":          "true",
		"nginx.ingress.kubernetes.io/enable-cors":        "true",
		"nginx.ingress.kubernetes.io/cors-allow-origin":  cfg.IngressServicesCORSAllowedOrigins,
		"nginx.ingress.kubernetes.io/cors-allow-methods": cfg.IngressServicesCORSAllowedMethods,
		"nginx.ingress.kubernetes.io/cors-allow-headers": cfg.IngressServicesCORSAllowedHeaders,
	}
	if service.Expose.SetAuth {
		annotation["nginx.ingress.kubernetes.io/auth-type"] = "basic"
		annotation["nginx.ingress.kubernetes.io/auth-secret"] = getSecretName(service.Name)
		annotation["nginx.ingress.kubernetes.io/auth-realm"] = "Authentication Required"
	}
	ingress := &net.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name_ingress,
			Namespace:   namespace,
			Annotations: annotation,
		},
		Spec:   specification,
		Status: net.IngressStatus{},
	}
	return ingress
}

/// List the kuberntes ingress

func listIngress(namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) (*net.IngressList, error) {
	ingress, err := kubeClientset.NetworkingV1().Ingresses(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return ingress, nil
}

// Delete a kubernetes ingress
func deleteIngress(name string, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	// if secret exist, delete
	err := kubeClientset.NetworkingV1().Ingresses(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func deleteHTTPRoute(name string, namespace string) error {
	gatewayClientset, err := gatewayClientsetProvider()
	if err != nil {
		return err
	}

	return gatewayClientset.Resource(httpRouteGVR).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func createTraefikCORSMiddleware(service types.Service, namespace string, cfg *types.Config) error {
	gatewayClientset, err := gatewayClientsetProvider()
	if err != nil {
		return err
	}

	middleware := getTraefikCORSMiddlewareSpec(service, namespace, cfg)
	_, err = gatewayClientset.Resource(traefikMiddlewareGVR).Namespace(namespace).Create(context.TODO(), middleware, metav1.CreateOptions{})
	return err
}

func updateTraefikCORSMiddleware(service types.Service, namespace string, cfg *types.Config) error {
	gatewayClientset, err := gatewayClientsetProvider()
	if err != nil {
		return err
	}

	middleware := getTraefikCORSMiddlewareSpec(service, namespace, cfg)
	_, err = gatewayClientset.Resource(traefikMiddlewareGVR).Namespace(namespace).Update(context.TODO(), middleware, metav1.UpdateOptions{})
	return err
}

func upsertTraefikCORSMiddleware(service types.Service, namespace string, cfg *types.Config) error {
	if existsTraefikCORSMiddleware(service.Name, namespace) {
		return updateTraefikCORSMiddleware(service, namespace, cfg)
	}

	return createTraefikCORSMiddleware(service, namespace, cfg)
}

func createTraefikAuthMiddleware(service types.Service, namespace string) error {
	gatewayClientset, err := gatewayClientsetProvider()
	if err != nil {
		return err
	}

	middleware := getTraefikAuthMiddlewareSpec(service, namespace)
	_, err = gatewayClientset.Resource(traefikMiddlewareGVR).Namespace(namespace).Create(context.TODO(), middleware, metav1.CreateOptions{})
	return err
}

func updateTraefikAuthMiddleware(service types.Service, namespace string) error {
	gatewayClientset, err := gatewayClientsetProvider()
	if err != nil {
		return err
	}

	middleware := getTraefikAuthMiddlewareSpec(service, namespace)
	_, err = gatewayClientset.Resource(traefikMiddlewareGVR).Namespace(namespace).Update(context.TODO(), middleware, metav1.UpdateOptions{})
	return err
}

func upsertTraefikAuthMiddleware(service types.Service, namespace string) error {
	if existsTraefikAuthMiddleware(service.Name, namespace) {
		return updateTraefikAuthMiddleware(service, namespace)
	}

	return createTraefikAuthMiddleware(service, namespace)
}

func getTraefikCORSMiddlewareSpec(service types.Service, namespace string, cfg *types.Config) *unstructured.Unstructured {
	nameMiddleware := getTraefikCORSMiddlewareName(service.Name)

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "traefik.io/v1alpha1",
		"kind":       "Middleware",
		"metadata": map[string]any{
			"name":      nameMiddleware,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"headers": map[string]any{
				"accessControlAllowOriginList": splitCSVAny(cfg.IngressServicesCORSAllowedOrigins),
				"accessControlAllowMethods":    splitCSVAny(cfg.IngressServicesCORSAllowedMethods),
				"accessControlAllowHeaders":    splitCSVAny(cfg.IngressServicesCORSAllowedHeaders),
				"addVaryHeader":                true,
			},
		},
	}}
}

func getTraefikAuthMiddlewareSpec(service types.Service, namespace string) *unstructured.Unstructured {
	nameMiddleware := getTraefikAuthMiddlewareName(service.Name)

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "traefik.io/v1alpha1",
		"kind":       "Middleware",
		"metadata": map[string]any{
			"name":      nameMiddleware,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"basicAuth": map[string]any{
				"secret": getTraefikAuthSecretName(service.Name),
			},
		},
	}}
}

func deleteTraefikCORSMiddleware(name string, namespace string) error {
	gatewayClientset, err := gatewayClientsetProvider()
	if err != nil {
		return err
	}

	return gatewayClientset.Resource(traefikMiddlewareGVR).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func existsTraefikCORSMiddleware(serviceName string, namespace string) bool {
	gatewayClientset, err := gatewayClientsetProvider()
	if err != nil {
		return false
	}

	_, err = gatewayClientset.Resource(traefikMiddlewareGVR).Namespace(namespace).Get(context.TODO(), getTraefikCORSMiddlewareName(serviceName), metav1.GetOptions{})
	return err == nil
}

func deleteTraefikAuthMiddleware(name string, namespace string) error {
	gatewayClientset, err := gatewayClientsetProvider()
	if err != nil {
		return err
	}

	return gatewayClientset.Resource(traefikMiddlewareGVR).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func existsTraefikAuthMiddleware(serviceName string, namespace string) bool {
	gatewayClientset, err := gatewayClientsetProvider()
	if err != nil {
		return false
	}

	_, err = gatewayClientset.Resource(traefikMiddlewareGVR).Namespace(namespace).Get(context.TODO(), getTraefikAuthMiddlewareName(serviceName), metav1.GetOptions{})
	return err == nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitCSVAny(value string) []any {
	parts := splitCSV(value)
	result := make([]any, 0, len(parts))
	for _, part := range parts {
		result = append(result, part)
	}
	return result
}

func createTraefikAuthSecret(service types.Service, namespace string, kubeClientset kubernetes.Interface) error {
	secret := getTraefikAuthSecretSpec(service, namespace)
	_, err := kubeClientset.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	return err
}

func updateTraefikAuthSecret(service types.Service, namespace string, kubeClientset kubernetes.Interface) error {
	secret := getTraefikAuthSecretSpec(service, namespace)
	_, err := kubeClientset.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	return err
}

func upsertTraefikAuthSecret(service types.Service, namespace string, kubeClientset kubernetes.Interface) error {
	if existsTraefikAuthSecret(service.Name, namespace, kubeClientset) {
		return updateTraefikAuthSecret(service, namespace, kubeClientset)
	}

	return createTraefikAuthSecret(service, namespace, kubeClientset)
}

func deleteTraefikAuthSecret(name string, namespace string, kubeClientset kubernetes.Interface) error {
	return kubeClientset.CoreV1().Secrets(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func existsTraefikAuthSecret(serviceName string, namespace string, kubeClientset kubernetes.Interface) bool {
	_, err := kubeClientset.CoreV1().Secrets(namespace).Get(context.TODO(), getTraefikAuthSecretName(serviceName), metav1.GetOptions{})
	return err == nil
}

func getTraefikAuthSecretSpec(service types.Service, namespace string) *v1.Secret {
	hash := make(htpasswd.HashedPasswords)
	err := hash.SetPassword(service.Name, service.Token, htpasswd.HashAPR1)
	if err != nil {
		ExposeLogger.Print(err.Error())
	}

	inmutable := false
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getTraefikAuthSecretName(service.Name),
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		Immutable: &inmutable,
		StringData: map[string]string{
			"users": service.Name + ":" + hash[service.Name],
		},
		Type: "Opaque",
	}
}

// Secret

func createSecret(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	secret := getSecretSpec(service, namespace, cfg)
	_, err := kubeClientset.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func updateSecret(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	secret := getSecretSpec(service, namespace, cfg)
	_, err := kubeClientset.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func deleteSecret(name string, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	secret := getSecretName(name)
	err := kubeClientset.CoreV1().Secrets(namespace).Delete(context.TODO(), secret, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}
func getSecretSpec(service types.Service, namespace string, cfg *types.Config) *v1.Secret {
	//setPassword
	hash := make(htpasswd.HashedPasswords)
	err := hash.SetPassword(service.Name, service.Token, htpasswd.HashAPR1)
	if err != nil {
		ExposeLogger.Print(err.Error())
	}
	//Create Secret
	inmutable := false
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getSecretName(service.Name),
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		Immutable: &inmutable,
		StringData: map[string]string{
			"auth": service.Name + ":" + hash[service.Name],
		},
		Type: "Opaque",
	}
	return secret
}

func existsSecret(serviceName string, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) bool {
	secret := getSecretName(serviceName)
	exist, err := kubeClientset.CoreV1().Secrets(namespace).Get(context.TODO(), secret, metav1.GetOptions{})
	if err != nil {
		return false
	}
	if exist != nil {
		return true
	}
	return false
}

func existsIngress(serviceName string, namespace string, kubeClientset kubernetes.Interface) bool {
	_, err := kubeClientset.NetworkingV1().Ingresses(namespace).Get(context.TODO(), getIngressName(serviceName), metav1.GetOptions{})
	return err == nil
}

func existsHTTPRoute(serviceName string, namespace string) bool {
	gatewayClientset, err := gatewayClientsetProvider()
	if err != nil {
		return false
	}

	_, err = gatewayClientset.Resource(httpRouteGVR).Namespace(namespace).Get(context.TODO(), getHTTPRouteName(serviceName), metav1.GetOptions{})
	return err == nil
}

/// These are auxiliary functions

func getServiceName(name_container string) string {
	return name_container + "-svc"
}

func getIngressName(name_container string) string {
	return name_container + "-ing"
}

func getHTTPRouteName(name_container string) string {
	return name_container + "-route"
}

func getTraefikCORSMiddlewareName(name_container string) string {
	return name_container + "-cors-mdw"
}

func getTraefikAuthMiddlewareName(name_container string) string {
	return name_container + "-auth-mdw"
}

func getTraefikAuthSecretName(name_container string) string {
	return name_container + "-auth-traefik"
}

func getAPIPath(name_container string) string {
	return "/system/services/" + name_container + "/exposed"
}

func normalizeHealthPath(path string) string {
	if path == "" {
		return "/"
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func isDirectProbeMode(service types.Service) bool {
	return strings.EqualFold(strings.TrimSpace(service.Expose.ProbeMode), "direct")
}

func getProbePath(service types.Service) string {
	healthPath := normalizeHealthPath(service.Expose.HealthPath)
	if isDirectProbeMode(service) {
		return healthPath
	}
	// Legacy default behavior: when rewrite_target is enabled, probe through
	// the ingress-prefixed path used by OSCAR exposed services.
	if service.Expose.RewriteTarget {
		return getAPIPath(service.Name) + healthPath
	}
	return healthPath
}

func GetDeploymentName(nameContainer string) string {
	return nameContainer + "-dlp"
}

func getHPAName(nameContainer string) string {
	return nameContainer + "-hpa"
}

func getSecretName(nameContainer string) string {
	return nameContainer + "-auth-expose"
}

func GetKeyLabelApp(serviceName string) string {
	return prefixLabelApp + serviceName
}
