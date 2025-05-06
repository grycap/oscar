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
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
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
	// Create a fake Kubernetes clientset
	kubeClientset := fake.NewSimpleClientset(
		&v1.NodeList{
			Items: []v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Status: v1.NodeStatus{
						Allocatable: v1.ResourceList{
							"cpu":    *resource.NewMilliQuantity(2000, resource.DecimalSI),
							"memory": *resource.NewQuantity(8*1024*1024*1024, resource.BinarySI),
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node2"},
					Status: v1.NodeStatus{
						Allocatable: v1.ResourceList{
							"cpu":    *resource.NewMilliQuantity(4000, resource.DecimalSI),
							"memory": *resource.NewQuantity(16*1024*1024*1024, resource.BinarySI),
						},
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
					ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					Usage: v1.ResourceList{
						"cpu":    *resource.NewMilliQuantity(1000, resource.DecimalSI),
						"memory": *resource.NewQuantity(4*1024*1024*1024, resource.BinarySI),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "node2"},
					Usage: v1.ResourceList{
						"cpu":    *resource.NewMilliQuantity(2000, resource.DecimalSI),
						"memory": *resource.NewQuantity(8*1024*1024*1024, resource.BinarySI),
					},
				},
			},
		}, nil
	})

	// Create a new Gin router
	router := gin.Default()
	router.GET("/status", MakeStatusHandler(kubeClientset, metricsClientset.MetricsV1beta1()))

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

	expectedResponse := map[string]interface{}{
		"numberNodes":     1.0,
		"cpuFreeTotal":    2000.0,
		"cpuMaxFree":      2000.0,
		"memoryFreeTotal": 16.0 * 1024 * 1024 * 1024,
		"memoryMaxFree":   8.0 * 1024 * 1024 * 1024,
		"detail": []interface{}{
			map[string]interface{}{
				"nodeName":         "node2",
				"cpuCapacity":      "4000",
				"cpuUsage":         "2000",
				"cpuPercentage":    "50.00",
				"memoryCapacity":   "17179869184",
				"memoryUsage":      "8589934592",
				"memoryPercentage": "50.00",
			},
		},
	}

	if !reflect.DeepEqual(jsonResponse, expectedResponse) {
		t.Errorf("Expected response %v, but got %v", expectedResponse, jsonResponse)
	}
}
