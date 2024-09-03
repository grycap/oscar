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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	ofv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
	ofclientset "github.com/openfaas/faas-netes/pkg/client/clientset/versioned"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var errOpenfaasOperator = errors.New("the OpenFaaS Operator is not creating the service deployment")

// OpenfaasBackend struct to represent an Openfaas client
type OpenfaasBackend struct {
	kubeClientset   kubernetes.Interface
	ofClientset     *ofclientset.Clientset
	namespace       string
	gatewayEndpoint string
	scaler          *utils.OpenfaasScaler
	config          *types.Config
}

// MakeOpenfaasBackend makes a OpenfaasBackend from the provided k8S clientset and config
func MakeOpenfaasBackend(kubeClientset kubernetes.Interface, kubeConfig *rest.Config, cfg *types.Config) *OpenfaasBackend {
	ofClientset, err := ofclientset.NewForConfig(kubeConfig)
	if err != nil {
		log.Fatal(err)
	}

	return &OpenfaasBackend{
		kubeClientset:   kubeClientset,
		ofClientset:     ofClientset,
		namespace:       cfg.ServicesNamespace,
		gatewayEndpoint: fmt.Sprintf("gateway.%s:%d", cfg.OpenfaasNamespace, cfg.OpenfaasPort),
		scaler:          utils.NewOFScaler(kubeClientset, cfg),
		config:          cfg,
	}
}

// GetInfo returns the ServerlessBackendInfo with the name and version
func (of *OpenfaasBackend) GetInfo() *types.ServerlessBackendInfo {
	backInfo := &types.ServerlessBackendInfo{
		Name: "OpenFaaS",
	}

	version, err := of.ofClientset.Discovery().ServerVersion()
	if err == nil {
		backInfo.Version = version.GitVersion
	}

	return backInfo
}

// ListServices returns a slice with all services registered in the provided namespace
func (of *OpenfaasBackend) ListServices() ([]*types.Service, error) {
	// Get the list with all Knative services
	configmaps, err := getAllServicesConfigMaps(of.namespace, of.kubeClientset)
	if err != nil {
		log.Printf("WARNING: %v\n", err)
		return nil, err
	}
	services := []*types.Service{}

	for _, cm := range configmaps.Items {
		service, err := getServiceFromConfigMap(&cm)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}

	return services, nil
}

// CreateService creates a new service as a OpenFaaS function
func (of *OpenfaasBackend) CreateService(service types.Service) error {
	// Create the configMap with FDL and user-script
	err := createServiceConfigMap(&service, of.namespace, of.kubeClientset)
	if err != nil {
		return err
	}

	// Check if a deployment of the function was already created to get its ResourceVersion
	var resourceVersion string
	oldDeploy, err := of.kubeClientset.AppsV1().Deployments(of.namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
	if err == nil {
		resourceVersion = oldDeploy.ResourceVersion
	}

	// Create the Function through the OpenFaaS operator
	function := of.createOFFunctionDefinition(&service)
	_, err = of.ofClientset.OpenfaasV1().Functions(of.namespace).Create(context.TODO(), function, metav1.CreateOptions{})
	if err != nil {
		// Delete the previously created configMap
		if delErr := deleteServiceConfigMap(service.Name, of.namespace, of.kubeClientset); delErr != nil {
			log.Println(delErr.Error())
		}
		return err
	}

	// Watch for deployment changes in services namespace
	var timeoutSeconds int64 = 30
	var deploymentCreated = false
	listOpts := metav1.ListOptions{
		TimeoutSeconds:  &timeoutSeconds,
		Watch:           true,
		ResourceVersion: resourceVersion,
	}
	watcher, err := of.kubeClientset.AppsV1().Deployments(of.namespace).Watch(context.TODO(), listOpts)
	if err != nil {
		// Delete the function
		delErr := of.ofClientset.OpenfaasV1().Functions(of.namespace).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
		if delErr != nil {
			log.Println(delErr.Error())
		}
		// Delete the previously created configMap
		if delErr := deleteServiceConfigMap(service.Name, of.namespace, of.kubeClientset); delErr != nil {
			log.Println(delErr.Error())
		}
		return err
	}
	ch := watcher.ResultChan()
	for event := range ch {
		deploy, ok := event.Object.(*appsv1.Deployment)
		if ok {
			if event.Type == watch.Added && deploy.Name == service.Name {
				deploymentCreated = true
				break
			}
		}
	}
	watcher.Stop()
	// Return an error if the OpenFaaS Operator doesn't create the deployment
	if !deploymentCreated {
		// Delete the function
		delErr := of.ofClientset.OpenfaasV1().Functions(of.namespace).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
		if delErr != nil {
			log.Println(delErr.Error())
		}
		// Delete the previously created configMap
		if delErr := deleteServiceConfigMap(service.Name, of.namespace, of.kubeClientset); delErr != nil {
			log.Println(delErr.Error())
		}
		return errOpenfaasOperator
	}

	// Create podSpec from the service
	podSpec, err := service.ToPodSpec(of.config)
	if err != nil {
		// Delete the function
		delErr := of.ofClientset.OpenfaasV1().Functions(of.namespace).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
		if delErr != nil {
			log.Println(delErr.Error())
		}
		// Delete the previously created configMap
		if delErr := deleteServiceConfigMap(service.Name, of.namespace, of.kubeClientset); delErr != nil {
			log.Println(delErr.Error())
		}
		return err
	}

	// Create JSON Patch
	// https://tools.ietf.org/html/rfc6902
	patch := []struct {
		Op    string      `json:"op"`
		Path  string      `json:"path"`
		Value *v1.PodSpec `json:"value"`
	}{
		{
			Op:    "replace",
			Path:  "/spec/template/spec",
			Value: podSpec,
		},
	}
	jsonPatch, _ := json.Marshal(patch)

	// Update the deployment
	_, err = of.kubeClientset.AppsV1().Deployments(of.namespace).Patch(context.TODO(), service.Name, k8stypes.JSONPatchType, jsonPatch, metav1.PatchOptions{})
	if err != nil {
		// Delete the function
		delErr := of.ofClientset.OpenfaasV1().Functions(of.namespace).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
		if delErr != nil {
			log.Println(delErr.Error())
		}
		// Delete the previously created configMap
		if delErr := deleteServiceConfigMap(service.Name, of.namespace, of.kubeClientset); delErr != nil {
			log.Println(delErr.Error())
		}
		return err
	}

	return nil
}

// ReadService returns a Service
func (of *OpenfaasBackend) ReadService(name string) (*types.Service, error) {
	// Check if service exists
	if _, err := of.kubeClientset.AppsV1().Deployments(of.namespace).Get(context.TODO(), name, metav1.GetOptions{}); err != nil {
		return nil, err
	}

	// Get the configMap of the Service
	cm, err := of.kubeClientset.CoreV1().ConfigMaps(of.namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("the service \"%s\" does not have a registered ConfigMap", name)
	}
	// Get service from configMap's FDL
	svc, err := getServiceFromConfigMap(cm)
	if err != nil {
		return nil, err
	}

	return svc, nil
}

// UpdateService updates an existent service
func (of *OpenfaasBackend) UpdateService(service types.Service) error {
	// Get the old service's configMap
	oldCm, err := of.kubeClientset.CoreV1().ConfigMaps(of.namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("the service \"%s\" does not have a registered ConfigMap", service.Name)
	}

	// Update the configMap with FDL and user-script
	if err := updateServiceConfigMap(&service, of.namespace, of.kubeClientset); err != nil {
		return err
	}

	// Create podSpec from the service
	podSpec, err := service.ToPodSpec(of.config)
	if err != nil {
		// Restore the old configMap
		_, resErr := of.kubeClientset.CoreV1().ConfigMaps(of.namespace).Update(context.TODO(), oldCm, metav1.UpdateOptions{})
		if resErr != nil {
			log.Println(resErr.Error())
		}
		return err
	}

	// Get the service's deployment to update its podSpec
	deployment, err := of.kubeClientset.AppsV1().Deployments(of.namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
	if err != nil {
		// Restore the old configMap
		_, resErr := of.kubeClientset.CoreV1().ConfigMaps(of.namespace).Update(context.TODO(), oldCm, metav1.UpdateOptions{})
		if resErr != nil {
			log.Println(resErr.Error())
		}
		return err
	}

	// Update podSpec in the deployment
	deployment.Spec.Template.Spec = *podSpec

	// Update the deployment
	_, err = of.kubeClientset.AppsV1().Deployments(of.namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		// Restore the old configMap
		_, resErr := of.kubeClientset.CoreV1().ConfigMaps(of.namespace).Update(context.TODO(), oldCm, metav1.UpdateOptions{})
		if resErr != nil {
			log.Println(resErr.Error())
		}
		return err
	}

	return nil
}

// DeleteService deletes a service
func (of *OpenfaasBackend) DeleteService(service types.Service) error {
	name := service.Name
	if err := of.ofClientset.OpenfaasV1().Functions(of.namespace).Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	// Delete the service's configMap
	if delErr := deleteServiceConfigMap(name, of.namespace, of.kubeClientset); delErr != nil {
		log.Println(delErr.Error())
	}

	// Delete all the service's jobs
	if err := deleteServiceJobs(name, of.namespace, of.kubeClientset); err != nil {
		log.Printf("Error deleting associated jobs for service \"%s\": %v\n", name, err)
	}

	return nil
}

// GetProxyDirector returns a director function to use in a httputil.ReverseProxy
func (of *OpenfaasBackend) GetProxyDirector(serviceName string) func(req *http.Request) {
	return func(req *http.Request) {
		req.URL.Scheme = "http"
		req.URL.Host = of.gatewayEndpoint
		req.URL.Path = fmt.Sprintf("/function/%s", serviceName)
	}
}

func (of *OpenfaasBackend) createOFFunctionDefinition(service *types.Service) *ofv1.Function {
	// Add label "com.openfaas.scale.zero=true" for scaling to zero
	service.Labels[types.OpenfaasZeroScalingLabel] = "true"

	return &ofv1.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: of.namespace,
		},
		Spec: ofv1.FunctionSpec{
			Image:       service.Image,
			Name:        service.Name,
			Annotations: &service.Annotations,
			Labels:      &service.Labels,
		},
	}
}

// StartScaler starts the OpenFaaS Scaler
func (of *OpenfaasBackend) StartScaler() {
	of.scaler.Start()
}

// GetKubeClientset returns the Kubernetes Clientset
func (of *OpenfaasBackend) GetKubeClientset() kubernetes.Interface {
	return of.kubeClientset
}
