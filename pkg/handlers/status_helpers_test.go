package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
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

func makeFakeClients() (*fake.Clientset, *metricsfake.Clientset) {
	// Worker node includes GPU resources so we can verify aggregation logic.
	workerNode := v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-node",
		},
		Status: v1.NodeStatus{
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:              *resource.NewMilliQuantity(4000, resource.DecimalSI),
				v1.ResourceMemory:           *resource.NewQuantity(16*1024*1024*1024, resource.BinarySI),
				"nvidia.com/gpu":            *resource.NewQuantity(2, resource.DecimalSI),
				v1.ResourceEphemeralStorage: *resource.NewQuantity(100, resource.BinarySI),
			},
			Conditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionTrue,
				},
			},
		},
	}

	interlinkNode := v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "interlink-node",
			Labels: map[string]string{
				"virtual-node.interlink/type": "virtual-kubelet",
			},
		},
		Status: v1.NodeStatus{
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(2000, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(8*1024*1024*1024, resource.BinarySI),
			},
		},
	}

	controlPlaneNode := v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "control-plane",
			Labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "",
			},
		},
		Status: v1.NodeStatus{
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(2*1024*1024*1024, resource.BinarySI),
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(
		&v1.NodeList{Items: []v1.Node{workerNode, interlinkNode, controlPlaneNode}},
		&apps.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "oscar",
				Namespace: "oscar",
				Labels:    map[string]string{"app": "oscar"},
			},
			Spec: apps.DeploymentSpec{
				Replicas: int32Ptr(2),
				Strategy: apps.DeploymentStrategy{
					Type: apps.RollingUpdateDeploymentStrategyType,
				},
			},
			Status: apps.DeploymentStatus{
				ReadyReplicas:     2,
				AvailableReplicas: 2,
			},
		},
		&batch.JobList{
			Items: []batch.Job{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "job-success",
						Namespace: "oscar-svc",
					},
					Status: batch.JobStatus{
						Succeeded: 1,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "job-failed",
						Namespace: "oscar-svc",
					},
					Status: batch.JobStatus{
						Failed: 1,
					},
				},
			},
		},
		&v1.PodList{
			Items: []v1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-success",
						Namespace: "oscar-svc",
					},
					Status: v1.PodStatus{Phase: v1.PodSucceeded},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-running",
						Namespace: "oscar-svc",
					},
					Status: v1.PodStatus{Phase: v1.PodRunning},
				},
			},
		},
	)

	metricsClient := metricsfake.NewSimpleClientset()
	metricsClient.Fake.PrependReactor("list", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &metricsv1beta1api.NodeMetricsList{
			Items: []metricsv1beta1api.NodeMetrics{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "worker-node"},
					Usage: v1.ResourceList{
						v1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
						v1.ResourceMemory: *resource.NewQuantity(4*1024*1024*1024, resource.BinarySI),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "interlink-node"},
					Usage: v1.ResourceList{
						v1.ResourceCPU:    *resource.NewMilliQuantity(500, resource.DecimalSI),
						v1.ResourceMemory: *resource.NewQuantity(2*1024*1024*1024, resource.BinarySI),
					},
				},
			},
		}, nil
	})
	return fakeClient, metricsClient
}

func int32Ptr(v int32) *int32 {
	return &v
}

func TestGetNodesInfoAggregatesClusterData(t *testing.T) {
	fakeClient, _ := makeFakeClients()
	statusInfo := &NewStatusInfo{}

	nodeInfo, err := getNodesInfo(fakeClient, statusInfo)
	if err != nil {
		t.Fatalf("getNodesInfo returned unexpected error: %v", err)
	}

	if len(nodeInfo) != 2 {
		t.Fatalf("expected 2 worker nodes, got %d", len(nodeInfo))
	}

	if statusInfo.Cluster.Metrics.GPU.TotalGPU != 2 {
		t.Fatalf("expected total GPUs to be 2, got %d", statusInfo.Cluster.Metrics.GPU.TotalGPU)
	}

	worker := nodeInfo["worker-node"]
	if worker == nil {
		t.Fatalf("missing worker-node entry in nodeInfo")
	}
	if worker.NodeDetail.GPU != 2.0 {
		t.Fatalf("expected worker-node HasGPU true, got %v", worker.NodeDetail.GPU)
	}
	if worker.NodeDetail.IsInterlink {
		t.Fatalf("worker-node should not be marked as interlink")
	}

	interlink := nodeInfo["interlink-node"]
	if interlink == nil {
		t.Fatalf("missing interlink-node entry in nodeInfo")
	}
	if !interlink.NodeDetail.IsInterlink {
		t.Fatalf("expected interlink-node IsInterLink true")
	}
	if worker.NodeDetail.GPU == 0.0 {
		t.Fatalf("interlink-node should not expose GPU resources")
	}
}

func TestGetMetricsInfoUpdatesUsage(t *testing.T) {
	fakeClient, metricsClient := makeFakeClients()
	statusInfo := &NewStatusInfo{}

	nodeInfo, err := getNodesInfo(fakeClient, statusInfo)
	if err != nil {
		t.Fatalf("getNodesInfo returned unexpected error: %v", err)
	}

	if err := getMetricsInfo(fakeClient, metricsClient.MetricsV1beta1(), nodeInfo, statusInfo); err != nil {
		t.Fatalf("getMetricsInfo returned unexpected error: %v", err)
	}

	if statusInfo.Cluster.NodesCount != 2 {
		t.Fatalf("expected NumberNodes 2, got %d", statusInfo.Cluster.NodesCount)
	}

	if statusInfo.Cluster.Metrics.CPU.TotalFreeCores != (4000-1000)+(2000-500) {
		t.Fatalf("unexpected CPUFreeTotal: %d", statusInfo.Cluster.Metrics.CPU.TotalFreeCores)
	}

	worker := nodeInfo["worker-node"].NodeDetail
	if worker.CPU.UsageCores != 1000 || worker.Memory.UsageBytes != 4294967296 {
		t.Fatalf("unexpected usage data for worker-node: %+v", worker)
	}

}

func TestGetDeploymentInfoPopulatesOscarSection(t *testing.T) {
	fakeClient, _ := makeFakeClients()
	cfg := &types.Config{
		Namespace:        "oscar",
		OIDCEnable:       true,
		OIDCValidIssuers: []string{"issuer"},
		OIDCGroups:       []string{"group"},
	}

	statusInfo := &NewStatusInfo{}

	if err := getDeploymentInfo(fakeClient, cfg, statusInfo); err != nil {
		t.Fatalf("getDeploymentInfo returned unexpected error: %v", err)
	}

	if statusInfo.Oscar.DeploymentName != "oscar" {
		t.Fatalf("expected deployment name 'oscar', got %s", statusInfo.Oscar.DeploymentName)
	}
	if !statusInfo.Oscar.Ready {
		t.Fatalf("expected DeploymentReady true")
	}
	if !statusInfo.Oscar.OIDC.Enabled {
		t.Fatalf("expected OIDC.Enabled to mirror config")
	}
}

func TestGetJobsInfoSummarisesStatus(t *testing.T) {
	fakeClient, _ := makeFakeClients()
	statusInfo := &NewStatusInfo{
		Oscar: OscarInfo{},
	}
	cfg := &types.Config{
		ServicesNamespace: "oscar-svc",
	}

	if err := getJobsInfo(cfg, fakeClient, statusInfo); err != nil {
		t.Fatalf("getJobsInfo returned unexpected error: %v", err)
	}

	if statusInfo.Oscar.Pods.States["Succeeded"] != 1 || statusInfo.Oscar.Pods.States["Running"] != 1 {
		t.Fatalf("unexpected jobs count: %+v", statusInfo.Oscar.Pods)
	}
	if statusInfo.Oscar.Pods.Total != 2 {
		t.Fatalf("expected 2 pods in PodsInfo.Total, got %d", statusInfo.Oscar.Pods.Total)
	}
	if statusInfo.Oscar.Pods.States["Running"] != 1 {
		t.Fatalf("expected 1 running pod, got %+v", statusInfo.Oscar.Pods.States["Running"])
	}
}

func TestMakeStatusHandlerHandlesNodeListErrors(t *testing.T) {
	fakeClient, metricsClient := makeFakeClients()
	fakeClient.Fake.PrependReactor("list", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("cannot list nodes")
	})

	cfg := &types.Config{
		Namespace:         "oscar",
		ServicesNamespace: "oscar-svc",
		UsersAdmin:        []string{"admin@example.org"},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "user@example.org")
		c.Next()
	})
	router.GET("/status", MakeStatusHandler(cfg, fakeClient, metricsClient.MetricsV1beta1()))

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", resp.Code)
	}
}

func TestMakeStatusHandlerReturnsClusterInfoForNonAdmin(t *testing.T) {
	fakeClient, metricsClient := makeFakeClients()
	cfg := &types.Config{
		Namespace:         "oscar",
		ServicesNamespace: "oscar-svc",
		UsersAdmin:        []string{"admin@example.org"},
		MinIOProvider:     &types.MinIOProvider{},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "user@example.org")
		c.Set("multitenancyConfig", auth.NewMultitenancyConfig(fakeClient, "user@example.org"))
		c.Next()
	})

	router.GET("/status", MakeStatusHandler(cfg, fakeClient, metricsClient.MetricsV1beta1()))

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}

	var payload NewStatusInfo
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Cluster.NodesCount != 2 {
		t.Fatalf("expected NumberNodes 2, got %d", payload.Cluster.NodesCount)
	}
	if payload.Oscar.Deployment.AvailableReplicas == 0 {
		t.Fatal("expected OSCAR.DeploymentInfo to be populated")
	}
	// Non admin users should not receive MinIO information.
	if payload.MinIO.BucketsCount != 0 {
		t.Fatalf("expected no MinIO buckets for non admin user: %+v", payload.MinIO.BucketsCount)
	}
}
