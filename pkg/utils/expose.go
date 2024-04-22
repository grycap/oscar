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

package utils

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/grycap/oscar/v3/pkg/types"
	apps "k8s.io/api/apps/v1"
	autos "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	net "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

// / Main function that creates all the kubernetes components
func CreateExpose(service types.Service, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	ExposeLogger.Printf("DEBUG: Creating exposed service: \n%v\n", service)
	err := createDeployment(service, kubeClientset, cfg)
	if err != nil {
		ExposeLogger.Printf("WARNING: %v\n", err)
		return err
	}
	err = createService(service, kubeClientset, cfg)
	if err != nil {
		ExposeLogger.Printf("WARNING: %v\n", err)
		return err
	}
	if service.Expose.NodePort == 0 {
		err = createIngress(service, kubeClientset, cfg)
		if err != nil {
			ExposeLogger.Printf("WARNING: %v\n", err)
			log.Printf("WARNING: %v\n", err)
			return err
		}
	}

	return nil
}

// /Main function that deletes all the kubernetes components
func DeleteExpose(name string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	err := deleteDeployment(name, kubeClientset, cfg)
	if err != nil {
		ExposeLogger.Printf("WARNING: %v\n", err)
		return err
	}
	err = deleteService(name, kubeClientset, cfg)
	if err != nil {
		ExposeLogger.Printf("WARNING: %v\n", err)
		return err
	}

	ings, _ := listIngress(kubeClientset, cfg)
	for i := 0; i < len(ings.Items); i++ {
		if ings.Items[i].Name == getNameIngress(name) {
			err = deleteIngress(name, kubeClientset, cfg)
			if err != nil {
				ExposeLogger.Printf("WARNING: %v\n", err)
				log.Printf("WARNING: %v\n", err)
				return err
			}
			return nil
		}
	}
	return nil
}

// /Main function that updates all the kubernetes components
func UpdateExpose(service types.Service, kubeClientset kubernetes.Interface, cfg *types.Config) error {

	deployment := getNameDeployment(service.Name)
	_, error := kubeClientset.AppsV1().Deployments(cfg.ServicesNamespace).Get(context.TODO(), deployment, metav1.GetOptions{})
	//If the deployment does not exist the function above will return a error and it will create the hold process
	if error != nil && service.Expose.Port != 0 {
		CreateExpose(service, kubeClientset, cfg)
		return nil
	}
	// If the deployment exist and we select the port 0, it will delete all expose components
	if service.Expose.Port == 0 {
		DeleteExpose(service.Name, kubeClientset, cfg)
		return nil
	}
	// If the deployment exist, and keep continue it will upload.
	err := updateDeployment(service, kubeClientset, cfg)
	if err != nil {
		ExposeLogger.Printf("WARNING: %v\n", err)
		return err
	}
	err2 := updateService(service, kubeClientset, cfg)
	if err2 != nil {
		ExposeLogger.Printf("WARNING: %v\n", err2)
		return err2
	}
	// Cluster to cluster -> Update, got in list( exist) and NodePort is 0
	// Cluster to Node -> Delete got in list( exist) and NodePort is not  0

	// Node to Node -> Nothing 			not exist  and NodePort is not  0
	// Node to Cluster -> create       not exist and NodePort is 0

	ings, _ := listIngress(kubeClientset, cfg)
	for i := 0; i < len(ings.Items); i++ {
		if ings.Items[i].Name == getNameIngress(service.Name) {

			if service.Expose.NodePort == 0 {
				//Cluster to cluster
				err3 := updateIngress(service, kubeClientset, cfg)
				if err3 != nil {
					log.Printf("WARNING: %v\n", err)
					return err
				}
			} else {
				//Cluster to Node
				err = deleteIngress(getNameIngress(service.Name), kubeClientset, cfg)
				if err != nil {
					log.Printf("WARNING: %v\n", err)
					return err
				}
			}
			return nil
		}
	}
	//Node to Cluster
	if service.Expose.NodePort == 0 {
		err = createIngress(service, kubeClientset, cfg)
		if err != nil {
			log.Printf("WARNING: %v\n", err)
			return err
		}
	}
	//Node to Node
	return nil
}

// /Main function that list all the kubernetes components
// This function is not used, in the future could be usefull
func ListExpose(service types.Service, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	deploy, hpa, err := listDeployments(service, kubeClientset, cfg)

	services, err2 := listServices(service, kubeClientset, cfg)
	ingress, err3 := listIngress(kubeClientset, cfg)
	if err != nil {
		ExposeLogger.Printf("WARNING: %v\n", err)
		return err
	}
	if err2 != nil {
		ExposeLogger.Printf("WARNING: %v\n", err2)
		return err
	}
	if err3 != nil {
		ExposeLogger.Printf("WARNING: %v\n", err3)
		return err
	}
	fmt.Println(deploy, hpa, services, ingress)
	return nil

}

//////////// Deployment

/// Create deployment and horizontal autoscale

func createDeployment(service types.Service, client kubernetes.Interface, cfg *types.Config) error {
	deployment := getDeploymentSpec(service, cfg)
	_, err := client.AppsV1().Deployments(cfg.ServicesNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	hpa := getHortizontalAutoScaleSpec(service, cfg)
	_, err = client.AutoscalingV1().HorizontalPodAutoscalers(cfg.ServicesNamespace).Create(context.TODO(), hpa, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Return the component deployment, ready to create or update
func getDeploymentSpec(service types.Service, cfg *types.Config) *apps.Deployment {
	name_deployment := getNameDeployment(service.Name)
	deployment := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name_deployment,
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
	name_hpa := getNameHPA(service.Name)
	name_deployment := getNameDeployment(service.Name)
	hpa := &autos.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name_hpa,
			Namespace: cfg.ServicesNamespace,
		},
		Spec: autos.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autos.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       name_deployment,
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

	for i, _ := range podSpec.Containers {
		podSpec.Containers[i].Ports = []v1.ContainerPort{
			{
				Name:          podPortName,
				ContainerPort: int32(service.Expose.Port),
			},
		}
		podSpec.Containers[i].Resources = v1.ResourceRequirements{
			Requests: v1.ResourceList{
				"cpu": *resource.NewMilliQuantity(500, resource.DecimalSI),
			},
		}
		podSpec.Containers[i].VolumeMounts[0].ReadOnly = false
		if service.Expose.DefaultCommand == true {
			podSpec.Containers[i].Command = nil
			podSpec.Containers[i].Args = nil
		} else {
			podSpec.Containers[i].Command = []string{"/bin/sh"}
			podSpec.Containers[i].Args = []string{"-c", fmt.Sprintf("%s/%s", types.ConfigPath, types.ScriptFileName)}
		}
	}
	var num int32 = 0777
	podSpec.Volumes[0].VolumeSource.ConfigMap.DefaultMode = &num
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

	if service.Expose.EnableSGX {
		ExposeLogger.Printf("DEBUG: Enabling components to use SGX plugin\n")
		types.SetSecurityContext(&template.Spec)
		sgx, _ := resource.ParseQuantity("1")
		template.Spec.Containers[0].Resources.Limits["sgx.intel.com/enclave"] = sgx
	}

	return template
}

// / List deployment and the horizontal auto scale
func listDeployments(service types.Service, client kubernetes.Interface, cfg *types.Config) (*apps.DeploymentList, *autos.HorizontalPodAutoscalerList, error) {
	deployment, err := client.AppsV1().Deployments(cfg.ServicesNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	hpa, err2 := client.AutoscalingV1().HorizontalPodAutoscalers(cfg.ServicesNamespace).List(context.TODO(), metav1.ListOptions{})
	if err2 != nil {
		return nil, nil, err2
	}
	return deployment, hpa, nil
}

// Delete Deployment and HPA
func deleteDeployment(name string, client kubernetes.Interface, cfg *types.Config) error {
	name_hpa := getNameHPA(name)
	err := client.AutoscalingV1().HorizontalPodAutoscalers(cfg.ServicesNamespace).Delete(context.TODO(), name_hpa, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	deployment := getNameDeployment(name)
	err = client.AppsV1().Deployments(cfg.ServicesNamespace).Delete(context.TODO(), deployment, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

///Update Deployment and HPA

func updateDeployment(service types.Service, client kubernetes.Interface, cfg *types.Config) error {
	_, err := client.AppsV1().Deployments(cfg.ServicesNamespace).Get(context.TODO(), getNameDeployment(service.Name), metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment := getDeploymentSpec(service, cfg)
	_, err = client.AppsV1().Deployments(cfg.ServicesNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	client.AutoscalingV1().HorizontalPodAutoscalers(cfg.ServicesNamespace).Get(context.TODO(), getNameHPA(service.Name), metav1.GetOptions{})
	hpa := getHortizontalAutoScaleSpec(service, cfg)
	_, err = client.AutoscalingV1().HorizontalPodAutoscalers(cfg.ServicesNamespace).Update(context.TODO(), hpa, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

/////////// Service

// Create a kubernetes service component
func createService(service types.Service, client kubernetes.Interface, cfg *types.Config) error {
	service_spec := getServiceSpec(service, cfg)
	_, err := client.CoreV1().Services(cfg.ServicesNamespace).Create(context.TODO(), service_spec, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Return a kubernetes service component, ready to deploy or update
func getServiceSpec(service types.Service, cfg *types.Config) *v1.Service {
	name_service := getNameService(service.Name)
	var port v1.ServicePort = v1.ServicePort{
		Name: servicePortName,
		Port: servicePortNumber,
		TargetPort: intstr.IntOrString{
			Type:   0,
			IntVal: int32(service.Expose.Port),
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

func listServices(service types.Service, client kubernetes.Interface, cfg *types.Config) (*v1.ServiceList, error) {
	services, err := client.CoreV1().Services(cfg.ServicesNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return services, nil
}

// / Update a kubernete service
func updateService(service types.Service, client kubernetes.Interface, cfg *types.Config) error {
	kube_service := getServiceSpec(service, cfg)
	_, err := client.CoreV1().Services(cfg.ServicesNamespace).Update(context.TODO(), kube_service, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

/// Delete kubernetes service

func deleteService(name string, client kubernetes.Interface, cfg *types.Config) error {
	kube_service := getNameService(name)
	err := client.CoreV1().Services(cfg.ServicesNamespace).Delete(context.TODO(), kube_service, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

/////////// Ingress

// / Create an ingress component
func createIngress(service types.Service, client kubernetes.Interface, cfg *types.Config) error {
	// Create Secret

	ingress := getIngressSpec(service, client, cfg)
	_, err := client.NetworkingV1().Ingresses(cfg.ServicesNamespace).Create(context.TODO(), ingress, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// / Update a kubernete service
func updateIngress(service types.Service, client kubernetes.Interface, cfg *types.Config) error {
	//if exist continue and need -> Update
	//if exist and not need -> delete
	//if not  exist create
	kube_ingress := getIngressSpec(service, client, cfg)
	_, err := client.NetworkingV1().Ingresses(cfg.ServicesNamespace).Update(context.TODO(), kube_ingress, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Return a kubernetes ingress component, ready to deploy or update
func getIngressSpec(service types.Service, client kubernetes.Interface, cfg *types.Config) *net.Ingress {
	name_ingress := getNameIngress(service.Name)
	pathofapi := getPathAPI(service.Name)
	name_service := getNameService(service.Name)
	var ptype net.PathType = "Prefix"
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
		specification = net.IngressSpec{
			TLS:   []net.IngressTLS{tls},
			Rules: []net.IngressRule{rule}, //IngressClassName:
		}
	}

	rewriteOption := "/$1"
	if service.Expose.RewriteTarget == true {
		rewriteOption = pathofapi + "/$1"
	}
	ingress := &net.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name_ingress,
			Namespace: cfg.ServicesNamespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": rewriteOption,
				"kubernetes.io/ingress.class":                "nginx",
				"nginx.ingress.kubernetes.io/use-regex":      "true",
			},
		},
		Spec:   specification,
		Status: net.IngressStatus{},
	}
	return ingress
}

/// List the kuberntes ingress

func listIngress(client kubernetes.Interface, cfg *types.Config) (*net.IngressList, error) {
	ingress, err := client.NetworkingV1().Ingresses(cfg.ServicesNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return ingress, nil
}

// Delete a kubernetes ingress
func deleteIngress(name string, client kubernetes.Interface, cfg *types.Config) error {
	// if secret exist, delete
	ingress := getNameIngress(name)
	err := client.NetworkingV1().Ingresses(cfg.ServicesNamespace).Delete(context.TODO(), ingress, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

/// These are auxiliary functions

func getNameService(name_container string) string {
	return name_container + "-svc"
}

func getNameIngress(name_container string) string {
	return name_container + "-ing"
}

func getPathAPI(name_container string) string {
	return "/system/services/" + name_container + "/exposed"
}

func getNameDeployment(name_container string) string {
	return name_container + "-dlp"
}

func getNameHPA(name_container string) string {
	return name_container + "-hpa"
}
