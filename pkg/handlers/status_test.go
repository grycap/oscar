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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func TestMakeStatusHandler(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		if hreq.URL.Path != "/" && hreq.URL.Path != "/output" && !strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/") {
			t.Errorf("Unexpected path in request, got: %s", hreq.URL.Path)
		}
		if hreq.URL.Path == "/minio/admin/v3/info" {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"Mode": "local", "Region": "us-east-1"}`))
		} else {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"status": "success"}`))
		}
	}))
	// Create a fake Kubernetes clientset
	replicas := int32(1)
	kubeClientset := fake.NewSimpleClientset(
		&v1.NodeList{
			Items: []v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "control-plane-node",
						Labels: map[string]string{
							"node-role.kubernetes.io/control-plane": "", // This will be filtered out
						},
					},
					Status: v1.NodeStatus{
						Allocatable: v1.ResourceList{
							"cpu":    *resource.NewMilliQuantity(2000, resource.DecimalSI),
							"memory": *resource.NewQuantity(8*1024*1024*1024, resource.BinarySI),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "worker-node",
						Labels: map[string]string{}, // No control-plane label - will be included
					},
					Status: v1.NodeStatus{
						Allocatable: v1.ResourceList{
							"cpu":            *resource.NewMilliQuantity(4000, resource.DecimalSI),
							"memory":         *resource.NewQuantity(16*1024*1024*1024, resource.BinarySI),
							"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI), // Has GPU
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "interlink-node",
						Labels: map[string]string{
							"virtual-node.interlink/type": "virtual-kubelet", // InterLink node
						},
					},
					Status: v1.NodeStatus{
						Allocatable: v1.ResourceList{
							"cpu":    *resource.NewMilliQuantity(8000, resource.DecimalSI),
							"memory": *resource.NewQuantity(32*1024*1024*1024, resource.BinarySI),
						},
					},
				},
			},
		},
		&apps.DeploymentList{
			Items: []apps.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "oscar",
						Namespace:         "oscar",
						CreationTimestamp: metav1.Now(),
					},
					Status: apps.DeploymentStatus{
						Replicas:          1,
						AvailableReplicas: 1,
						ReadyReplicas:     1,
					},
					Spec: apps.DeploymentSpec{
						Strategy: apps.DeploymentStrategy{
							Type: apps.RollingUpdateDeploymentStrategyType,
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "oscar"},
						},
						Replicas: &replicas,
					},
				},
			},
		},
		&batch.JobList{
			Items: []batch.Job{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "oscar-job-1",
						Namespace:         "oscar-svc",
						CreationTimestamp: metav1.Now(),
					},
					Status: batch.JobStatus{
						Succeeded: 1,
						Active:    0,
						Failed:    0,
					},
				},
			},
		},
		&v1.PodList{
			Items: []v1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "oscar-pod-1",
						Namespace:         "oscar-svc",
						CreationTimestamp: metav1.Now(),
					},
					Status: v1.PodStatus{
						Phase: v1.PodSucceeded,
					},
				},
			},
		},
	)

	// Create a fake Metrics clientset
	metricsClientset := metricsfake.NewSimpleClientset()
	// Add NodeMetrics objects to the fake clientset's store
	metricsClientset.Fake.PrependReactor("list", "nodes", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &metricsv1beta1api.NodeMetricsList{
			Items: []metricsv1beta1api.NodeMetrics{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "worker-node"},
					Usage: v1.ResourceList{
						"cpu":    *resource.NewMilliQuantity(2000, resource.DecimalSI),       // 50% usage
						"memory": *resource.NewQuantity(8*1024*1024*1024, resource.BinarySI), // 50% usage
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "interlink-node"},
					Usage: v1.ResourceList{
						"cpu":    *resource.NewMilliQuantity(4000, resource.DecimalSI),        // 50% usage
						"memory": *resource.NewQuantity(16*1024*1024*1024, resource.BinarySI), // 50% usage
					},
				},
			},
		}, nil
	})
	cfg := types.Config{
		ServicesNamespace: "oscar-svc",
		Namespace:         "oscar",
		MinIOProvider: &types.MinIOProvider{
			Region:    "us-east-1",
			Endpoint:  server.URL,
			AccessKey: "ak",
			SecretKey: "sk",
		},
	}

	// Create a new Gin router
	router := gin.Default()
	router.GET("/status", MakeStatusHandler(&cfg, kubeClientset, metricsClientset.MetricsV1beta1()))

	// Create a new HTTP request
	req, _ := http.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()

	// Perform the request
	router.ServeHTTP(w, req)

	// Check the response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, but got %d", http.StatusOK, w.Code)
	}

	var jsonResponse map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &jsonResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Calculate expected values:
	// worker-node: 4000 - 2000 = 2000 CPU free, 16GB - 8GB = 8GB memory free
	// interlink-node: 8000 - 4000 = 4000 CPU free, 32GB - 16GB = 16GB memory free
	// Totals: 6000 CPU free, 24GB memory free
	// Max: 4000 CPU free, 16GB memory free
	expectedResponse := map[string]interface{}{
		"numberNodes":     2.0,                       // worker-node + interlink-node (control-plane filtered out)
		"cpuFreeTotal":    6000.0,                    // 2000 + 4000
		"cpuMaxFree":      4000.0,                    // max(2000, 4000)
		"memoryFreeTotal": 24.0 * 1024 * 1024 * 1024, // 8GB + 16GB
		"memoryMaxFree":   16.0 * 1024 * 1024 * 1024, // max(8GB, 16GB)
		"detail": []interface{}{
			map[string]interface{}{
				"nodeName":         "worker-node",
				"cpuCapacity":      "4000",
				"cpuUsage":         "2000",
				"cpuPercentage":    "50.00",
				"memoryCapacity":   "17179869184", // 16GB in bytes
				"memoryUsage":      "8589934592",  // 8GB in bytes
				"memoryPercentage": "50.00",
				"isInterLink":      false,
				"hasGPU":           true, // Has nvidia.com/gpu
			},
			map[string]interface{}{
				"nodeName":         "interlink-node",
				"cpuCapacity":      "8000",
				"cpuUsage":         "4000",
				"cpuPercentage":    "50.00",
				"memoryCapacity":   "34359738368", // 32GB in bytes
				"memoryUsage":      "17179869184", // 16GB in bytes
				"memoryPercentage": "50.00",
				"isInterLink":      true, // Has virtual-node.interlink/type=virtual-kubelet
				"hasGPU":           false,
			},
		},
	}

	// Since the order of nodes in the detail array can vary (map iteration),
	// we should check each field separately rather than using DeepEqual
	if jsonResponse["numberNodes"] != expectedResponse["numberNodes"] {
		t.Errorf("Expected numberNodes %v, but got %v", expectedResponse["numberNodes"], jsonResponse["numberNodes"])
	}

	if jsonResponse["cpuFreeTotal"] != expectedResponse["cpuFreeTotal"] {
		t.Errorf("Expected cpuFreeTotal %v, but got %v", expectedResponse["cpuFreeTotal"], jsonResponse["cpuFreeTotal"])
	}

	if jsonResponse["cpuMaxFree"] != expectedResponse["cpuMaxFree"] {
		t.Errorf("Expected cpuMaxFree %v, but got %v", expectedResponse["cpuMaxFree"], jsonResponse["cpuMaxFree"])
	}

	if jsonResponse["memoryFreeTotal"] != expectedResponse["memoryFreeTotal"] {
		t.Errorf("Expected memoryFreeTotal %v, but got %v", expectedResponse["memoryFreeTotal"], jsonResponse["memoryFreeTotal"])
	}

	if jsonResponse["memoryMaxFree"] != expectedResponse["memoryMaxFree"] {
		t.Errorf("Expected memoryMaxFree %v, but got %v", expectedResponse["memoryMaxFree"], jsonResponse["memoryMaxFree"])
	}

	// Check that we have 2 nodes in detail
	detail, ok := jsonResponse["detail"].([]interface{})
	if !ok || len(detail) != 2 {
		t.Errorf("Expected 2 nodes in detail, but got %d", len(detail))
	}

	// Verify each node exists with correct properties
	nodeMap := make(map[string]map[string]interface{})
	for _, nodeInterface := range detail {
		node := nodeInterface.(map[string]interface{})
		nodeName := node["nodeName"].(string)
		nodeMap[nodeName] = node
	}

	// Check worker-node
	if workerNode, exists := nodeMap["worker-node"]; exists {
		if workerNode["hasGPU"] != true {
			t.Error("Expected worker-node to have GPU")
		}
		if workerNode["isInterLink"] != false {
			t.Error("Expected worker-node to NOT be InterLink")
		}
	} else {
		t.Error("Expected to find worker-node in response")
	}

	// Check interlink-node
	if interlinkNode, exists := nodeMap["interlink-node"]; exists {
		if interlinkNode["hasGPU"] != false {
			t.Error("Expected interlink-node to NOT have GPU")
		}
		if interlinkNode["isInterLink"] != true {
			t.Error("Expected interlink-node to be InterLink")
		}
	} else {
		t.Error("Expected to find interlink-node in response")
	}
}
