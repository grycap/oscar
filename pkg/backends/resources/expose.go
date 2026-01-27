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

	htpasswd "github.com/foomo/htpasswd"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	apps "k8s.io/api/apps/v1"
	autos "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	net "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	typeNodePort      = "NodePort"
	typeClusterIP     = "ClusterIP"
	prefixLabelApp    = "oscar-svc-exp-"
	keyLabelApp       = "app"
	podPortName       = "podport"
	servicePortName   = "serviceport"
	servicePortNumber = 80
)

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
		err = createIngress(service, targetNamespace, kubeClientset, cfg)
		if err != nil {
			return fmt.Errorf("error creating ingress for exposed service '%s': %v", service.Name, err)
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
	if err != nil {
		return fmt.Errorf("error deleting deployment for exposed service '%s': %v", name, err)
	}
	err = deleteService(name, targetNamespace, kubeClientset, cfg)
	if err != nil {
		return fmt.Errorf("error deleting service for exposed service '%s': %v", name, err)
	}

	ingressType := existsIngress(name, targetNamespace, kubeClientset)
	if ingressType {
		err = deleteIngress(getIngressName(name), targetNamespace, kubeClientset, cfg)
		if existsSecret(name, targetNamespace, kubeClientset, cfg) {
			err = deleteSecret(name, targetNamespace, kubeClientset, cfg)
			if err != nil {
				return err
			}
		}
		if err != nil {
			return fmt.Errorf("error deleting ingress for exposed service '%s': %v", name, err)
		}
	}
	termination := int64(0)
	back := metav1.DeletePropagationBackground
	delete := metav1.DeleteOptions{
		GracePeriodSeconds: &termination,
		PropagationPolicy:  &back,
	}
	listOpts := metav1.ListOptions{
		LabelSelector: "app=oscar-svc-exp-" + name,
	}
	err = kubeClientset.CoreV1().Pods(targetNamespace).DeleteCollection(context.TODO(), delete, listOpts)
	if err != nil {
		return fmt.Errorf("error deleting pods of exposed service '%s': %v", name, err)
	}
	utils.DeleteWorkload(name, targetNamespace, cfg)

	return nil
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

	ingressType := existsIngress(service.Name, targetNamespace, kubeClientset)
	// Old service config was Ingress type
	if ingressType {
		// New service config if NodePort
		if service.Expose.NodePort != 0 {
			err = deleteIngress(getIngressName(service.Name), targetNamespace, kubeClientset, cfg)
			if existsSecret(service.Name, targetNamespace, kubeClientset, cfg) {
				err := deleteSecret(service.Name, targetNamespace, kubeClientset, cfg)
				if err != nil {
					return err
				}
			}
			if err != nil {
				log.Printf("error deleting ingress service: %v\n", err)
				return err
			}
		} else {
			err = updateIngress(service, targetNamespace, kubeClientset, cfg)
			if err != nil {
				log.Printf("error updating ingress service: %v\n", err)
				return err
			}
		}
	} else {
		// Old service config is NodeType and the new one is Ingress type
		if service.Expose.NodePort == 0 {
			err = createIngress(service, targetNamespace, kubeClientset, cfg)
			if err != nil {
				log.Printf("error creating ingress service: %v\n", err)
				return err
			}
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
				ExposeLogger.Printf(err.Error())
			}
			ExposeLogger.Printf("Error checking workload admission: change the cpu/memory requests\n")
			return err
		}
	}

	return nil
}

// Return the component deployment, ready to create or update
func getDeploymentSpec(service types.Service, namespace string, cfg *types.Config) *apps.Deployment {
	deployName := getDeploymentName(service.Name)
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
				types.KueueOwnerLabel: service.Owner,
			},
		},
		Spec: apps.DeploymentSpec{
			Replicas: &minScale,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					keyLabelApp: prefixLabelApp + service.Name,
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
	deployName := getDeploymentName(service.Name)
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
		if service.Expose.RewriteTarget {
			probePath = getAPIPath(service.Name) + service.Expose.HealthPath
		}

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
				keyLabelApp: prefixLabelApp + service.Name,
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
	if err != nil {
		return err
	}
	deployment := getDeploymentName(name)
	err = kubeClientset.AppsV1().Deployments(namespace).Delete(context.TODO(), deployment, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

///Update Deployment and HPA

func updateDeployment(service types.Service, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	_, err := kubeClientset.AppsV1().Deployments(namespace).Get(context.TODO(), getDeploymentName(service.Name), metav1.GetOptions{})
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
				keyLabelApp: prefixLabelApp + service.Name,
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

/// These are auxiliary functions

func getServiceName(name_container string) string {
	return name_container + "-svc"
}

func getIngressName(name_container string) string {
	return name_container + "-ing"
}

func getAPIPath(name_container string) string {
	return "/system/services/" + name_container + "/exposed"
}

func getDeploymentName(name_container string) string {
	return name_container + "-dlp"
}

func getHPAName(name_container string) string {
	return name_container + "-hpa"
}

func getSecretName(name_container string) string {
	return name_container + "-auth-expose"
}
