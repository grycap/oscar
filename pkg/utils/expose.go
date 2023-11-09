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

	"github.com/grycap/oscar/v2/pkg/types"
	apps "k8s.io/api/apps/v1"
	autos "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	net "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

type Expose struct {
	Name         string ` binding:"required"`
	NameSpace    string ` binding:"required"`
	Image        string ` `
	Variables    map[string]string
	MaxScale     int32 `default:"10"`
	MinScale     int32 `default:"1"`
	Port         int   ` binding:"required" default:"80"`
	CpuThreshold int32 `default:"80"`
	EnableSGX    bool
}

// Custom logger
var ExposeLogger = log.New(os.Stdout, "[EXPOSED-SERVICE] ", log.Flags())

// / Main function that creates all the kubernetes components
func CreateExpose(expose Expose, kubeClientset kubernetes.Interface, cfg types.Config) error {
	ExposeLogger.Printf("DEBUG: Creating exposed service: \n%v\n", expose)
	err := createDeployment(expose, kubeClientset)
	if err != nil {
		ExposeLogger.Printf("WARNING: %v\n", err)
		return err
	}
	err = createService(expose, kubeClientset)
	if err != nil {
		ExposeLogger.Printf("WARNING: %v\n", err)
		return err
	}
	err = createIngress(expose, kubeClientset, cfg)
	if err != nil {
		ExposeLogger.Printf("WARNING: %v\n", err)
		return err
	}
	return nil
}

// /Main function that deletes all the kubernetes components
func DeleteExpose(expose Expose, kubeClientset kubernetes.Interface) error {
	err := deleteDeployment(expose, kubeClientset)
	if err != nil {
		ExposeLogger.Printf("WARNING: %v\n", err)
		return err
	}
	err = deleteService(expose, kubeClientset)
	if err != nil {
		ExposeLogger.Printf("WARNING: %v\n", err)
		return err
	}
	err = deleteIngress(expose, kubeClientset)
	if err != nil {
		ExposeLogger.Printf("WARNING: %v\n", err)
		return err
	}
	return nil
}

// /Main function that updates all the kubernetes components
func UpdateExpose(expose Expose, kubeClientset kubernetes.Interface, cfg types.Config) error {

	deployment := getNameDeployment(expose.Name)
	_, error := kubeClientset.AppsV1().Deployments(expose.NameSpace).Get(context.TODO(), deployment, metav1.GetOptions{})
	//If the deployment does not exist the function above will return a error and it will create the hold process
	if error != nil && expose.Port != 0 {
		CreateExpose(expose, kubeClientset, cfg)
		return nil
	}
	// If the deployment exist and we select the port 0, it will delete all expose components
	if expose.Port == 0 {
		DeleteExpose(expose, kubeClientset)
		return nil
	}
	err := updateDeployment(expose, kubeClientset)
	if err != nil {
		ExposeLogger.Printf("WARNING: %v\n", err)
		return err
	}
	err2 := updateService(expose, kubeClientset)
	if err2 != nil {
		ExposeLogger.Printf("WARNING: %v\n", err2)
		return err2
	}
	return nil
}

// /Main function that list all the kubernetes components
// This function is not used, in the future could be usefull
func ListExpose(expose Expose, kubeClientset kubernetes.Interface) error {
	deploy, hpa, err := listDeployments(expose, kubeClientset)

	services, err2 := listServices(expose, kubeClientset)
	ingress, err3 := listIngress(expose, kubeClientset)
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

func createDeployment(e Expose, client kubernetes.Interface) error {
	deployment := getDeployment(e)
	_, err := client.AppsV1().Deployments(e.NameSpace).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	hpa := getHortizontalAutoScale(e)
	_, err = client.AutoscalingV1().HorizontalPodAutoscalers(e.NameSpace).Create(context.TODO(), hpa, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Return the component deployment, ready to create or update
func getDeployment(e Expose) *apps.Deployment {
	name_deployment := getNameDeployment(e.Name)
	deployment := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name_deployment,
			Namespace: e.NameSpace,
		},
		Spec: apps.DeploymentSpec{
			Replicas: &e.MinScale,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "oscar-svc-exp-" + e.Name,
				},
			},
			Template: getPodTemplateSpec(e),
		},
		Status: apps.DeploymentStatus{},
	}

	return deployment
}

// Return the component HorizontalAutoScale, ready to create or update
func getHortizontalAutoScale(e Expose) *autos.HorizontalPodAutoscaler {
	name_hpa := getNameHPA(e.Name)
	name_deployment := getNameDeployment(e.Name)
	hpa := &autos.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name_hpa,
			Namespace: e.NameSpace,
		},
		Spec: autos.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autos.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       name_deployment,
				APIVersion: "apps/v1",
			},
			MinReplicas:                    &e.MinScale,
			MaxReplicas:                    e.MaxScale,
			TargetCPUUtilizationPercentage: &e.CpuThreshold,
		},
		Status: autos.HorizontalPodAutoscalerStatus{},
	}
	return hpa
}

// Return the Pod spec inside of deployment, ready to create or update

func getPodTemplateSpec(e Expose) v1.PodTemplateSpec {
	var ports v1.ContainerPort = v1.ContainerPort{
		Name:          "port",
		ContainerPort: int32(e.Port),
	}
	cores := resource.NewMilliQuantity(500, resource.DecimalSI)

	template := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      e.Name,
			Namespace: e.Image,
			Labels: map[string]string{
				"app": "oscar-svc-exp-" + e.Name,
			},
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{},
			Containers: []v1.Container{
				{
					Name:  e.Name,
					Image: e.Image,
					Env:   types.ConvertEnvVars(e.Variables),
					Ports: []v1.ContainerPort{ports},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							"cpu": *cores,
						},
						// Empty Limits list initialized in case enabling SGX is needed
						Limits: v1.ResourceList{},
					},
				},
			},
		},
	}

	if e.EnableSGX {
		ExposeLogger.Printf("DEBUG: Enabling components to use SGX plugin\n")
		types.SetSecurityContext(&template.Spec)
		sgx, _ := resource.ParseQuantity("1")
		template.Spec.Containers[0].Resources.Limits["sgx.intel.com/enclave"] = sgx
	}

	return template
}

// / List deployment and the horizontal auto scale
func listDeployments(e Expose, client kubernetes.Interface) (*apps.DeploymentList, *autos.HorizontalPodAutoscalerList, error) {
	deployment, err := client.AppsV1().Deployments(e.NameSpace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	hpa, err2 := client.AutoscalingV1().HorizontalPodAutoscalers(e.NameSpace).List(context.TODO(), metav1.ListOptions{})
	if err2 != nil {
		return nil, nil, err2
	}
	return deployment, hpa, nil
}

// Delete Deployment and HPA
func deleteDeployment(e Expose, client kubernetes.Interface) error {
	name_hpa := getNameHPA(e.Name)
	err := client.AutoscalingV1().HorizontalPodAutoscalers(e.NameSpace).Delete(context.TODO(), name_hpa, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	deployment := getNameDeployment(e.Name)
	err = client.AppsV1().Deployments(e.NameSpace).Delete(context.TODO(), deployment, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

///Update Deployment and HPA

func updateDeployment(e Expose, client kubernetes.Interface) error {
	_, err := client.AppsV1().Deployments(e.NameSpace).Get(context.TODO(), getNameDeployment(e.Name), metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment := getDeployment(e)
	_, err = client.AppsV1().Deployments(e.NameSpace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	client.AutoscalingV1().HorizontalPodAutoscalers(e.NameSpace).Get(context.TODO(), getNameHPA(e.Name), metav1.GetOptions{})
	hpa := getHortizontalAutoScale(e)
	_, err = client.AutoscalingV1().HorizontalPodAutoscalers(e.NameSpace).Update(context.TODO(), hpa, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

/////////// Service

// Create a kubernetes service component
func createService(e Expose, client kubernetes.Interface) error {
	service := getService(e)
	_, err := client.CoreV1().Services(e.NameSpace).Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Return a kubernetes service component, ready to deploy or update
func getService(e Expose) *v1.Service {
	name_service := getNameService(e.Name)
	var port v1.ServicePort = v1.ServicePort{
		Name: "",
		Port: 80,
		TargetPort: intstr.IntOrString{
			Type:   0,
			IntVal: int32(e.Port),
		},
	}
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name_service,
			Namespace: e.NameSpace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{port},
			Selector: map[string]string{
				"app": "oscar-svc-exp-" + e.Name,
			},
		},
		Status: v1.ServiceStatus{},
	}
	return service
}

/// List services in a certain namespace

func listServices(e Expose, client kubernetes.Interface) (*v1.ServiceList, error) {
	services, err := client.CoreV1().Services(e.NameSpace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return services, nil
}

// / Update a kubernete service
func updateService(e Expose, client kubernetes.Interface) error {
	service := getService(e)
	_, err := client.CoreV1().Services(e.NameSpace).Update(context.TODO(), service, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

/// Delete kubernetes service

func deleteService(e Expose, client kubernetes.Interface) error {
	service := getNameService(e.Name)
	err := client.CoreV1().Services(e.NameSpace).Delete(context.TODO(), service, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

/////////// Ingress

// / Create an ingress component
func createIngress(e Expose, client kubernetes.Interface, cfg types.Config) error {

	ingress := getIngress(e, client, cfg)
	_, err := client.NetworkingV1().Ingresses(e.NameSpace).Create(context.TODO(), ingress, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Return a kubernetes ingress component, ready to deploy or update
func getIngress(e Expose, client kubernetes.Interface, cfg types.Config) *net.Ingress {
	name_ingress := getNameIngress(e.Name)
	pathofapi := getPathAPI(e.Name)
	name_service := getNameService(e.Name)
	var ptype net.PathType = "Prefix"
	var ingresspath net.HTTPIngressPath = net.HTTPIngressPath{
		Path:     pathofapi,
		PathType: &ptype,
		Backend: net.IngressBackend{
			Service: &net.IngressServiceBackend{
				Name: name_service,
				Port: net.ServiceBackendPort{
					Number: 80,
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

	//////
	ingress := &net.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name_ingress,
			Namespace: e.NameSpace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/$1",
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

func listIngress(e Expose, client kubernetes.Interface) (*net.IngressList, error) {
	ingress, err := client.NetworkingV1().Ingresses(e.NameSpace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return ingress, nil
}

// Delete a kubernetes ingress
func deleteIngress(e Expose, client kubernetes.Interface) error {
	ingress := getNameIngress(e.Name)
	err := client.NetworkingV1().Ingresses(e.NameSpace).Delete(context.TODO(), ingress, metav1.DeleteOptions{})
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
	return "/system/services/" + name_container + "/exposed/?(.*)"
}

func getNameDeployment(name_container string) string {
	return name_container + "-dlp"
}

func getNameHPA(name_container string) string {
	return name_container + "-hpa"
}
