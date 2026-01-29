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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type StatusInfo struct {
	Cluster ClusterInfo `json:"cluster"`
	Oscar   OscarInfo   `json:"oscar"`
	MinIO   MinioInfo   `json:"minio"`
}

// NEW STRUCTURES (CLUSTER)

type ClusterInfo struct {
	NodesCount int64          `json:"nodes_count"`
	Metrics    ClusterMetrics `json:"metrics"`
	Nodes      []NodeDetail   `json:"nodes"`
}

type ClusterMetrics struct {
	CPU    CPUMetrics    `json:"cpu"`
	Memory MemoryMetrics `json:"memory"`
	GPU    GPUMetrics    `json:"gpu"`
}

type CPUMetrics struct {
	TotalFreeCores     int64 `json:"total_free_cores"`
	MaxFreeOnNodeCores int64 `json:"max_free_on_node_cores"`
}

type MemoryMetrics struct {
	TotalFreeBytes     int64 `json:"total_free_bytes"`
	MaxFreeOnNodeBytes int64 `json:"max_free_on_node_bytes"`
}

type GPUMetrics struct {
	TotalGPU int64 `json:"total_gpu"`
}

type NodeDetail struct {
	Name        string                `json:"name"`
	CPU         NodeResource          `json:"cpu"`
	Memory      NodeResource          `json:"memory"`
	GPU         int64                 `json:"gpu"`
	IsInterlink bool                  `json:"is_interlink"`
	Status      string                `json:"status"`
	Conditions  []NodeConditionSimple `json:"conditions"`
}

type NodeResource struct {
	CapacityCores int64 `json:"capacity_cores,omitempty"`
	UsageCores    int64 `json:"usage_cores,omitempty"`
	CapacityBytes int64 `json:"capacity_bytes,omitempty"`
	UsageBytes    int64 `json:"usage_bytes,omitempty"`
}

type NodeConditionSimple struct {
	Type   string `json:"type"`
	Status bool   `json:"status"`
}

// NEW STRUCTURES (OSCAR)

type OscarInfo struct {
	DeploymentName string          `json:"deployment_name"`
	Ready          bool            `json:"ready"`
	Deployment     OscarDeployment `json:"deployment"`
	JobsCount      int             `json:"jobs_count"` // Total jobs (Active + Succeeded + Failed)
	Pods           PodStates       `json:"pods"`
	OIDC           OIDCInfo        `json:"oidc"`
}

type OscarDeployment struct {
	AvailableReplicas int32             `json:"available_replicas"`
	ReadyReplicas     int32             `json:"ready_replicas"`
	Replicas          int32             `json:"replicas"`
	CreationTimestamp metav1.Time       `json:"creation_timestamp"`
	Strategy          string            `json:"strategy"`
	Labels            map[string]string `json:"labels"`
}

type PodStates struct {
	Total  int            `json:"total"`
	States map[string]int `json:"states"`
}

type OIDCInfo struct {
	Enabled bool     `json:"enabled"`
	Issuers []string `json:"issuers"`
	Groups  []string `json:"groups"`
}

//  NEW STRUCTURES (MINIO)

type MinioInfo struct {
	BucketsCount int `json:"buckets_count"`
	TotalObjects int `json:"total_objects"`
}
