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
}

// MakeStatusHandler Status handler for kubernetes deployment.
func MakeStatusHandler(kubeClientset *kubernetes.Clientset, metricsClientset *versioned.MetricsV1beta1Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get  nodes list
		nodes, err := kubeClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting nodes list: %v\n", err)
			os.Exit(1)
		}

		// Get metrics nodes.
		nodeMetricsList, err := metricsClientset.NodeMetricses().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting metrics nodes: %v\n", err)
			os.Exit(1)
		}

		var nodeInfoList []NodeInfo
		var clusterInfo GeneralInfo

		var cpu_free_total int64 = 0
		var cpu_max_free int64 = 0
		var memory_free_total int64 = 0
		var memory_max_free int64 = 0
		var number_nodes int64 = 0

		// Parameters CPU and Memory.
		for id, node := range nodes.Items {
			//remove fronted node from json list
			if id > 0 {
				nodeName := node.Name
				cpu_alloc := node.Status.Allocatable.Cpu().MilliValue()
				cpu_usage := nodeMetricsList.Items[id].Usage["cpu"]
				cpu_usage_percent := (float64(cpu_usage.MilliValue()) / float64(cpu_alloc)) * 100

				memory_alloc := node.Status.Allocatable.Memory().Value()
				memory_usage := nodeMetricsList.Items[id].Usage["memory"]
				memory_usage_percent := (float64(memory_usage.Value()) / float64(memory_alloc)) * 100

				nodeInfo := NodeInfo{
					NodeName:         nodeName,
					CPUCapacity:      strconv.Itoa(int(cpu_alloc)),
					CPUUsage:         strconv.Itoa(int(cpu_usage.MilliValue())),
					CPUPercentage:    fmt.Sprintf("%.2f", cpu_usage_percent),
					MemoryCapacity:   strconv.Itoa(int(memory_alloc)),
					MemoryUsage:      strconv.Itoa(int(memory_usage.Value())),
					MemoryPercentage: fmt.Sprintf("%.2f", memory_usage_percent),
				}
				number_nodes++
				cpu_node_free := cpu_alloc - cpu_usage.MilliValue()
				cpu_free_total += cpu_node_free

				if cpu_max_free < cpu_node_free {
					cpu_max_free = cpu_node_free
				}

				memory_node_free := memory_alloc - memory_usage.Value()
				memory_free_total += memory_alloc

				if memory_max_free < memory_node_free {
					memory_max_free = memory_node_free
				}

				nodeInfoList = append(nodeInfoList, nodeInfo)
			}
			// Create cluster status structure
			clusterInfo = GeneralInfo{
				NumberNodes:     number_nodes,
				CPUFreeTotal:    cpu_free_total,
				CPUMaxFree:      cpu_max_free,
				MemoryFreeTotal: memory_free_total,
				MemoryMaxFree:   memory_max_free,
				DetailsNodes:    nodeInfoList,
			}
		}
		// Encode list of NodeInfo structures in json format.

		c.JSON(http.StatusOK, clusterInfo)

	}
}
