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

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	versioned "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

type GeneralInfo struct {
	NumberNodes     int64      `json:"numberNodes"`
	CPUFreeTotal    int64      `json:"cpuFreeTotal"`
	CPUMaxFree      int64      `json:"cpuMaxFree"`
	MemoryFreeTotal int64      `json:"memoryFreeTotal"`
	MemoryMaxFree   int64      `json:"memoryMaxFree"`
	DetailsNodes    []NodeInfo `json:"detail"`
}

type NodeInfo struct {
	NodeName         string `json:"nodeName"`
	CPUCapacity      string `json:"cpuCapacity"`
	CPUUsage         string `json:"cpuUsage"`
	CPUPercentage    string `json:"cpuPercentage"`
	MemoryCapacity   string `json:"memoryCapacity"`
	MemoryUsage      string `json:"memoryUsage"`
	MemoryPercentage string `json:"memoryPercentage"`
	IsInterLink      bool   `json:"isInterLink"`
	HasGPU           bool   `json:"hasGPU"`
}

// Enhanced struct to store both display strings and int64 values
type NodeInfoWithAllocatable struct {
	NodeInfo          NodeInfo
	CPUAllocatable    int64
	MemoryAllocatable int64
}

// MakeStatusHandler Status handler for kubernetes deployment.
func MakeStatusHandler(kubeClientset kubernetes.Interface, metricsClientset versioned.MetricsV1beta1Interface) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get nodes list
		nodes, err := kubeClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get nodes list"})
			return
		}

		// Get metrics nodes.
		nodeMetricsList, err := metricsClientset.NodeMetricses().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting metrics nodes: %v\n", err)
			os.Exit(1)
		}

		// Use a map to store nodeInfo with int64 allocatable values
		nodeInfoMap := make(map[string]*NodeInfoWithAllocatable)
		var cpu_free_total int64 = 0
		var cpu_max_free int64 = 0
		var memory_free_total int64 = 0
		var memory_max_free int64 = 0

		// First pass: Create nodeInfo entries for all nodes (except control plane)
		for _, node := range nodes.Items {
			// Skip control plane nodes by checking their roles
			if isControlPlaneNode(node) {
				continue
			}

			nodeName := node.Name
			cpu_alloc := node.Status.Allocatable.Cpu().MilliValue()
			memory_alloc := node.Status.Allocatable.Memory().Value()

			// Check if node is interLink (look for specific labels or annotations)
			isInterLink := checkIfInterLinkNode(node)

			// Check if node has GPU (look for nvidia.com/gpu or amd.com/gpu resources)
			hasGPU := checkIfNodeHasGPU(node)

			nodeInfoMap[nodeName] = &NodeInfoWithAllocatable{
				NodeInfo: NodeInfo{
					NodeName:         nodeName,
					CPUCapacity:      strconv.Itoa(int(cpu_alloc)),
					CPUUsage:         "0", // Default to 0
					CPUPercentage:    "0.00",
					MemoryCapacity:   strconv.Itoa(int(memory_alloc)),
					MemoryUsage:      "0", // Default to 0
					MemoryPercentage: "0.00",
					IsInterLink:      isInterLink,
					HasGPU:           hasGPU,
				},
				CPUAllocatable:    cpu_alloc,
				MemoryAllocatable: memory_alloc,
			}
		}

		// Second pass: Going through nodeMetricsList, populating the rest of fields
		for _, metrics := range nodeMetricsList.Items {
			nodeName := metrics.Name
			if nodeInfo, exists := nodeInfoMap[nodeName]; exists {
				// Use the stored int64 values directly (no string parsing!)
				cpu_alloc := nodeInfo.CPUAllocatable
				memory_alloc := nodeInfo.MemoryAllocatable

				cpu_usage := metrics.Usage["cpu"]
				memory_usage := metrics.Usage["memory"]
				cpu_usage_percent := (float64(cpu_usage.MilliValue()) / float64(cpu_alloc)) * 100
				memory_usage_percent := (float64(memory_usage.Value()) / float64(memory_alloc)) * 100

				// Update the nodeInfo fields directly by name
				nodeInfo.NodeInfo.CPUUsage = strconv.Itoa(int(cpu_usage.MilliValue()))
				nodeInfo.NodeInfo.CPUPercentage = fmt.Sprintf("%.2f", cpu_usage_percent)
				nodeInfo.NodeInfo.MemoryUsage = strconv.Itoa(int(memory_usage.Value()))
				nodeInfo.NodeInfo.MemoryPercentage = fmt.Sprintf("%.2f", memory_usage_percent)

				// Calculate free resources for cluster totals
				cpu_node_free := cpu_alloc - cpu_usage.MilliValue()
				cpu_free_total += cpu_node_free

				if cpu_max_free < cpu_node_free {
					cpu_max_free = cpu_node_free
				}

				memory_node_free := memory_alloc - memory_usage.Value()
				memory_free_total += memory_node_free

				if memory_max_free < memory_node_free {
					memory_max_free = memory_node_free
				}
			}
		}

		// Convert map to slice for JSON response
		var nodeInfoList []NodeInfo
		for _, nodeInfo := range nodeInfoMap {
			nodeInfoList = append(nodeInfoList, nodeInfo.NodeInfo)
		}

		// Create cluster status structure (only once, outside loops)
		clusterInfo := GeneralInfo{
			NumberNodes:     int64(len(nodeInfoMap)), // Use map size instead
			CPUFreeTotal:    cpu_free_total,
			CPUMaxFree:      cpu_max_free,
			MemoryFreeTotal: memory_free_total,
			MemoryMaxFree:   memory_max_free,
			DetailsNodes:    nodeInfoList,
		}

		// Encode list of NodeInfo structures in json format.
		c.JSON(http.StatusOK, clusterInfo)

	}
}

// Helper function to check if a node is an interLink node
func checkIfInterLinkNode(node v1.Node) bool {
	// Check for the specific interLink label
	if nodeType, exists := node.Labels["virtual-node.interlink/type"]; exists && nodeType == "virtual-kubelet" {
		return true
	}
	return false
}

// Helper function to check if a node has GPU
func checkIfNodeHasGPU(node v1.Node) bool {
	// Check for NVIDIA GPU resources in allocatable
	if gpu, exists := node.Status.Allocatable["nvidia.com/gpu"]; exists && !gpu.IsZero() {
		return true
	}
	return false
}

// Helper function to check if a node is a control plane node
func isControlPlaneNode(node v1.Node) bool {
	// Check for control-plane role label (Kubernetes 1.20+)
	if _, exists := node.Labels["node-role.kubernetes.io/control-plane"]; exists {
		return true
	}

	// Check for master role label (older Kubernetes versions)
	if _, exists := node.Labels["node-role.kubernetes.io/master"]; exists {
		return true
	}

	return false
}
