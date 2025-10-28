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
				v1.ResourceCPU:       *resource.NewMilliQuantity(4000, resource.DecimalSI),
				v1.ResourceMemory:    *resource.NewQuantity(16*1024*1024*1024, resource.BinarySI),
				"nvidia.com/gpu":     *resource.NewQuantity(2, resource.DecimalSI),
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
	clusterInfo := &GeneralInfo{}

	nodeInfo, err := getNodesInfo(fakeClient, clusterInfo)
	if err != nil {
		t.Fatalf("getNodesInfo returned unexpected error: %v", err)
	}

	if len(nodeInfo) != 2 {
		t.Fatalf("expected 2 worker nodes, got %d", len(nodeInfo))
	}

	if !clusterInfo.HasGPU {
		t.Fatal("expected clusterInfo.HasGPU to be true when GPU resources exist")
	}
	if clusterInfo.GPUsTotal != 2 {
		t.Fatalf("expected total GPUs to be 2, got %d", clusterInfo.GPUsTotal)
	}

	worker := nodeInfo["worker-node"]
	if worker == nil {
		t.Fatalf("missing worker-node entry in nodeInfo")
	}
	if worker.NodeInfo.HasGPU != true {
		t.Fatalf("expected worker-node HasGPU true, got %v", worker.NodeInfo.HasGPU)
	}
	if worker.NodeInfo.IsInterLink {
		t.Fatalf("worker-node should not be marked as interlink")
	}

	interlink := nodeInfo["interlink-node"]
	if interlink == nil {
		t.Fatalf("missing interlink-node entry in nodeInfo")
	}
	if !interlink.NodeInfo.IsInterLink {
		t.Fatalf("expected interlink-node IsInterLink true")
	}
	if interlink.NodeInfo.HasGPU {
		t.Fatalf("interlink-node should not expose GPU resources")
	}
}

func TestGetMetricsInfoUpdatesUsage(t *testing.T) {
	fakeClient, metricsClient := makeFakeClients()
	clusterInfo := &GeneralInfo{}
	nodeInfo, err := getNodesInfo(fakeClient, clusterInfo)
	if err != nil {
		t.Fatalf("getNodesInfo returned unexpected error: %v", err)
	}

	if err := getMetricsInfo(fakeClient, metricsClient.MetricsV1beta1(), nodeInfo, clusterInfo); err != nil {
		t.Fatalf("getMetricsInfo returned unexpected error: %v", err)
	}

	if clusterInfo.NumberNodes != 2 {
		t.Fatalf("expected NumberNodes 2, got %d", clusterInfo.NumberNodes)
	}

	if clusterInfo.CPUFreeTotal != (4000-1000)+(2000-500) {
		t.Fatalf("unexpected CPUFreeTotal: %d", clusterInfo.CPUFreeTotal)
	}

	worker := nodeInfo["worker-node"].NodeInfo
	if worker.CPUUsage != "1000" || worker.MemoryUsage != "4294967296" {
		t.Fatalf("unexpected usage data for worker-node: %+v", worker)
	}
	if worker.CPUPercentage != "25.00" {
		t.Fatalf("expected CPU percentage to be 25.00, got %s", worker.CPUPercentage)
	}
}

func TestGetDeploymentInfoPopulatesOscarSection(t *testing.T) {
	fakeClient, _ := makeFakeClients()
	cfg := &types.Config{
		Namespace:       "oscar",
		OIDCEnable:      true,
		OIDCValidIssuers: []string{"issuer"},
		OIDCGroups:      []string{"group"},
	}

	clusterInfo := &GeneralInfo{}
	if err := getDeploymentInfo(fakeClient, cfg, clusterInfo); err != nil {
		t.Fatalf("getDeploymentInfo returned unexpected error: %v", err)
	}

	if clusterInfo.OSCAR.DeploymentName != "oscar" {
		t.Fatalf("expected deployment name 'oscar', got %s", clusterInfo.OSCAR.DeploymentName)
	}
	if !clusterInfo.OSCAR.DeploymentReady {
		t.Fatalf("expected DeploymentReady true")
	}
	if !clusterInfo.OSCAR.OIDC.Enabled {
		t.Fatalf("expected OIDC.Enabled to mirror config")
	}
}

func TestGetJobsInfoSummarisesStatus(t *testing.T) {
	fakeClient, _ := makeFakeClients()
	clusterInfo := &GeneralInfo{
		OSCAR: OscarInfo{},
	}
	cfg := &types.Config{
		ServicesNamespace: "oscar-svc",
	}

	if err := getJobsInfo(cfg, fakeClient, clusterInfo); err != nil {
		t.Fatalf("getJobsInfo returned unexpected error: %v", err)
	}

	if clusterInfo.OSCAR.JobsCount["succeeded"] != 1 || clusterInfo.OSCAR.JobsCount["failed"] != 1 {
		t.Fatalf("unexpected jobs count: %+v", clusterInfo.OSCAR.JobsCount)
	}
	if clusterInfo.OSCAR.PodsInfo.Total != 2 {
		t.Fatalf("expected 2 pods in PodsInfo.Total, got %d", clusterInfo.OSCAR.PodsInfo.Total)
	}
	if clusterInfo.OSCAR.PodsInfo.States["Running"] != 1 {
		t.Fatalf("expected 1 running pod, got %+v", clusterInfo.OSCAR.PodsInfo.States)
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

	var payload GeneralInfo
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.NumberNodes != 2 {
		t.Fatalf("expected NumberNodes 2, got %d", payload.NumberNodes)
	}
	if payload.OSCAR.DeploymentInfo == nil {
		t.Fatal("expected OSCAR.DeploymentInfo to be populated")
	}
	// Non admin users should not receive MinIO information.
	if len(payload.MinIO.Buckets) != 0 {
		t.Fatalf("expected no MinIO buckets for non admin user: %+v", payload.MinIO.Buckets)
	}
}
