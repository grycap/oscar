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
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/grycap/oscar/v2/pkg/imagepuller"
	"github.com/grycap/oscar/v2/pkg/types"
	"github.com/grycap/oscar/v2/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	knv1 "knative.dev/serving/pkg/apis/serving/v1"
	knclientset "knative.dev/serving/pkg/client/clientset/versioned"
)

// KnativeBackend struct to represent a Knative client
type KnativeBackend struct {
	kubeClientset kubernetes.Interface
	knClientset   knclientset.Interface
	namespace     string
	config        *types.Config
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
		config:        cfg,
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

	// Create the Knative service definition
	knSvc, err := kn.createKNServiceDefinition(&service)
	if err != nil {
		// Delete the previously created configMap
		if delErr := deleteServiceConfigMap(service.Name, kn.namespace, kn.kubeClientset); delErr != nil {
			log.Println(delErr.Error())
		}
		return err
	}

	// Create the Knative service
	_, err = kn.knClientset.ServingV1().Services(kn.namespace).Create(context.TODO(), knSvc, metav1.CreateOptions{})
	if err != nil {
		// Delete the previously created configMap
		if delErr := deleteServiceConfigMap(service.Name, kn.namespace, kn.kubeClientset); delErr != nil {
			log.Println(delErr.Error())
		}
		return err
	}

	//Create an expose service
	if service.ExposeOptions.MaxReplicas != 0 {
		exposeConf := utils.Expose{
			Name:        service.Name,
			NameSpace:   kn.namespace,
			Variables:   service.Environment.Vars,
			Image:       service.Image,
			MaxReplicas: service.ExposeOptions.MaxReplicas,
		}
		if service.ExposeOptions.Port != 0 {
			exposeConf.Port = service.ExposeOptions.Port
		}
		if service.ExposeOptions.TopCPU != 0 {
			exposeConf.TopCPU = service.ExposeOptions.TopCPU
		}
		utils.CreateExpose(exposeConf, kn.kubeClientset, *kn.config)

	}
	//Create deaemonset to cache the service image on all the nodes
	if service.ImagePrefetch {
		err = imagepuller.CreateDaemonset(kn.config, service, kn.kubeClientset)
		if err != nil {
			return err
		}
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

// UpdateService updates an existent service
func (kn *KnativeBackend) UpdateService(service types.Service) error {
	// Get the old knative service
	oldSvc, err := kn.knClientset.ServingV1().Services(kn.namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Get the old service's configMap
	oldCm, err := kn.kubeClientset.CoreV1().ConfigMaps(kn.namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("the service \"%s\" does not have a registered ConfigMap", service.Name)
	}

	// Update the configMap with FDL and user-script
	if err := updateServiceConfigMap(&service, kn.namespace, kn.kubeClientset); err != nil {
		return err
	}

	// Create the Knative service definition
	knSvc, err := kn.createKNServiceDefinition(&service)
	if err != nil {
		// Restore the old configMap
		_, resErr := kn.kubeClientset.CoreV1().ConfigMaps(kn.namespace).Update(context.TODO(), oldCm, metav1.UpdateOptions{})
		if resErr != nil {
			log.Println(resErr.Error())
		}
		return err
	}

	// Set the new service's values on the old Knative service to avoid update issues
	oldSvc.ObjectMeta.Labels = knSvc.ObjectMeta.Labels
	oldSvc.Spec = knSvc.Spec
	// Update the annotations
	for k, v := range knSvc.ObjectMeta.Annotations {
		oldSvc.ObjectMeta.Annotations[k] = v
	}

	// Update the Knative service
	_, err = kn.knClientset.ServingV1().Services(kn.namespace).Update(context.TODO(), oldSvc, metav1.UpdateOptions{})
	if err != nil {
		// Restore the old configMap
		_, resErr := kn.kubeClientset.CoreV1().ConfigMaps(kn.namespace).Update(context.TODO(), oldCm, metav1.UpdateOptions{})
		if resErr != nil {
			log.Println(resErr.Error())
		}
		return err
	}

	//Update an expose service
	if service.ExposeOptions.MaxReplicas != 0 {
		exposeConf := utils.Expose{
			Name:        service.Name,
			NameSpace:   kn.namespace,
			Variables:   service.Environment.Vars,
			Image:       service.Image,
			MaxReplicas: service.ExposeOptions.MaxReplicas,
		}
		if service.ExposeOptions.Port != 0 {
			exposeConf.Port = service.ExposeOptions.Port
		}
		if service.ExposeOptions.TopCPU != 0 {
			exposeConf.TopCPU = service.ExposeOptions.TopCPU
		}
		utils.UpdateExpose(exposeConf, kn.kubeClientset)
	}

	return nil
}

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
	exposeConf := utils.Expose{
		Name:      name,
		NameSpace: kn.namespace,
		Image:     "service.Image",
	}
	if err2 := utils.DeleteExpose(exposeConf, kn.kubeClientset); err2 != nil {
		log.Printf("Error deleting all associated kubernetes component of an exposed service \"%s\": %v\n", name, err2)
	}

	return nil
}

// GetProxyDirector returns a director function to use in a httputil.ReverseProxy
func (kn *KnativeBackend) GetProxyDirector(serviceName string) func(req *http.Request) {
	return func(req *http.Request) {
		// Set the request Host parameter to avoid issues in the redirection
		// related issue: https://github.com/golang/go/issues/7682
		host := fmt.Sprintf("%s.%s", serviceName, kn.namespace)
		req.Host = host

		req.URL.Scheme = "http"
		req.URL.Host = host
		req.URL.Path = ""
	}
}

func (kn *KnativeBackend) createKNServiceDefinition(service *types.Service) (*knv1.Service, error) {
	// Add label "serving.knative.dev/visibility=cluster-local"
	// https://knative.dev/docs/serving/services/private-services/
	service.Labels[types.KnativeVisibilityLabel] = types.KnativeClusterLocalValue

	podSpec, err := service.ToPodSpec(kn.config)
	if err != nil {
		return nil, err
	}

	// fix ContainerConcurrency to 1 to avoid parallel invocations in the same container
	containerConcurrency := int64(1)

	knSvc := &knv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        service.Name,
			Namespace:   kn.namespace,
			Labels:      service.Labels,
			Annotations: service.Annotations,
		},
		Spec: knv1.ServiceSpec{
			ConfigurationSpec: knv1.ConfigurationSpec{
				Template: knv1.RevisionTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							// Set autoscaling bounds (min_scale and max_scale)
							types.KnativeMinScaleAnnotation: strconv.Itoa(service.Synchronous.MinScale),
							types.KnativeMaxScaleAnnotation: strconv.Itoa(service.Synchronous.MaxScale),
						},
					},
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
