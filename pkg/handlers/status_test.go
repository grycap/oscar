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
	"github.com/grycap/oscar/v3/pkg/testsupport"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
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

var (
	replicas      = int32(1)
	kubeClientset = fake.NewSimpleClientset(
		&v1.NodeList{
			Items: []v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "control-plane-node",
						Labels: map[string]string{
							"node-role.kubernetes.io/control-plane": "", // Filtered (not a worker node)
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
						Name: "worker-node",
					},
					Status: v1.NodeStatus{
						Allocatable: v1.ResourceList{
							"cpu":            *resource.NewMilliQuantity(4000, resource.DecimalSI),
							"memory":         *resource.NewQuantity(16*1024*1024*1024, resource.BinarySI),
							"nvidia.com/gpu": *resource.NewQuantity(1, resource.DecimalSI), // Has GPU
						},
						Conditions: []v1.NodeCondition{ // The node is Ready
							{Type: v1.NodeReady, Status: v1.ConditionTrue},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "interlink-node",
						Labels: map[string]string{
							"virtual-node.interlink/type": "virtual-kubelet", // InterLink Node
						},
					},
					Status: v1.NodeStatus{
						Allocatable: v1.ResourceList{
							"cpu":    *resource.NewMilliQuantity(8000, resource.DecimalSI),
							"memory": *resource.NewQuantity(32*1024*1024*1024, resource.BinarySI),
						},
						Conditions: []v1.NodeCondition{ // The node is NOT Ready
							{Type: v1.NodeReady, Status: v1.ConditionFalse},
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
						Labels:            map[string]string{"app": "oscar"},
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
						Succeeded: 1, // 1 successful job
						Active:    1, // 1 active job
						Failed:    1, // 1 failed job
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
						Phase: v1.PodSucceeded, // 1 successful pod
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "oscar-pod-2",
						Namespace:         "oscar-svc",
						CreationTimestamp: metav1.Now(),
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning, // 1 running pod
					},
				},
			},
		},
	)
	// Expected values based on the new structure
	// Allocatable:
	// worker-node: CPU=4000m, Mem=16GB, GPU=1
	// interlink-node: CPU=8000m, Mem=32GB, GPU=0
	// Usage (Metrics Usage - 50% of allocatable):
	// worker-node: CPU=2000m, Mem=8GB
	// interlink-node: CPU=4000m, Mem=16GB
	// Free:
	// worker-node: CPU=2000m, Mem=8GB
	// interlink-node: CPU=4000m, Mem=16GB
	// Totals:
	// TotalFreeCores: 2000 + 4000 = 6000m
	// MaxFreeOnNodeCores: max(2000, 4000) = 4000m
	// TotalFreeBytes: 8GB + 16GB = 24GB (24 * 1024^3)
	// MaxFreeOnNodeBytes: max(8GB, 16GB) = 16GB (16 * 1024^3)
	// TotalGPU: 1
	// JobsCount: 1 (Succeeded) + 1 (Active) + 1 (Failed) = 3
	// Pods: Total=2, States: Succeeded=1, Running=1
	expectedClusterMetrics = ClusterMetrics{
		CPU: CPUMetrics{
			TotalFreeCores:     6000,
			MaxFreeOnNodeCores: 4000,
		},
		Memory: MemoryMetrics{
			TotalFreeBytes:     24 * 1024 * 1024 * 1024,
			MaxFreeOnNodeBytes: 16 * 1024 * 1024 * 1024,
		},
		GPU: GPUMetrics{
			TotalGPU: 1,
		},
	}

	expectedWorkerNodeDetail = NodeDetail{
		Name: "worker-node",
		CPU: NodeResource{
			CapacityCores: 4000,
			UsageCores:    2000,
		},
		Memory: NodeResource{
			CapacityBytes: 16 * 1024 * 1024 * 1024,
			UsageBytes:    8 * 1024 * 1024 * 1024,
		},
		GPU:         1,
		IsInterlink: false,
		Status:      "Ready",
		Conditions: []NodeConditionSimple{
			{Type: "Ready", Status: true},
		},
	}

	expectedInterlinkNodeDetail = NodeDetail{
		Name: "interlink-node",
		CPU: NodeResource{
			CapacityCores: 8000,
			UsageCores:    4000,
		},
		Memory: NodeResource{
			CapacityBytes: 32 * 1024 * 1024 * 1024,
			UsageBytes:    16 * 1024 * 1024 * 1024,
		},
		GPU:         0,
		IsInterlink: true,
		Status:      "NotReady",
		Conditions: []NodeConditionSimple{
			{Type: "Ready", Status: false},
		},
	}
)

func checkClusterMetrics(metrics map[string]interface{}, expectedMetrics ClusterMetrics, t *testing.T) {
	cpu := metrics["cpu"].(map[string]interface{})
	mem := metrics["memory"].(map[string]interface{})
	gpu := metrics["gpu"].(map[string]interface{})

	// JSON numbers are unmarshaled as float64 by default in Go if the type is not specified.
	// That is why it is compared with float64.
	if cpu["total_free_cores"].(float64) != float64(expectedMetrics.CPU.TotalFreeCores) {
		t.Errorf("Expected total_free_cores %d, but got %v", expectedMetrics.CPU.TotalFreeCores, cpu["total_free_cores"])
	}
	if cpu["max_free_on_node_cores"].(float64) != float64(expectedMetrics.CPU.MaxFreeOnNodeCores) {
		t.Errorf("Expected max_free_on_node_cores %d, but got %v", expectedMetrics.CPU.MaxFreeOnNodeCores, cpu["max_free_on_node_cores"])
	}

	if mem["total_free_bytes"].(float64) != float64(expectedMetrics.Memory.TotalFreeBytes) {
		t.Errorf("Expected total_free_bytes %d, but got %v", expectedMetrics.Memory.TotalFreeBytes, mem["total_free_bytes"])
	}
	if mem["max_free_on_node_bytes"].(float64) != float64(expectedMetrics.Memory.MaxFreeOnNodeBytes) {
		t.Errorf("Expected max_free_on_node_bytes %d, but got %v", expectedMetrics.Memory.MaxFreeOnNodeBytes, mem["max_free_on_node_bytes"])
	}

	if gpu["total_gpu"].(float64) != float64(expectedMetrics.GPU.TotalGPU) {
		t.Errorf("Expected total_gpu %d, but got %v", expectedMetrics.GPU.TotalGPU, gpu["total_gpu"])
	}
}

func checkNodeDetail(detail map[string]interface{}, expected NodeDetail, t *testing.T) {
	if detail["name"] != expected.Name {
		t.Errorf("Node %s: Expected name %s, got %s", expected.Name, expected.Name, detail["name"])
	}
	if detail["gpu"].(float64) != float64(expected.GPU) {
		t.Errorf("Node %s: Expected gpu %d, got %v", expected.Name, expected.GPU, detail["gpu"])
	}
	if detail["is_interlink"] != expected.IsInterlink {
		t.Errorf("Node %s: Expected is_interlink %t, got %t", expected.Name, expected.IsInterlink, detail["is_interlink"])
	}
	if detail["status"] != expected.Status {
		t.Errorf("Node %s: Expected status %s, got %s", expected.Name, expected.Status, detail["status"])
	}

	cpu := detail["cpu"].(map[string]interface{})
	mem := detail["memory"].(map[string]interface{})

	if cpu["capacity_cores"].(float64) != float64(expected.CPU.CapacityCores) {
		t.Errorf("Node %s: Expected cpu capacity %d, got %v", expected.Name, expected.CPU.CapacityCores, cpu["capacity_cores"])
	}
	if cpu["usage_cores"].(float64) != float64(expected.CPU.UsageCores) {
		t.Errorf("Node %s: Expected cpu usage %d, got %v", expected.Name, expected.CPU.UsageCores, cpu["usage_cores"])
	}

	if mem["capacity_bytes"].(float64) != float64(expected.Memory.CapacityBytes) {
		t.Errorf("Node %s: Expected memory capacity %d, got %v", expected.Name, expected.Memory.CapacityBytes, mem["capacity_bytes"])
	}
	if mem["usage_bytes"].(float64) != float64(expected.Memory.UsageBytes) {
		t.Errorf("Node %s: Expected memory usage %d, got %v", expected.Name, expected.Memory.UsageBytes, mem["usage_bytes"])
	}
}

// Renamed function to avoid conflict with existing status_test.go file
func checkStatusModResult(jsonResponse map[string]interface{}, t *testing.T, isAdmin bool) {
	// Root elements
	cluster := jsonResponse["cluster"].(map[string]interface{})
	oscar := jsonResponse["oscar"].(map[string]interface{})
	minio := jsonResponse["minio"].(map[string]interface{})

	// --- CLUSTER VERIFICATIONS ---
	if cluster["nodes_count"].(float64) != 2.0 {
		t.Errorf("Expected nodes_count 2, but got %v", cluster["nodes_count"])
	}

	checkClusterMetrics(cluster["metrics"].(map[string]interface{}), expectedClusterMetrics, t)

	// Node detail verification
	details := cluster["nodes"].([]interface{})
	if len(details) != 2 {
		t.Fatalf("Expected 2 nodes in detail, but got %d", len(details))
	}

	nodeMap := make(map[string]map[string]interface{})
	for _, nodeInterface := range details {
		node := nodeInterface.(map[string]interface{})
		nodeMap[node["name"].(string)] = node
	}

	if workerNode, exists := nodeMap["worker-node"]; exists {
		checkNodeDetail(workerNode, expectedWorkerNodeDetail, t)
	} else {
		t.Error("Expected to find 'worker-node' in the response")
	}

	if interlinkNode, exists := nodeMap["interlink-node"]; exists {
		checkNodeDetail(interlinkNode, expectedInterlinkNodeDetail, t)
	} else {
		t.Error("Expected to find 'interlink-node' in the response")
	}

	// --- OSCAR VERIFICATIONS ---
	if oscar["deployment_name"] != "oscar" {
		t.Errorf("Expected deployment_name 'oscar', got %s", oscar["deployment_name"])
	}
	if oscar["ready"] != true {
		t.Errorf("Expected ready true, got %t", oscar["ready"])
	}

	deployment := oscar["deployment"].(map[string]interface{})
	if deployment["replicas"].(float64) != float64(replicas) {
		t.Errorf("Expected replicas %d, got %v", replicas, deployment["replicas"])
	}

	if isAdmin {
		// Admin/Basic Auth Verifications
		if oscar["jobs_count"].(float64) != 3.0 { // 1 Succeeded + 1 Active + 1 Failed
			t.Errorf("Expected jobs_count 3, got %v", oscar["jobs_count"])
		}

		pods := oscar["pods"].(map[string]interface{})
		if pods["total"].(float64) != 2.0 { // 1 Succeeded + 1 Running
			t.Errorf("Expected total pods 2, got %v", pods["total"])
		}
		states := pods["states"].(map[string]interface{})
		if states["Succeeded"].(float64) != 1.0 {
			t.Errorf("Expected 1 Succeeded pod, got %v", states["Succeeded"])
		}
		if states["Running"].(float64) != 1.0 {
			t.Errorf("Expected 1 Running pod, got %v", states["Running"])
		}

		// MinIO is only available for admin
		if minio["buckets_count"].(float64) != 2.0 { // Mock has 2 buckets
			t.Errorf("Expected buckets_count 2, got %v", minio["buckets_count"])
		}
		if minio["total_objects"].(float64) != 3.0 { // Mock has 3 objects
			t.Errorf("Expected total_objects 3, got %v", minio["total_objects"])
		}
	} else {
		// Regular user/Bearer Token Verifications (Jobs/Pods/MinIO must be zero/empty)
		if oscar["jobs_count"].(float64) != 0.0 {
			t.Errorf("Expected jobs_count 0 for non-admin, got %v", oscar["jobs_count"])
		}
		pods := oscar["pods"].(map[string]interface{})
		if pods["total"].(float64) != 0.0 {
			t.Errorf("Expected total pods 0 for non-admin, got %v", pods["total"])
		}
		if minio["buckets_count"].(float64) != 0.0 {
			t.Errorf("Expected buckets_count 0 for non-admin, got %v", minio["buckets_count"])
		}
	}
}

func TestMakeStatusHandler(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	// Mock HTTP Server for MinIO
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		// Mock ListBuckets (used by getMinioInfo)
		if hreq.URL.Path == "/?list-type=2" {
			rw.WriteHeader(http.StatusOK)
			// Mock of 2 buckets: "bucket-a" and "bucket-b"
			rw.Write([]byte(`
			<ListAllMyBucketsResult>
				<Buckets>
					<Bucket><Name>bucket-a</Name><CreationDate>2023-01-01T00:00:00Z</CreationDate></Bucket>
					<Bucket><Name>bucket-b</Name><CreationDate>2023-01-02T00:00:00Z</CreationDate></Bucket>
				</Buckets>
			</ListAllMyBucketsResult>`))
			return
		}

		// Mock ListObjects (used by getMinioInfo to calculate count)
		if strings.Contains(hreq.URL.RawQuery, "list-type=2") {
			rw.WriteHeader(http.StatusOK)
			// Mock of 3 total objects (2 in bucket-a, 1 in bucket-b)
			if strings.HasPrefix(hreq.URL.Path, "/bucket-a") {
				// 2 objects
				rw.Write([]byte(`
				<ListBucketResult>
					<Contents><Key>file1.txt</Key><Size>100</Size></Contents>
					<Contents><Key>file2.txt</Key><Size>200</Size></Contents>
					<Contents><Key>dir/</Key><Size>0</Size></Contents>
				</ListBucketResult>`))
				return
			} else if strings.HasPrefix(hreq.URL.Path, "/bucket-b") {
				// 1 object
				rw.Write([]byte(`
				<ListBucketResult>
					<Contents><Key>file3.dat</Key><Size>300</Size></Contents>
				</ListBucketResult>`))
				return
			}
		}

		// Mock Admin info (used by the admin client)
		if strings.HasPrefix(hreq.URL.Path, "/minio/admin/") {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"Mode": "local", "Region": "us-east-1"}`))
			return
		}

		t.Errorf("Unexpected path or query in MinIO request: %s", hreq.URL.Path+"?"+hreq.URL.RawQuery)
		rw.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Create a fake Metrics clientset
	metricsClientset := metricsfake.NewSimpleClientset()
	// Add NodeMetrics objects to the fake clientset store
	metricsClientset.Fake.PrependReactor("list", "nodemetrics", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		// Usage values must be 50% of the Allocatable defined above
		return true, &metricsv1beta1api.NodeMetricsList{
			Items: []metricsv1beta1api.NodeMetrics{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "worker-node"},
					Usage: v1.ResourceList{
						"cpu":    *resource.NewMilliQuantity(2000, resource.DecimalSI),       // 50% usage of 4000m
						"memory": *resource.NewQuantity(8*1024*1024*1024, resource.BinarySI), // 50% usage of 16GB
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "interlink-node"},
					Usage: v1.ResourceList{
						"cpu":    *resource.NewMilliQuantity(4000, resource.DecimalSI),        // 50% usage of 8000m
						"memory": *resource.NewQuantity(16*1024*1024*1024, resource.BinarySI), // 50% usage of 32GB
					},
				},
			},
		}, nil
	})

	cfg := types.Config{
		ServicesNamespace: "oscar-svc",
		Namespace:         "oscar",
		Username:          "testuser",
		Password:          "testpass",
		UsersAdmin:        []string{"adminuser@egi.eu"},
		OIDCEnable:        true,
		OIDCValidIssuers:  []string{"issuer1", "issuer2"},
		OIDCGroups:        []string{"group1", "group2"},
		MinIOProvider: &types.MinIOProvider{
			Region:    "us-east-1",
			Endpoint:  server.URL,
			AccessKey: "ak",
			SecretKey: "sk",
		},
	}

	// Create a new Gin router
	router := gin.Default()
	router.Use(func(c *gin.Context) {
		// Simulate context values set by another middleware for a regular user
		c.Set("uidOrigin", "somelonguid@egi.eu")
		c.Set("multitenancyConfig", auth.NewMultitenancyConfig(kubeClientset, "regularuser@egi.eu"))
		c.Next()
	})
	router.GET("/status", MakeStatusHandler(&cfg, kubeClientset, metricsClientset.MetricsV1beta1()))

	// --- 1. NON-ADMIN Test (Bearer Token) ---
	req, _ := http.NewRequest("GET", "/status", nil)
	// Non-admin/user token, so isAdmin will be FALSE
	req.Header.Set("Authorization", "Bearer 11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Non-admin request failed. Expected status code %d, but got %d. Response: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var jsonResponse map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &jsonResponse)
	if err != nil {
		t.Fatalf("Failed to decode non-admin response: %v", err)
	}

	checkStatusModResult(jsonResponse, t, false)

	// --- 2. ADMIN Test (Basic Auth) ---

	// Reconfigure the context to simulate an admin user
	router = gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "adminuser@egi.eu") // This user is in cfg.UsersAdmin
		c.Set("multitenancyConfig", auth.NewMultitenancyConfig(kubeClientset, "adminuser@egi.eu"))
		c.Next()
	})
	router.GET("/status", MakeStatusHandler(&cfg, kubeClientset, metricsClientset.MetricsV1beta1()))

	req2, _ := http.NewRequest("GET", "/status", nil)
	req2.Header.Set("Authorization", "Basic dGVzdHVzZXI6dGVzdHBhc3M=") // Ignored, but Basic Auth usually implies Admin

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Admin request failed. Expected status code %d, but got %d. Response: %s", http.StatusOK, w2.Code, w2.Body.String())
	}

	var jsonResponse2 map[string]interface{}
	err2 := json.Unmarshal(w2.Body.Bytes(), &jsonResponse2)
	if err2 != nil {
		t.Fatalf("Failed to decode admin response: %v", err2)
	}

	checkStatusModResult(jsonResponse2, t, true)

}
