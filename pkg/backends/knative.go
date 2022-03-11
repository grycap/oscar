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

package backends

import (
	"context"
	"log"

	"github.com/grycap/oscar/v2/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	knv1 "knative.dev/serving/pkg/apis/serving/v1"
	knclientset "knative.dev/serving/pkg/client/clientset/versioned"
)

// TODO

// TODO: add annotation "serving.knative.dev/visibility=cluster-local"
// to make all services only cluster-local, the Kn serving component can be configured to use the default domain "svc.cluster.local"
// https://knative.dev/docs/serving/cluster-local-route/

// KnativeBackend struct to represent a Knative client
type KnativeBackend struct {
	kubeClientset kubernetes.Interface
	knClientset   *knclientset.Clientset
	namespace     string
	//gatewayEndpoint string
	config *types.Config
}

// MakeKnativeBackend makes a KnativeBackend from the provided k8S clientset and config
func MakeKnativeBackend(kubeClientset kubernetes.Interface, kubeConfig *rest.Config, cfg *types.Config) *KnativeBackend {
	knClientset, err := knclientset.NewForConfig(kubeConfig)
	if err != nil {
		log.Fatal(err)
	}

	return &KnativeBackend{
		kubeClientset: kubeClientset,
		knClientset:   knClientset,
		namespace:     cfg.ServicesNamespace,
		//gatewayEndpoint: fmt.Sprintf("gateway.%s:%d", cfg.OpenfaasNamespace, cfg.OpenfaasPort),
		config: cfg,
	}
}

// GetInfo returns the ServerlessBackendInfo with the name and version
func (kn *KnativeBackend) GetInfo() *types.ServerlessBackendInfo {
	backInfo := &types.ServerlessBackendInfo{
		Name: "Knative",
	}

	version, err := kn.knClientset.Discovery().ServerVersion()
	if err == nil {
		backInfo.Version = version.GitVersion
	}

	return backInfo
}

// ListServices returns a slice with all services registered in the provided namespace
func (kn *KnativeBackend) ListServices() ([]*types.Service, error) {
	// Get the list with all Knative services
	knSvcs, err := kn.knClientset.ServingV1().Services(kn.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	services := []*types.Service{}
	for _, knSvc := range knSvcs.Items {
		// Get service from configMap's FDL
		svc, err := getServiceFromFDL(knSvc.Name, kn.namespace, kn.kubeClientset)
		if err != nil {
			log.Printf("WARNING: %v\n", err)
		} else {
			services = append(services, svc)
		}
	}

	return services, nil
}

// CreateService creates a new service as a Knative service
func (kn *KnativeBackend) CreateService(service types.Service) error {
	// Create the configMap with FDL and user-script
	err := createServiceConfigMap(&service, kn.namespace, kn.kubeClientset)
	if err != nil {
		return err
	}

	// Create the Function through the OpenFaaS operator
	knSvc, err := kn.createKNServiceDefinition(&service)
	if err != nil {
		// Delete the previously created configMap
		if delErr := deleteServiceConfigMap(service.Name, kn.namespace, kn.kubeClientset); delErr != nil {
			log.Println(delErr.Error())
		}
		return err
	}

	_, err = kn.knClientset.ServingV1().Services(kn.namespace).Create(context.TODO(), knSvc, metav1.CreateOptions{})
	if err != nil {
		// Delete the previously created configMap
		if delErr := deleteServiceConfigMap(service.Name, kn.namespace, kn.kubeClientset); delErr != nil {
			log.Println(delErr.Error())
		}
		return err
	}

	return nil
}

// ReadService returns a Service
func (kn *KnativeBackend) ReadService(name string) (*types.Service, error) {
	// Check if service exists
	if _, err := kn.knClientset.ServingV1().Services(kn.namespace).Get(context.TODO(), name, metav1.GetOptions{}); err != nil {
		return nil, err
	}

	// Get service from configMap's FDL
	svc, err := getServiceFromFDL(name, kn.namespace, kn.kubeClientset)
	if err != nil {
		return nil, err
	}

	return svc, nil
}

// TODO
// // UpdateService updates an existent service
// func (kn *KnativeBackend) UpdateService(service types.Service) error {
// 	// Get the old service's configMap
// 	oldCm, err := kn.kubeClientset.CoreV1().ConfigMaps(kn.namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
// 	if err != nil {
// 		return fmt.Errorf("the service \"%s\" does not have a registered ConfigMap", service.Name)
// 	}

// 	// Update the configMap with FDL and user-script
// 	if err := updateServiceConfigMap(&service, kn.namespace, kn.kubeClientset); err != nil {
// 		return err
// 	}

// 	// TODO: create new knative service definition (including annotation/labels)
// 	// Create podSpec from the service
// 	podSpec, err := service.ToPodSpec(of.config)
// 	if err != nil {
// 		// Restore the old configMap
// 		_, resErr := of.kubeClientset.CoreV1().ConfigMaps(of.namespace).Update(context.TODO(), oldCm, metav1.UpdateOptions{})
// 		if resErr != nil {
// 			log.Println(resErr.Error())
// 		}
// 		return err
// 	}

// 	// Get the service's deployment to update its podSpec
// 	deployment, err := of.kubeClientset.AppsV1().Deployments(of.namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
// 	if err != nil {
// 		// Restore the old configMap
// 		_, resErr := of.kubeClientset.CoreV1().ConfigMaps(of.namespace).Update(context.TODO(), oldCm, metav1.UpdateOptions{})
// 		if resErr != nil {
// 			log.Println(resErr.Error())
// 		}
// 		return err
// 	}

// 	// Update podSpec in the deployment
// 	deployment.Spec.Template.Spec = *podSpec

// 	// Update the deployment
// 	_, err = of.kubeClientset.AppsV1().Deployments(of.namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
// 	if err != nil {
// 		// Restore the old configMap
// 		_, resErr := of.kubeClientset.CoreV1().ConfigMaps(of.namespace).Update(context.TODO(), oldCm, metav1.UpdateOptions{})
// 		if resErr != nil {
// 			log.Println(resErr.Error())
// 		}
// 		return err
// 	}

// 	return nil
// }

// DeleteService deletes a service
func (kn *KnativeBackend) DeleteService(name string) error {
	if err := kn.knClientset.ServingV1().Services(kn.namespace).Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	// Delete the service's configMap
	if delErr := deleteServiceConfigMap(name, kn.namespace, kn.kubeClientset); delErr != nil {
		log.Println(delErr.Error())
	}

	// Delete all the service's jobs
	if err := deleteServiceJobs(name, kn.namespace, kn.kubeClientset); err != nil {
		log.Printf("Error deleting associated jobs for service \"%s\": %v\n", name, err)
	}

	return nil
}

// TODO
// // GetProxyDirector returns a director function to use in a httputil.ReverseProxy
// func (of *OpenfaasBackend) GetProxyDirector(serviceName string) func(req *http.Request) {
// 	return func(req *http.Request) {
// 		req.URL.Scheme = "http"
// 		req.URL.Host = of.gatewayEndpoint
// 		req.URL.Path = fmt.Sprintf("/function/%s", serviceName)
// 	}
// }

func (kn *KnativeBackend) createKNServiceDefinition(service *types.Service) (*knv1.Service, error) {
	// Add label "com.openfaas.scale.zero=true" for scaling to zero
	// TODO: add here anotation "serving.knative.dev/visibility=cluster-local"
	service.Labels[types.OpenfaasZeroScalingLabel] = "true"

	podSpec, err := service.ToPodSpec(kn.config)
	if err != nil {
		return nil, err
	}

	// fix ContainerConcurrency to 1 to avoid parallel invocations in the same container
	containerConcurrency := int64(1)

	knSvc := &knv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: kn.namespace,
		},
		Spec: knv1.ServiceSpec{
			ConfigurationSpec: knv1.ConfigurationSpec{
				Template: knv1.RevisionTemplateSpec{
					Spec: knv1.RevisionSpec{
						ContainerConcurrency: &containerConcurrency,
						PodSpec:              *podSpec,
					},
				},
			},
		},
	}

	return knSvc, nil
}

// GetKubeClientset returns the Kubernetes Clientset
func (kn *KnativeBackend) GetKubeClientset() kubernetes.Interface {
	return kn.kubeClientset
}
