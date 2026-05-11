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
	ClusterQueue string                 `json:"cluster_queue,omitempty"`
	Resources    map[string]QuotaValues `json:"resources,omitempty"`
	Volumes      *VolumeQuotaResponse   `json:"volumes,omitempty"`
}

type QuotaValues struct {
	Max  int64 `json:"max"`
	Used int64 `json:"used"`
}

type VolumeQuotaResponse struct {
	// Disk contains the user-visible storage quota for OSCAR-managed volumes.
	Disk VolumeQuotaValues `json:"disk"`
	// Volumes contains the user-visible count quota for OSCAR-managed volumes.
	Volumes          VolumeQuotaValues `json:"volumes"`
	MaxDiskperVolume string            `json:"max_disk_per_volume"`
	MinDiskperVolume string            `json:"min_disk_per_volume"`
}

type VolumeQuotaValues struct {
	Max  string `json:"max"`
	Used string `json:"used"`
}

type QuotaUpdateRequest struct {
	CPU     string             `json:"cpu"`
	Memory  string             `json:"memory"`
	Volumes *VolumeQuotaUpdate `json:"volumes,omitempty"`
}

type VolumeQuotaUpdate struct {
	// Disk sets the user-visible storage quota for OSCAR-managed volumes.
	Disk string `json:"disk,omitempty"`
	// Volumes sets the user-visible count quota for OSCAR-managed volumes.
	Volumes          string `json:"volumes,omitempty"`
	MaxDiskperVolume string `json:"max_disk_per_volume,omitempty"`
	MinDiskperVolume string `json:"min_disk_per_volume,omitempty"`
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
