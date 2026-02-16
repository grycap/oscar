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

	"github.com/grycap/oscar/v3/pkg/backends/resources"
	"github.com/grycap/oscar/v3/pkg/imagepuller"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	kserveclient "github.com/kserve/kserve/pkg/client/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	knv1 "knative.dev/serving/pkg/apis/serving/v1"
	knclientset "knative.dev/serving/pkg/client/clientset/versioned"
)

// Custom logger - uncomment if needed
// var knativeLogger = log.New(os.Stdout, "[KNATIVE] ", log.Flags())

// KnativeBackend struct to represent a Knative client
type KnativeBackend struct {
	kubeClientset   kubernetes.Interface
	knClientset     knclientset.Interface
	kserveClientset *kserveclient.Clientset
	config          *types.Config
}

// MakeKnativeBackend makes a KnativeBackend from the provided k8S clientset and config
func MakeKnativeBackend(kubeClientset kubernetes.Interface, kubeConfig *rest.Config, cfg *types.Config) *KnativeBackend {
	knClientset, err := knclientset.NewForConfig(kubeConfig)
	if err != nil {
		log.Fatal(err)
	}

	var kserveClientset *kserveclient.Clientset
	if cfg.KserveEnable {
		kserveClientset, err = kserveclient.NewForConfig(kubeConfig)
		if err != nil {
			log.Fatal(err)
		}
	}

	return &KnativeBackend{
		kubeClientset:   kubeClientset,
		knClientset:     knClientset,
		kserveClientset: kserveClientset,
		config:          cfg,
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
func (kn *KnativeBackend) ListServices(namespaces ...string) ([]*types.Service, error) {
	// Get the list with all Knative services
	configmaps, err := getAllServicesConfigMaps(kn.kubeClientset, namespaces...)
	if err != nil {
		log.Printf("WARNING: %v\n", err)
		return nil, err
	}
	services := []*types.Service{}

	for _, cm := range configmaps.Items {
		service, err := getServiceFromConfigMap(&cm) // #nosec G601
		if err != nil {
			return nil, err
		}
		service.Namespace = cm.Namespace
		services = append(services, service)
	}
	return services, nil
}

// CreateService creates a new service as a Knative service
func (kn *KnativeBackend) CreateService(service types.Service) error {
	namespace := service.Namespace
	if namespace == "" {
		namespace = kn.config.ServicesNamespace
	}
	var isKserve bool = (kn.kserveClientset != nil && utils.IsKserveService(&service))

	if isKserve {
		if service.Environment.Vars == nil {
			service.Environment.Vars = make(map[string]string)
		}
		// TODO: Replace value inyection method
		service.Environment.Vars["KSERVE_MODEL_NAME"] = service.Name
		service.Environment.Vars["KSERVE_HOST"] = fmt.Sprintf("%s.%s.svc.cluster.local", utils.KservePredictor(service.Name), namespace)
	}

	// Check if there is some user defined settings for OSCAR
	err := checkAdditionalConfig(ConfigMapNameOSCAR, kn.config.ServicesNamespace, service, kn.config, kn.kubeClientset)
	if err != nil {
		return err
	}

	// Create the configMap with FDL and user-script
	err = createServiceConfigMap(&service, namespace, kn.kubeClientset)
	if err != nil {
		return err
	}

	// Create the Knative service definition
	knSvc, err := kn.createKNServiceDefinition(&service, namespace)
	if err != nil {
		// Delete the previously created configMap
		if delErr := deleteServiceConfigMap(service.Name, namespace, kn.kubeClientset); delErr != nil {
			log.Println(delErr.Error())
		}
		return err
	}

	// Create the Knative service
	createdKnSvc, err := kn.knClientset.ServingV1().Services(namespace).Create(context.TODO(), knSvc, metav1.CreateOptions{})
	if err != nil {
		// Delete the previously created configMap
		if delErr := deleteServiceConfigMap(service.Name, namespace, kn.kubeClientset); delErr != nil {
			log.Println(delErr.Error())
		}
		return err
	}

	// If the service is a KServe service, create the associated InferenceService
	if isKserve {
		// The Kserve service set an OwnerReference to the Knative service, so if the Knative service is deleted the KServe InferenceService will be automatically deleted by Kubernetes garbage collection
		_, err := utils.CreateKserveInferenceService(kn.kserveClientset, &service, createdKnSvc)
		if err != nil {
			if knSvcDelErr := kn.knClientset.ServingV1().Services(namespace).Delete(context.TODO(), knSvc.Name, metav1.DeleteOptions{}); err != nil {
				log.Println(knSvcDelErr.Error())
			}
			if delErr := deleteServiceConfigMap(service.Name, namespace, kn.kubeClientset); delErr != nil {
				log.Println(delErr.Error())
			}
			if utils.SecretExists(knSvc.Name, namespace, kn.kubeClientset) {
				secretsErr := utils.DeleteSecret(knSvc.Name, namespace, kn.kubeClientset)
				if secretsErr != nil {
					log.Printf("Error deleting asociated secret: %v", secretsErr)
				}
			}
			return err
		}
	}

	//Create an expose service
	if service.Expose.APIPort != 0 {
		err = resources.CreateExpose(service, namespace, kn.kubeClientset, kn.config)
		if err != nil {
			return err
		}
	}
	//Create deaemonset to cache the service image on all the nodes
	if service.ImagePrefetch {
		err = imagepuller.CreateDaemonset(kn.config, service, namespace, kn.kubeClientset)
		if err != nil {
			return err
		}
	}

	return nil
}

// ReadService returns a Service
func (kn *KnativeBackend) ReadService(namespace, name string) (*types.Service, error) {
	serviceNamespace := namespace
	var err error
	if serviceNamespace == "" {
		serviceNamespace, err = kn.resolveServiceNamespace(name)
		if err != nil {
			return nil, err
		}
	}

	// Check if service exists
	if _, err := kn.knClientset.ServingV1().Services(serviceNamespace).Get(context.TODO(), name, metav1.GetOptions{}); err != nil {
		return nil, err
	}

	// Get the configMap of the Service
	cm, err := kn.kubeClientset.CoreV1().ConfigMaps(serviceNamespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("the service \"%s\" does not have a registered ConfigMap", name)
	}
	// Get service from configMap's FDL
	svc, err := getServiceFromConfigMap(cm)
	if err != nil {
		return nil, err
	}

	svc.Namespace = serviceNamespace

	return svc, nil
}

// UpdateService updates an existent service
func (kn *KnativeBackend) UpdateService(service types.Service) error {
	namespace := service.Namespace
	if namespace == "" {
		namespace = kn.config.ServicesNamespace
	}

	// Check if there is some user defined settings for OSCAR
	if err := checkAdditionalConfig(ConfigMapNameOSCAR, kn.config.ServicesNamespace, service, kn.config, kn.kubeClientset); err != nil {
		return err
	}

	// Get the old knative service
	oldSvc, err := kn.knClientset.ServingV1().Services(namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	// Get the old service's configMap
	oldCm, err := kn.kubeClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("the service \"%s\" does not have a registered ConfigMap", service.Name)
	}

	// Update the configMap with FDL and user-script
	if err := updateServiceConfigMap(&service, namespace, kn.kubeClientset); err != nil {
		return err
	}

	// Create the Knative service definition
	knSvc, err := kn.createKNServiceDefinition(&service, namespace)
	if err != nil {
		// Restore the old configMap
		_, resErr := kn.kubeClientset.CoreV1().ConfigMaps(namespace).Update(context.TODO(), oldCm, metav1.UpdateOptions{})
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
	_, err = kn.knClientset.ServingV1().Services(namespace).Update(context.TODO(), oldSvc, metav1.UpdateOptions{})
	if err != nil {
		// Restore the old configMap
		_, resErr := kn.kubeClientset.CoreV1().ConfigMaps(namespace).Update(context.TODO(), oldCm, metav1.UpdateOptions{})
		if resErr != nil {
			log.Println(resErr.Error())
		}
		return err
	}

	// If the service is exposed update its configuration
	if service.Expose.APIPort != 0 {
		err = resources.UpdateExpose(service, namespace, kn.kubeClientset, kn.config)
		if err != nil {
			return err
		}
	}

	//Create deaemonset to cache the service image on all the nodes
	if service.ImagePrefetch {
		err = imagepuller.CreateDaemonset(kn.config, service, namespace, kn.kubeClientset)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteService deletes a service
func (kn *KnativeBackend) DeleteService(service types.Service) error {

	name := service.Name
	namespace := service.Namespace
	if namespace == "" {
		namespace = kn.config.ServicesNamespace
	}
	if err := kn.knClientset.ServingV1().Services(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	// Delete the service's configMap
	if delErr := deleteServiceConfigMap(name, namespace, kn.kubeClientset); delErr != nil {
		log.Println(delErr.Error())
	}

	// Delete all the service's jobs
	if err := deleteServiceJobs(name, namespace, kn.kubeClientset); err != nil {
		log.Printf("Error deleting associated jobs for service \"%s\": %v\n", name, err)
	}

	if utils.SecretExists(name, namespace, kn.kubeClientset) {
		secretsErr := utils.DeleteSecret(name, namespace, kn.kubeClientset)
		if secretsErr != nil {
			log.Printf("Error deleting asociated secret: %v", secretsErr)
		}
	}

	// If service is exposed delete the exposed k8s components
	if service.Expose.APIPort != 0 {
		if err := resources.DeleteExpose(name, namespace, kn.kubeClientset, kn.config); err != nil {
			log.Printf("Error deleting all associated kubernetes component of an exposed service \"%s\": %v\n", name, err)
		}
	}

	return nil
}

// GetProxyDirector returns a director function to use in a httputil.ReverseProxy
func (kn *KnativeBackend) GetProxyDirector(serviceName string) func(req *http.Request) {
	return func(req *http.Request) {
		namespace := kn.config.ServicesNamespace
		if resolved, err := kn.resolveServiceNamespace(serviceName); err == nil && resolved != "" {
			namespace = resolved
		}

		// Set the request Host parameter to avoid issues in the redirection
		// related issue: https://github.com/golang/go/issues/7682
		host := fmt.Sprintf("%s.%s", serviceName, namespace)
		req.Host = host

		req.URL.Scheme = "http"
		req.URL.Host = host
		req.URL.Path = ""
	}
}

func (kn *KnativeBackend) resolveServiceNamespace(name string) (string, error) {
	listOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", types.ServiceLabel, name),
	}
	configmaps, err := kn.kubeClientset.CoreV1().ConfigMaps("").List(context.TODO(), listOpts)
	if err != nil {
		return "", err
	}

	if len(configmaps.Items) == 0 {
		return "", apierrors.NewNotFound(v1.Resource("configmap"), name)
	}

	if len(configmaps.Items) > 1 {
		return "", fmt.Errorf("service \"%s\" found in multiple namespaces", name)
	}

	return configmaps.Items[0].Namespace, nil
}

func (kn *KnativeBackend) createKNServiceDefinition(service *types.Service, namespace string) (*knv1.Service, error) {
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
			Namespace:   namespace,
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
						//Empty labels map to avoid nil pointer errors
						Labels: map[string]string{},
					},
					Spec: knv1.RevisionSpec{
						ContainerConcurrency: &containerConcurrency,
						PodSpec:              *podSpec,
					},
				},
			},
		},
	}
	// Add secrets as environment variables if defined
	if utils.SecretExists(service.Name, namespace, kn.GetKubeClientset()) {
		podSpec.Containers[0].EnvFrom = []v1.EnvFromSource{
			{
				SecretRef: &v1.SecretEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: service.Name,
					},
				},
			},
		}
	}
	// Add to the service labels the user VO for accounting on knative pods
	if service.Labels["vo"] != "" {
		knSvc.Spec.ConfigurationSpec.Template.ObjectMeta.Labels["vo"] = service.Labels["vo"]
	}
	if service.Labels["kueue.x-k8s.io/queue-name"] != "" {
		knSvc.Spec.ConfigurationSpec.Template.ObjectMeta.Labels["kueue.x-k8s.io/queue-name"] = service.Labels["kueue.x-k8s.io/queue-name"]
	}
	if service.EnableSGX {
		knSvc.Spec.ConfigurationSpec.Template.ObjectMeta.Annotations["kubernetes.podspec-securitycontext"] = "enabled"
		knSvc.Spec.ConfigurationSpec.Template.ObjectMeta.Annotations["kubernetes.containerspec-addcapabilities"] = "enabled"
	}

	return knSvc, nil
}

// GetKubeClientset returns the Kubernetes Clientset
func (kn *KnativeBackend) GetKubeClientset() kubernetes.Interface {
	return kn.kubeClientset
}
