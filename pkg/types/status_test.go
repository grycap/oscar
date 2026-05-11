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
	"encoding/json"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStatusInfoJSONSerialization(t *testing.T) {
	now := metav1.Now()

	statusInfo := StatusInfo{
		Cluster: ClusterInfo{
			NodesCount: 3,
			Metrics: ClusterMetrics{
				CPU: CPUMetrics{
					TotalFreeCores:     8,
					MaxFreeOnNodeCores: 4,
				},
				Memory: MemoryMetrics{
					TotalFreeBytes:     16000000000,
					MaxFreeOnNodeBytes: 8000000000,
				},
				GPU: GPUMetrics{
					TotalGPU: 2,
				},
			},
			Nodes: []NodeDetail{
				{
					Name: "node-1",
					CPU: NodeResource{
						CapacityCores: 4,
						UsageCores:    2,
						CapacityBytes: 8000000000,
						UsageBytes:    4000000000,
					},
					Memory: NodeResource{
						CapacityCores: 8000000000,
						UsageCores:    4000000000,
						CapacityBytes: 16000000000,
						UsageBytes:    8000000000,
					},
					GPU:         1,
					IsInterlink: true,
					Status:      "Ready",
					Conditions: []NodeConditionSimple{
						{Type: "Ready", Status: true},
						{Type: "MemoryPressure", Status: false},
					},
				},
			},
		},
		Oscar: OscarInfo{
			DeploymentName: "oscar-deployment",
			Ready:          true,
			Deployment: OscarDeployment{
				AvailableReplicas: 3,
				ReadyReplicas:     3,
				Replicas:          3,
				CreationTimestamp: now,
				Strategy:          "RollingUpdate",
				Labels: map[string]string{
					"app": "oscar",
				},
			},
			JobsCount: 5,
			Pods: PodStates{
				Total: 5,
				States: map[string]int{
					"Running":   3,
					"Succeeded": 1,
					"Failed":    1,
					"Pending":   0,
				},
			},
			OIDC: OIDCInfo{
				Enabled: true,
				Issuers: []string{"https://auth.example.com"},
				Groups:  []string{"users", "admins"},
			},
		},
		MinIO: MinioInfo{
			BucketsCount: 10,
			TotalObjects: 1500,
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(statusInfo)
	if err != nil {
		t.Fatalf("Failed to marshal StatusInfo: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled StatusInfo
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal StatusInfo: %v", err)
	}

	// Verify cluster info
	if unmarshaled.Cluster.NodesCount != 3 {
		t.Errorf("Expected NodesCount 3, got %d", unmarshaled.Cluster.NodesCount)
	}

	if unmarshaled.Cluster.Metrics.CPU.TotalFreeCores != 8 {
		t.Errorf("Expected TotalFreeCores 8, got %d", unmarshaled.Cluster.Metrics.CPU.TotalFreeCores)
	}

	if unmarshaled.Cluster.Nodes[0].Name != "node-1" {
		t.Errorf("Expected node name 'node-1', got '%s'", unmarshaled.Cluster.Nodes[0].Name)
	}

	// Verify Oscar info
	if unmarshaled.Oscar.DeploymentName != "oscar-deployment" {
		t.Errorf("Expected DeploymentName 'oscar-deployment', got '%s'", unmarshaled.Oscar.DeploymentName)
	}

	if !unmarshaled.Oscar.Ready {
		t.Error("Expected Ready to be true")
	}

	if unmarshaled.Oscar.JobsCount != 5 {
		t.Errorf("Expected JobsCount 5, got %d", unmarshaled.Oscar.JobsCount)
	}

	if unmarshaled.Oscar.Pods.Total != 5 {
		t.Errorf("Expected Total pods 5, got %d", unmarshaled.Oscar.Pods.Total)
	}

	// Verify MinIO info
	if unmarshaled.MinIO.BucketsCount != 10 {
		t.Errorf("Expected BucketsCount 10, got %d", unmarshaled.MinIO.BucketsCount)
	}

	if unmarshaled.MinIO.TotalObjects != 1500 {
		t.Errorf("Expected TotalObjects 1500, got %d", unmarshaled.MinIO.TotalObjects)
	}
}

func TestStatusInfoEmptyFields(t *testing.T) {
	statusInfo := StatusInfo{}

	data, err := json.Marshal(statusInfo)
	if err != nil {
		t.Fatalf("Failed to marshal empty StatusInfo: %v", err)
	}

	var unmarshaled StatusInfo
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal empty StatusInfo: %v", err)
	}

	// Check zero values
	if unmarshaled.Cluster.NodesCount != 0 {
		t.Errorf("Expected NodesCount 0, got %d", unmarshaled.Cluster.NodesCount)
	}

	if unmarshaled.Oscar.JobsCount != 0 {
		t.Errorf("Expected JobsCount 0, got %d", unmarshaled.Oscar.JobsCount)
	}

	if unmarshaled.MinIO.BucketsCount != 0 {
		t.Errorf("Expected BucketsCount 0, got %d", unmarshaled.MinIO.BucketsCount)
	}

	if len(unmarshaled.Cluster.Nodes) != 0 {
		t.Errorf("Expected empty Nodes slice, got %d elements", len(unmarshaled.Cluster.Nodes))
	}
}

func TestStatusInfoJSONTags(t *testing.T) {
	statusInfo := StatusInfo{
		Cluster: ClusterInfo{
			NodesCount: 1,
		},
		Oscar: OscarInfo{
			DeploymentName: "test",
			Ready:          true,
			JobsCount:      1,
		},
		MinIO: MinioInfo{
			BucketsCount: 1,
		},
	}

	data, err := json.Marshal(statusInfo)
	if err != nil {
		t.Fatalf("Failed to marshal StatusInfo: %v", err)
	}

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	if err != nil {
		t.Fatalf("Failed to unmarshal to raw map: %v", err)
	}

	// Check that JSON field names match tags
	expectedFields := []string{
		"cluster", "oscar", "minio",
	}

	for _, field := range expectedFields {
		if _, exists := raw[field]; !exists {
			t.Errorf("Expected '%s' field in JSON", field)
		}
	}

	// Check nested structures
	if cluster, exists := raw["cluster"]; exists {
		if clusterMap, ok := cluster.(map[string]interface{}); ok {
			if _, exists := clusterMap["nodes_count"]; !exists {
				t.Error("Expected 'nodes_count' field in cluster")
			}
		}
	}
}

func TestNodeDetailStructures(t *testing.T) {
	nodeDetail := NodeDetail{
		Name: "test-node",
		CPU: NodeResource{
			CapacityCores: 8,
			UsageCores:    4,
			CapacityBytes: 16000000000,
			UsageBytes:    8000000000,
		},
		Memory: NodeResource{
			CapacityCores: 16000000000,
			UsageCores:    8000000000,
			CapacityBytes: 32000000000,
			UsageBytes:    16000000000,
		},
		GPU:         2,
		IsInterlink: true,
		Status:      "Ready",
		Conditions: []NodeConditionSimple{
			{Type: "Ready", Status: true},
			{Type: "MemoryPressure", Status: false},
			{Type: "DiskPressure", Status: false},
		},
	}

	data, err := json.Marshal(nodeDetail)
	if err != nil {
		t.Fatalf("Failed to marshal NodeDetail: %v", err)
	}

	var unmarshaled NodeDetail
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal NodeDetail: %v", err)
	}

	if unmarshaled.Name != nodeDetail.Name {
		t.Errorf("Expected Name %s, got %s", nodeDetail.Name, unmarshaled.Name)
	}

	if unmarshaled.CPU.CapacityCores != nodeDetail.CPU.CapacityCores {
		t.Errorf("Expected CPU CapacityCores %d, got %d", nodeDetail.CPU.CapacityCores, unmarshaled.CPU.CapacityCores)
	}

	if unmarshaled.Memory.CapacityBytes != nodeDetail.Memory.CapacityBytes {
		t.Errorf("Expected Memory CapacityBytes %d, got %d", nodeDetail.Memory.CapacityBytes, unmarshaled.Memory.CapacityBytes)
	}

	if unmarshaled.GPU != nodeDetail.GPU {
		t.Errorf("Expected GPU %d, got %d", nodeDetail.GPU, unmarshaled.GPU)
	}

	if unmarshaled.IsInterlink != nodeDetail.IsInterlink {
		t.Errorf("Expected IsInterlink %t, got %t", nodeDetail.IsInterlink, unmarshaled.IsInterlink)
	}

	if unmarshaled.Status != nodeDetail.Status {
		t.Errorf("Expected Status %s, got %s", nodeDetail.Status, unmarshaled.Status)
	}

	if len(unmarshaled.Conditions) != len(nodeDetail.Conditions) {
		t.Errorf("Expected %d conditions, got %d", len(nodeDetail.Conditions), len(unmarshaled.Conditions))
	}
}

func TestOscarDeploymentStructures(t *testing.T) {
	now := metav1.Now()

	oscarDeployment := OscarDeployment{
		AvailableReplicas: 5,
		ReadyReplicas:     4,
		Replicas:          5,
		CreationTimestamp: now,
		Strategy:          "RollingUpdate",
		Labels: map[string]string{
			"app":     "oscar",
			"version": "v1.0.0",
		},
	}

	data, err := json.Marshal(oscarDeployment)
	if err != nil {
		t.Fatalf("Failed to marshal OscarDeployment: %v", err)
	}

	var unmarshaled OscarDeployment
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal OscarDeployment: %v", err)
	}

	if unmarshaled.AvailableReplicas != oscarDeployment.AvailableReplicas {
		t.Errorf("Expected AvailableReplicas %d, got %d", oscarDeployment.AvailableReplicas, unmarshaled.AvailableReplicas)
	}

	if unmarshaled.ReadyReplicas != oscarDeployment.ReadyReplicas {
		t.Errorf("Expected ReadyReplicas %d, got %d", oscarDeployment.ReadyReplicas, unmarshaled.ReadyReplicas)
	}

	if unmarshaled.Replicas != oscarDeployment.Replicas {
		t.Errorf("Expected Replicas %d, got %d", oscarDeployment.Replicas, unmarshaled.Replicas)
	}

	if !unmarshaled.CreationTimestamp.Time.Truncate(time.Second).Equal(now.Truncate(time.Second)) {
		t.Errorf("Expected CreationTimestamp approximately %v, got %v", now.Time, unmarshaled.CreationTimestamp.Time)
	}

	if unmarshaled.Strategy != oscarDeployment.Strategy {
		t.Errorf("Expected Strategy %s, got %s", oscarDeployment.Strategy, unmarshaled.Strategy)
	}

	if len(unmarshaled.Labels) != len(oscarDeployment.Labels) {
		t.Errorf("Expected %d labels, got %d", len(oscarDeployment.Labels), len(unmarshaled.Labels))
	}

	if unmarshaled.Labels["app"] != "oscar" {
		t.Errorf("Expected app label 'oscar', got '%s'", unmarshaled.Labels["app"])
	}
}

func TestPodStatesStructures(t *testing.T) {
	podStates := PodStates{
		Total: 10,
		States: map[string]int{
			"Running":   5,
			"Succeeded": 3,
			"Failed":    1,
			"Pending":   1,
			"Unknown":   0,
		},
	}

	data, err := json.Marshal(podStates)
	if err != nil {
		t.Fatalf("Failed to marshal PodStates: %v", err)
	}

	var unmarshaled PodStates
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal PodStates: %v", err)
	}

	if unmarshaled.Total != podStates.Total {
		t.Errorf("Expected Total %d, got %d", podStates.Total, unmarshaled.Total)
	}

	if len(unmarshaled.States) != len(podStates.States) {
		t.Errorf("Expected %d states, got %d", len(podStates.States), len(unmarshaled.States))
	}

	if unmarshaled.States["Running"] != 5 {
		t.Errorf("Expected Running 5, got %d", unmarshaled.States["Running"])
	}

	if unmarshaled.States["Succeeded"] != 3 {
		t.Errorf("Expected Succeeded 3, got %d", unmarshaled.States["Succeeded"])
	}
}

func TestOIDCInfoStructures(t *testing.T) {
	oidcInfo := OIDCInfo{
		Enabled: true,
		Issuers: []string{
			"https://auth.example.com",
			"https://auth2.example.com",
		},
		Groups: []string{
			"users",
			"admins",
			"developers",
		},
	}

	data, err := json.Marshal(oidcInfo)
	if err != nil {
		t.Fatalf("Failed to marshal OIDCInfo: %v", err)
	}

	var unmarshaled OIDCInfo
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal OIDCInfo: %v", err)
	}

	if unmarshaled.Enabled != oidcInfo.Enabled {
		t.Errorf("Expected Enabled %t, got %t", oidcInfo.Enabled, unmarshaled.Enabled)
	}

	if len(unmarshaled.Issuers) != len(oidcInfo.Issuers) {
		t.Errorf("Expected %d issuers, got %d", len(oidcInfo.Issuers), len(unmarshaled.Issuers))
	}

	if unmarshaled.Issuers[0] != "https://auth.example.com" {
		t.Errorf("Expected first issuer 'https://auth.example.com', got '%s'", unmarshaled.Issuers[0])
	}

	if len(unmarshaled.Groups) != len(oidcInfo.Groups) {
		t.Errorf("Expected %d groups, got %d", len(oidcInfo.Groups), len(unmarshaled.Groups))
	}

	if unmarshaled.Groups[0] != "users" {
		t.Errorf("Expected first group 'users', got '%s'", unmarshaled.Groups[0])
	}
}

func TestMinimalStatusInfo(t *testing.T) {
	statusInfo := StatusInfo{
		Cluster: ClusterInfo{},
		Oscar:   OscarInfo{},
		MinIO:   MinioInfo{},
	}

	data, err := json.Marshal(statusInfo)
	if err != nil {
		t.Fatalf("Failed to marshal minimal StatusInfo: %v", err)
	}

	var unmarshaled StatusInfo
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal minimal StatusInfo: %v", err)
	}

	// All nested structs should be present with zero values
	if unmarshaled.Cluster.NodesCount != 0 {
		t.Errorf("Expected NodesCount 0, got %d", unmarshaled.Cluster.NodesCount)
	}

	if unmarshaled.Oscar.JobsCount != 0 {
		t.Errorf("Expected JobsCount 0, got %d", unmarshaled.Oscar.JobsCount)
	}

	if unmarshaled.MinIO.BucketsCount != 0 {
		t.Errorf("Expected BucketsCount 0, got %d", unmarshaled.MinIO.BucketsCount)
	}
}
