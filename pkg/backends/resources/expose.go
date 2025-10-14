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
	apps "k8s.io/api/apps/v1"
	autos "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	net "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
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
func CreateExpose(service types.Service, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	//ExposeLogger.Printf("Creating exposed service: \n%v\n", service)
	err := createDeployment(service, kubeClientset, cfg)
	if err != nil {
		return fmt.Errorf("error creating deployment for exposed service '%s': %v", service.Name, err)
	}
	err = createService(service, kubeClientset, cfg)
	if err != nil {
		return fmt.Errorf("error creating svc for exposed service '%s': %v", service.Name, err)
	}
	if service.Expose.NodePort == 0 {
		err = createIngress(service, kubeClientset, cfg)
		if err != nil {
			return fmt.Errorf("error creating ingress for exposed service '%s': %v", service.Name, err)
		}
	}
	return nil
}

// DeleteExpose removes all the components of the exposed service from the cluster
func DeleteExpose(name string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	err := deleteDeployment(name, kubeClientset, cfg)
	if err != nil {
		return fmt.Errorf("error deleting deployment for exposed service '%s': %v", name, err)
	}
	err = deleteService(name, kubeClientset, cfg)
	if err != nil {
		return fmt.Errorf("error deleting service for exposed service '%s': %v", name, err)
	}

	ingressType := existsIngress(name, cfg.ServicesNamespace, kubeClientset)
	if ingressType {
		err = deleteIngress(getIngressName(name), kubeClientset, cfg)
		if existsSecret(name, kubeClientset, cfg) {
			err = deleteSecret(name, kubeClientset, cfg)
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
	err = kubeClientset.CoreV1().Pods(cfg.ServicesNamespace).DeleteCollection(context.TODO(), delete, listOpts)
	if err != nil {
		return fmt.Errorf("error deleting pods of exposed service '%s': %v", name, err)
	}
	return nil
}

// UpdateExpose updates all the components of the exposed service on the cluster
func UpdateExpose(service types.Service, kubeClientset kubernetes.Interface, cfg *types.Config) error {

	// If the deployment exist, and keep continue it will upload.
	err := updateDeployment(service, kubeClientset, cfg)
	if err != nil {
		ExposeLogger.Printf("error updating exposed service deployment: %v\n", err)
		return err
	}
	err = updateService(service, kubeClientset, cfg)
	if err != nil {
		ExposeLogger.Printf("error updating exposed service: %v\n", err)
		return err
	}

	ingressType := existsIngress(service.Name, cfg.ServicesNamespace, kubeClientset)
	// Old service config was Ingress type
	if ingressType {
		// New service config if NodePort
		if service.Expose.NodePort != 0 {
			err = deleteIngress(getIngressName(service.Name), kubeClientset, cfg)
			if existsSecret(service.Name, kubeClientset, cfg) {
				err := deleteSecret(service.Name, kubeClientset, cfg)
				if err != nil {
					return err
				}
			}
			if err != nil {
				log.Printf("error deleting ingress service: %v\n", err)
				return err
			}
		} else {
			err = updateIngress(service, kubeClientset, cfg)
			if err != nil {
				log.Printf("error updating ingress service: %v\n", err)
				return err
			}
		}
	} else {
		// Old service config is NodeType and the new one is Ingress type
		if service.Expose.NodePort == 0 {
			err = createIngress(service, kubeClientset, cfg)
			if err != nil {
				log.Printf("error creating ingress service: %v\n", err)
				return err
			}
		}
	}

	return nil
}

// TODO check and refactor
// Main function that list all the kubernetes components
// This function is not used, in the future could be useful
func ListExpose(kubeClientset kubernetes.Interface, cfg *types.Config) error {
	deploy, hpa, err := listDeployments(kubeClientset, cfg)

	services, err2 := listServices(kubeClientset, cfg)
	ingress, err3 := listIngress(kubeClientset, cfg)
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

}

//////////// Deployment

/// Create deployment and horizontal autoscale

func createDeployment(service types.Service, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	deployment := getDeploymentSpec(service, cfg)
	if utils.SecretExists(service.Name, cfg.ServicesNamespace, kubeClientset) {
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
	_, err := kubeClientset.AppsV1().Deployments(cfg.ServicesNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	hpa := getHortizontalAutoScaleSpec(service, cfg)
	_, err = kubeClientset.AutoscalingV1().HorizontalPodAutoscalers(cfg.ServicesNamespace).Create(context.TODO(), hpa, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Return the component deployment, ready to create or update
func getDeploymentSpec(service types.Service, cfg *types.Config) *apps.Deployment {
	deployName := getDeploymentName(service.Name)
	deployment := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployName,
			Namespace: cfg.ServicesNamespace,
		},
		Spec: apps.DeploymentSpec{
			Replicas: &service.Expose.MinScale,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					keyLabelApp: prefixLabelApp + service.Name,
				},
			},
			Template: getPodTemplateSpec(service, cfg),
		},
		Status: apps.DeploymentStatus{},
	}

	return deployment
}

// Return the component HorizontalAutoScale, ready to create or update
func getHortizontalAutoScaleSpec(service types.Service, cfg *types.Config) *autos.HorizontalPodAutoscaler {
	hpaName := getHPAName(service.Name)
	deployName := getDeploymentName(service.Name)
	hpa := &autos.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hpaName,
			Namespace: cfg.ServicesNamespace,
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

func getPodTemplateSpec(service types.Service, cfg *types.Config) v1.PodTemplateSpec {
	podSpec, _ := service.ToPodSpec(cfg)
	podSpec.EnableServiceLinks = ptr.To(false)

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

		probeHandler := v1.ProbeHandler{
			HTTPGet: &v1.HTTPGetAction{
				Path: getAPIPath(service.Name),
				Port: intstr.FromString(podPortName),
			},
		}
		podSpec.Containers[i].LivenessProbe = &v1.Probe{
			InitialDelaySeconds: 5,
			ProbeHandler:        probeHandler,
		}
		podSpec.Containers[i].ReadinessProbe = &v1.Probe{
			PeriodSeconds: 5,
			ProbeHandler:  probeHandler,
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
			Namespace: service.Image,
			Labels: map[string]string{
				keyLabelApp: prefixLabelApp + service.Name,
			},
		},
		Spec: *podSpec,
	}
	return template
}

// / List deployment and the horizontal auto scale
func listDeployments(kubeClientset kubernetes.Interface, cfg *types.Config) (*apps.DeploymentList, *autos.HorizontalPodAutoscalerList, error) {
	deployment, err := kubeClientset.AppsV1().Deployments(cfg.ServicesNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	hpa, err2 := kubeClientset.AutoscalingV1().HorizontalPodAutoscalers(cfg.ServicesNamespace).List(context.TODO(), metav1.ListOptions{})
	if err2 != nil {
		return nil, nil, err2
	}
	return deployment, hpa, nil
}

// Delete Deployment and HPA
func deleteDeployment(name string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	name_hpa := getHPAName(name)
	err := kubeClientset.AutoscalingV1().HorizontalPodAutoscalers(cfg.ServicesNamespace).Delete(context.TODO(), name_hpa, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	deployment := getDeploymentName(name)
	err = kubeClientset.AppsV1().Deployments(cfg.ServicesNamespace).Delete(context.TODO(), deployment, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

///Update Deployment and HPA

func updateDeployment(service types.Service, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	_, err := kubeClientset.AppsV1().Deployments(cfg.ServicesNamespace).Get(context.TODO(), getDeploymentName(service.Name), metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment := getDeploymentSpec(service, cfg)
	if utils.SecretExists(service.Name, cfg.ServicesNamespace, kubeClientset) {
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
	_, err = kubeClientset.AppsV1().Deployments(cfg.ServicesNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	_, err = kubeClientset.AutoscalingV1().HorizontalPodAutoscalers(cfg.ServicesNamespace).Get(context.TODO(), getHPAName(service.Name), metav1.GetOptions{})
	if err != nil {
		return err
	}
	hpa := getHortizontalAutoScaleSpec(service, cfg)
	_, err = kubeClientset.AutoscalingV1().HorizontalPodAutoscalers(cfg.ServicesNamespace).Update(context.TODO(), hpa, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

/////////// Service

// Create a kubernetes service component
func createService(service types.Service, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	service_spec := getServiceSpec(service, cfg)
	_, err := kubeClientset.CoreV1().Services(cfg.ServicesNamespace).Create(context.TODO(), service_spec, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Return a kubernetes service component, ready to deploy or update
func getServiceSpec(service types.Service, cfg *types.Config) *v1.Service {
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
			Namespace: cfg.ServicesNamespace,
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

func listServices(kubeClientset kubernetes.Interface, cfg *types.Config) (*v1.ServiceList, error) {
	services, err := kubeClientset.CoreV1().Services(cfg.ServicesNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return services, nil
}

// / Update a kubernete service
func updateService(service types.Service, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	kube_service := getServiceSpec(service, cfg)
	_, err := kubeClientset.CoreV1().Services(cfg.ServicesNamespace).Update(context.TODO(), kube_service, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

/// Delete kubernetes service

func deleteService(name string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	kube_service := getServiceName(name)
	err := kubeClientset.CoreV1().Services(cfg.ServicesNamespace).Delete(context.TODO(), kube_service, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

/////////// Ingress

// / Create an ingress component
func createIngress(service types.Service, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	// Create Secret

	ingress := getIngressSpec(service, cfg)
	_, err := kubeClientset.NetworkingV1().Ingresses(cfg.ServicesNamespace).Create(context.TODO(), ingress, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	if service.Expose.SetAuth {
		cerr := createSecret(service, kubeClientset, cfg)
		if cerr != nil {
			return cerr
		}
	}
	return nil
}

// / Update a kubernete service
func updateIngress(service types.Service, kubeClientset kubernetes.Interface, cfg *types.Config) error {

	serviceName := service.Name
	//if exist continue and need -> Update
	//if exist and not need -> delete
	//if not  exist create
	kube_ingress := getIngressSpec(service, cfg)
	_, err := kubeClientset.NetworkingV1().Ingresses(cfg.ServicesNamespace).Update(context.TODO(), kube_ingress, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	secret := existsSecret(serviceName, kubeClientset, cfg)
	if secret {
		if service.Expose.SetAuth {
			uerr := updateSecret(service, kubeClientset, cfg)
			if uerr != nil {
				return uerr
			}
		} else {
			derr := deleteSecret(service.Name, kubeClientset, cfg)
			if derr != nil {
				return derr
			}
		}
	} else {
		if service.Expose.SetAuth {
			cerr := createSecret(service, kubeClientset, cfg)
			if cerr != nil {
				return cerr
			}
		}
	}

	return nil
}

// Return a kubernetes ingress component, ready to deploy or update
func getIngressSpec(service types.Service, cfg *types.Config) *net.Ingress {
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
			Namespace:   cfg.ServicesNamespace,
			Annotations: annotation,
		},
		Spec:   specification,
		Status: net.IngressStatus{},
	}
	return ingress
}

/// List the kuberntes ingress

func listIngress(kubeClientset kubernetes.Interface, cfg *types.Config) (*net.IngressList, error) {
	ingress, err := kubeClientset.NetworkingV1().Ingresses(cfg.ServicesNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return ingress, nil
}

// Delete a kubernetes ingress
func deleteIngress(name string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	// if secret exist, delete
	err := kubeClientset.NetworkingV1().Ingresses(cfg.ServicesNamespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Secret

func createSecret(service types.Service, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	secret := getSecretSpec(service, cfg)
	_, err := kubeClientset.CoreV1().Secrets(cfg.ServicesNamespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func updateSecret(service types.Service, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	secret := getSecretSpec(service, cfg)
	_, err := kubeClientset.CoreV1().Secrets(cfg.ServicesNamespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func deleteSecret(name string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	secret := getSecretName(name)
	err := kubeClientset.CoreV1().Secrets(cfg.ServicesNamespace).Delete(context.TODO(), secret, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}
func getSecretSpec(service types.Service, cfg *types.Config) *v1.Secret {
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
			Namespace: cfg.ServicesNamespace,
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

func existsSecret(serviceName string, kubeClientset kubernetes.Interface, cfg *types.Config) bool {
	secret := getSecretName(serviceName)
	exist, err := kubeClientset.CoreV1().Secrets(cfg.ServicesNamespace).Get(context.TODO(), secret, metav1.GetOptions{})
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
