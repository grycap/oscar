// Copyright (C) GRyCAP - I3M - UPV
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backends

import (
	"github.com/grycap/oscar/pkg/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// KubeBackend struct to represent a Kubernetes client to store services as podTemplates
type KubeBackend struct {
	kubeClientset *kubernetes.Clientset
}

// MakeKubeBackend makes a KubeBackend with the provided k8s clientset
func MakeKubeBackend(kubeClientset *kubernetes.Clientset) *KubeBackend {
	return &KubeBackend{
		kubeClientset: kubeClientset,
	}
}

// GetServicePodSpec returns a k8s podSpec for the service from the serverless backend
func (k *KubeBackend) GetServicePodSpec(name, namespace string) (*v1.PodSpec, error) {
	podTemplate, err := k.kubeClientset.CoreV1().PodTemplates(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return &podTemplate.Template.Spec, nil

}

// GetInfo returns the ServerlessBackendInfo with the name and version
func (k *KubeBackend) GetInfo() *types.ServerlessBackendInfo {
	// As this ServerlessBackend stores the Services in k8s, the BackendInfo is not needed
	// because types.Info already shows the kubernetes version of the system
	return nil
}

// ListServices returns a slice with all the services registered in the provided namespace
func (k *KubeBackend) ListServices(namespace string) ([]types.Service, error) {
	podTemplates, err := k.kubeClientset.CoreV1().PodTemplates(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	services := []types.Service{}
	for _, podTemplate := range podTemplates.Items {
		services = append(services, types.Service{
			Name: podTemplate.Name,
		})
	}

	return services, nil
}

// CreateService create a new service as a k8s podTemplate
func (k *KubeBackend) CreateService(service types.Service) error {

}

//
func (k *KubeBackend) ReadService(name, namespace string) (*types.Service, error) {

}

//
func (k *KubeBackend) UpdateService(service types.Service) error {

}

//
func (k *KubeBackend) DeleteService(name, namespace string) error {

}

//func podSpecToService
