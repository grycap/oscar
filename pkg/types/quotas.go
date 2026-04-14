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
package types

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kueueclientset "sigs.k8s.io/kueue/client-go/clientset/versioned"
)

type QuotaBackend struct {
	Kueueclient   *kueueclientset.Clientset
	KubeClientset kubernetes.Interface
}

type QuotaResponse struct {
	UserID       string                 `json:"user_id"`
	ClusterQueue string                 `json:"cluster_queue"`
	Resources    map[string]QuotaValues `json:"resources"`
}

type QuotaValues struct {
	Max  int64 `json:"max"`
	Used int64 `json:"used"`
}

type QuotaUpdateRequest struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

func CreateQuotaBackend(kubeConfig *rest.Config, kubeClientset *kubernetes.Clientset) *QuotaBackend {
	client, err := kueueclientset.NewForConfig(kubeConfig)
	if err != nil {
		// #nosec
		fmt.Errorf("creating kueue client: %w", err)
		return nil
	}
	qb := QuotaBackend{
		Kueueclient:   client,
		KubeClientset: kubeClientset,
	}
	return &qb
}
