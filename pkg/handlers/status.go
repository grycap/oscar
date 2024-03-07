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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	versioned "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

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

		// Get metrics nodes
		nodeMetricsList, err := metricsClientset.NodeMetricses().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting metrics nodes: %v\n", err)
			os.Exit(1)
		}

		var nodeInfoList []NodeInfo
		// GET Parameters CPU and Memory.
		for id, node := range nodes.Items {
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

			nodeInfoList = append(nodeInfoList, nodeInfo)
		}
		// Encode list of NodeInfo structures in json format.
		jsonData, err := json.MarshalIndent(nodeInfoList, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding json: %v\n", err)
			os.Exit(1)
		}
		c.JSON(http.StatusOK, jsonData)
		//c.String(http.StatusOK, "Ok")
	}
}
