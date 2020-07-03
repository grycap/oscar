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
	"fmt"

	"github.com/grycap/oscar/pkg/types"
	ofclientset "github.com/openfaas/faas-netes/pkg/client/clientset/versioned"
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
	ofClientset, _ := ofclientset.NewForConfig(kubeConfig)

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

// ListServices
// TODO: implement
func (of *OpenfaasBackend) ListServices() ([]*types.Service, error) {
	// TODO: list deployments directly (with kubeClientset)

}

// CreateService
// TODO: implement
func (of *OpenfaasBackend) CreateService(service types.Service) error {
	// TODO: update deployment after creation... add volume
	// TODO: add label "com.openfaas.scale.zero=true" for scaling to zero
	// TODO: use ofClientset
	return nil
}

// ReadService
// TODO: implement
func (of *OpenfaasBackend) ReadService(name string) (*types.Service, error) {
	// TODO: read deployment directly (with kubeClientset)

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
