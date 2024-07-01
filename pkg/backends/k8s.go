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
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/grycap/oscar/v3/pkg/imagepuller"
	"github.com/grycap/oscar/v3/pkg/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const ConfigMapNameOSCAR = "additional-oscar-config"

// KubeBackend struct to represent a Kubernetes client to store services as podTemplates
type KubeBackend struct {
	kubeClientset kubernetes.Interface
	namespace     string
	config        *types.Config
}

// MakeKubeBackend makes a KubeBackend with the provided k8s clientset
func MakeKubeBackend(kubeClientset kubernetes.Interface, cfg *types.Config) *KubeBackend {
	return &KubeBackend{
		kubeClientset: kubeClientset,
		namespace:     cfg.ServicesNamespace,
		config:        cfg,
	}
}

// GetInfo returns the ServerlessBackendInfo with the name and version
func (k *KubeBackend) GetInfo() *types.ServerlessBackendInfo {
	// As this ServerlessBackend stores the Services in k8s, the BackendInfo is not needed
	// because types.Info already shows the kubernetes version of the system
	return nil
}

// ListServices returns a slice with all services registered in the provided namespace
func (k *KubeBackend) ListServices() ([]*types.Service, error) {
	// Get the list with all podTemplates
	podTemplates, err := k.kubeClientset.CoreV1().PodTemplates(k.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	services := []*types.Service{}
	for _, podTemplate := range podTemplates.Items {
		// Get service from configMap's FDL
		svc, err := getServiceFromFDL(podTemplate.Name, k.namespace, k.kubeClientset)
		if err != nil {
			log.Printf("WARNING: %v\n", err)
		} else {
			services = append(services, svc)
		}
	}

	return services, nil
}

// CreateService creates a new service as a k8s podTemplate
func (k *KubeBackend) CreateService(service types.Service) error {

	// Check if there is some user defined settings for OSCAR
	err := checkAdditionalConfig(ConfigMapNameOSCAR, k.namespace, service, k.config, k.kubeClientset)
	if err != nil {
		return err
	}

	// Create the configMap with FDL and user-script
	err = createServiceConfigMap(&service, k.namespace, k.kubeClientset)
	if err != nil {
		return err
	}

	// Create podSpec from the service
	podSpec, err := service.ToPodSpec(k.config)
	if err != nil {
		// Delete the previously created configMap
		if delErr := deleteServiceConfigMap(service.Name, k.namespace, k.kubeClientset); delErr != nil {
			log.Println(delErr.Error())
		}
		return err
	}

	// Create the podTemplate spec
	podTemplate := &v1.PodTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:        service.Name,
			Namespace:   k.namespace,
			Labels:      service.Labels,
			Annotations: service.Annotations,
		},
		Template: v1.PodTemplateSpec{
			Spec: *podSpec,
		},
	}
	_, err = k.kubeClientset.CoreV1().PodTemplates(k.namespace).Create(context.TODO(), podTemplate, metav1.CreateOptions{})
	if err != nil {
		// Delete the previously created configMap
		if delErr := deleteServiceConfigMap(service.Name, k.namespace, k.kubeClientset); delErr != nil {
			log.Println(delErr.Error())
		}
		return err
	}

	//Create an expose service
	if service.Expose.APIPort != 0 {
		err = types.CreateExpose(service, k.kubeClientset, k.config)
		if err != nil {
			return err
		}
	}
	//Create deaemonset to cache the service image on all the nodes
	if service.ImagePrefetch {
		err = imagepuller.CreateDaemonset(k.config, service, k.kubeClientset)
		if err != nil {
			return err
		}
	}

	return nil
}

// ReadService returns a Service
func (k *KubeBackend) ReadService(name string) (*types.Service, error) {
	// Check if service exists
	if _, err := k.kubeClientset.CoreV1().PodTemplates(k.namespace).Get(context.TODO(), name, metav1.GetOptions{}); err != nil {
		return nil, err
	}

	// Get service from configMap's FDL
	svc, err := getServiceFromFDL(name, k.namespace, k.kubeClientset)
	if err != nil {
		return nil, err
	}

	return svc, nil
}

// UpdateService updates an existent service
func (k *KubeBackend) UpdateService(service types.Service) error {
	// Get the old service's configMap
	oldCm, err := k.kubeClientset.CoreV1().ConfigMaps(k.namespace).Get(context.TODO(), service.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("the service \"%s\" does not have a registered ConfigMap", service.Name)
	}

	// Update the configMap with FDL and user-script
	if err := updateServiceConfigMap(&service, k.namespace, k.kubeClientset); err != nil {
		return err
	}

	// Create podSpec from the service
	podSpec, err := service.ToPodSpec(k.config)
	if err != nil {
		// Restore the old configMap
		_, resErr := k.kubeClientset.CoreV1().ConfigMaps(k.namespace).Update(context.TODO(), oldCm, metav1.UpdateOptions{})
		if resErr != nil {
			log.Println(resErr.Error())
		}
		return err
	}

	// Create the podTemplate spec
	podTemplate := &v1.PodTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: k.namespace,
			Labels: map[string]string{
				types.ServiceLabel: service.Name,
			},
		},
		Template: v1.PodTemplateSpec{
			Spec: *podSpec,
		},
	}
	_, err = k.kubeClientset.CoreV1().PodTemplates(k.namespace).Update(context.TODO(), podTemplate, metav1.UpdateOptions{})
	if err != nil {
		// Restore the old configMap
		_, resErr := k.kubeClientset.CoreV1().ConfigMaps(k.namespace).Update(context.TODO(), oldCm, metav1.UpdateOptions{})
		if resErr != nil {
			log.Println(resErr.Error())
		}
		return err
	}

	// If the service is exposed update its configuration
	if service.Expose.APIPort != 0 {
		err = types.UpdateExpose(service, k.kubeClientset, k.config)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteService deletes a service
func (k *KubeBackend) DeleteService(service types.Service) error {
	name := service.Name
	if err := k.kubeClientset.CoreV1().PodTemplates(k.namespace).Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	// Delete the service's configMap
	if delErr := deleteServiceConfigMap(name, k.namespace, k.kubeClientset); delErr != nil {
		log.Println(delErr.Error())
	}

	// Delete all the service's jobs
	if err := deleteServiceJobs(name, k.namespace, k.kubeClientset); err != nil {
		log.Printf("Error deleting associated jobs for service \"%s\": %v\n", name, err)
	}

	// If service is exposed delete the exposed k8s components
	if service.Expose.APIPort != 0 {
		if err := types.DeleteExpose(name, k.kubeClientset, k.config); err != nil {
			log.Printf("Error deleting all associated kubernetes component of the expose config \"%s\": %v\n", name, err)
		}
	}

	return nil
}

func getServiceFromFDL(name string, namespace string, kubeClientset kubernetes.Interface) (*types.Service, error) {
	// Get the configMap of the Service
	cm, err := kubeClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("the service \"%s\" does not have a registered ConfigMap", name)
	}
	service := &types.Service{}

	// Unmarshal the FDL stored in the configMap
	if err = yaml.Unmarshal([]byte(cm.Data[types.FDLFileName]), service); err != nil {
		return nil, fmt.Errorf("the FDL of the service \"%s\" cannot be read", name)
	}

	// Add the script to the service from configmap's script value
	service.Script = cm.Data[types.ScriptFileName]

	return service, nil
}

func checkAdditionalConfig(configName string, configNamespace string, service types.Service, cfg *types.Config, kubeClientset kubernetes.Interface) error {
	// Get the configMapwith the service additional settings
	cm, err := kubeClientset.CoreV1().ConfigMaps(configNamespace).Get(context.TODO(), configName, metav1.GetOptions{})
	if err != nil {
		return nil
	}

	additionalConfig := &types.AdditionalConfig{}
	// Unmarshal the FDL stored in the configMap
	if err = yaml.Unmarshal([]byte(cm.Data[cfg.AdditionalConfigPath]), additionalConfig); err != nil {
		return nil
	}

	if len(additionalConfig.Images.AllowedPrefixes) > 0 {
		for _, prefix := range additionalConfig.Images.AllowedPrefixes {
			if !strings.Contains(service.Image, prefix) {
				return fmt.Errorf("image %s is not allowed for pull on the cluster. Check the additional configuration file on '%s'", service.Image, cfg.AdditionalConfigPath)
			}
		}
	}

	return nil
}

func createServiceConfigMap(service *types.Service, namespace string, kubeClientset kubernetes.Interface) error {
	// Copy script from service
	script := service.Script

	// Clear script from YAML
	service.Script = ""

	// Create FDL YAML
	fdl, err := service.ToYAML()
	if err != nil {
		return err
	}

	// Create ConfigMap
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: namespace,
			Labels: map[string]string{
				types.ServiceLabel: service.Name,
			},
		},
		Data: map[string]string{
			types.ScriptFileName: script,
			types.FDLFileName:    fdl,
		},
	}
	_, err = kubeClientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func updateServiceConfigMap(service *types.Service, namespace string, kubeClientset kubernetes.Interface) error {
	// Copy script from service
	script := service.Script

	// Clear script from YAML
	service.Script = ""

	// Create FDL YAML
	fdl, err := service.ToYAML()
	if err != nil {
		return err
	}

	// Create ConfigMap
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: namespace,
			Labels: map[string]string{
				types.ServiceLabel: service.Name,
			},
		},
		Data: map[string]string{
			types.ScriptFileName: script,
			types.FDLFileName:    fdl,
		},
	}
	_, err = kubeClientset.CoreV1().ConfigMaps(namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func deleteServiceConfigMap(name string, namespace string, kubeClientset kubernetes.Interface) error {
	err := kubeClientset.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

func deleteServiceJobs(name string, namespace string, kubeClientset kubernetes.Interface) error {
	// ListOptions to select all the associated jobs with the specified service
	listOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", types.ServiceLabel, name),
	}

	// Create DeleteOptions and configure PropagationPolicy for deleting associated pods in background
	background := metav1.DeletePropagationBackground
	delOpts := metav1.DeleteOptions{
		PropagationPolicy: &background,
	}

	// Delete jobs
	err := kubeClientset.BatchV1().Jobs(namespace).DeleteCollection(context.TODO(), delOpts, listOpts)
	if err != nil {
		return err
	}

	return nil
}

// GetKubeClientset returns the Kubernetes Clientset
func (k *KubeBackend) GetKubeClientset() kubernetes.Interface {
	return k.kubeClientset
}
