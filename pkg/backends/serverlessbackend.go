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
	"github.com/grycap/oscar/v2/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// MakeServerlessBackend returns a ServerlessBackend based on the configuration
func MakeServerlessBackend(kubeClientset kubernetes.Interface, kubeConfig *rest.Config, cfg *types.Config) types.ServerlessBackend {
	switch cfg.ServerlessBackend {
	case "openfaas":
		return MakeOpenfaasBackend(kubeClientset, kubeConfig, cfg)
	case "knative":
		return MakeKnativeBackend(kubeClientset, kubeConfig, cfg)
	}

	return MakeKubeBackend(kubeClientset, cfg)
}
