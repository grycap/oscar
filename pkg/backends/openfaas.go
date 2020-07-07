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

	"github.com/grycap/oscar/pkg/types"
	ofclientset "github.com/openfaas/faas-netes/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// OpenfaasBackend struct to represent an Openfaas client
type OpenfaasBackend struct {
	kubeClientset   *kubernetes.Clientset
	ofClientset     *ofclientset.Clientset
	namespace       string
	gatewayEndpoint string
}

// MakeOpenfaasBackend makes a OpenfaasBackend from the provided k8S clientset and config
func MakeOpenfaasBackend(kubeClientset *kubernetes.Clientset, kubeConfig *rest.Config, cfg *types.Config) *OpenfaasBackend {
	ofClientset, err := ofclientset.NewForConfig(kubeConfig)
	if err != nil {
		log.Fatal(err)
	}

	return &OpenfaasBackend{
		kubeClientset:   kubeClientset,
		ofClientset:     ofClientset,
		namespace:       cfg.ServicesNamespace,
		gatewayEndpoint: fmt.Sprintf("http://gateway.%s:%d", cfg.OpenfaasNamespace, cfg.OpenfaasPort),
	}
}

// GetInfo
// TODO: implement
func (of *OpenfaasBackend) GetInfo() *types.ServerlessBackendInfo {
	return nil
}

// ListServices returns a slice with all services registered in the provided namespace
func (of *OpenfaasBackend) ListServices() ([]*types.Service, error) {
	// Get the list with all deployments
	deployments, err := of.kubeClientset.AppsV1().Deployments(of.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	services := []*types.Service{}
	for _, deployment := range deployments.Items {
		// Get service from configMap's FDL
		svc, err := getServiceFromFDL(deployment.Name, of.namespace, of.kubeClientset)
		if err != nil {
			log.Printf("WARNING: %v\n", err)
		} else {
			services = append(services, svc)
		}
	}

	return services, nil

}

// CreateService
// TODO: implement
func (of *OpenfaasBackend) CreateService(service types.Service) error {
	// TODO: create function and watch (list) the deployment until it is created
	// TODO: update deployment after creation... add volume
	// TODO: add label "com.openfaas.scale.zero=true" for scaling to zero
	// TODO: use ofClientset
	return nil
}

// ReadService returns a Service
func (of *OpenfaasBackend) ReadService(name string) (*types.Service, error) {
	// Check if service exists
	if _, err := of.kubeClientset.AppsV1().Deployments(of.namespace).Get(context.TODO(), name, metav1.GetOptions{}); err != nil {
		return nil, err
	}

	// Get service from configMap's FDL
	svc, err := getServiceFromFDL(name, of.namespace, of.kubeClientset)
	if err != nil {
		return nil, err
	}

	return svc, nil

}

// UpdateService
// TODO: implement
func (of *OpenfaasBackend) UpdateService(service types.Service) error {
	// TODO: update the deployment directly (with kubeClientset)
	return nil
}

// DeleteService
// TODO: implement
func (of *OpenfaasBackend) DeleteService(name string) error {
	return nil
}
