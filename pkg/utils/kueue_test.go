package utils

import (
	"context"
	"math"
	"strconv"
	"testing"

	"github.com/grycap/oscar/v3/pkg/types"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	kueuev1 "sigs.k8s.io/kueue/apis/kueue/v1beta2"
)

func newTestConfig() *types.Config {
	return &types.Config{
		KueueEnable:                       true,
		KueueDefaultCPU:                   "1000m",
		KueueDefaultMemory:                "2Gi",
		KueueDefaultFlavor:                "default-flavor",
		ServicesNamespace:                 "oscar-svc",
		Namespace:                         "oscar",
		Name:                              "oscar",
		ServicePort:                       8080,
		IngressHost:                       "example.com",
		IngressServicesCORSAllowedOrigins: "*",
		IngressServicesCORSAllowedMethods: "GET,POST",
		IngressServicesCORSAllowedHeaders: "*",
	}
}

func newTestService(name, owner string) types.Service {
	return types.Service{
		Name:   name,
		Image:  "ghcr.io/grycap/test",
		Script: "echo test",
		Token:  "s3cr3t",
		Owner:  owner,
		Expose: types.Expose{
			MinScale:      1,
			MaxScale:      3,
			APIPort:       9090,
			CpuThreshold:  55,
			NodePort:      0,
			SetAuth:       false,
			RewriteTarget: false,
		},
		CPU:    "500m",
		Memory: "1Gi",
		Environment: struct {
			Vars    map[string]string `json:"variables"`
			Secrets map[string]string `json:"secrets"`
		}{
			Vars:    map[string]string{},
			Secrets: map[string]string{},
		},
	}
}

func TestSanitizeKueueName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple lowercase",
			input:    "test",
			expected: "test",
		},
		{
			name:     "uppercase conversion",
			input:    "Test",
			expected: "test",
		},
		{
			name:     "special characters replacement",
			input:    "test_service.name",
			expected: "test-service-name",
		},
		{
			name:     "multiple special chars",
			input:    "test@@@service!!!name",
			expected: "test---service---name",
		},
		{
			name:     "leading/trailing special chars",
			input:    "@@test@@",
			expected: "test",
		},
		{
			name:     "numbers allowed",
			input:    "test123",
			expected: "test123",
		},
		{
			name:     "empty string",
			input:    "",
			expected: defaultKueueQueuePrefix,
		},
		{
			name:     "only special chars",
			input:    "@@@!!!",
			expected: defaultKueueQueuePrefix,
		},
		{
			name:     "very long string",
			input:    "this-is-a-very-long-name-that-exceeds-the-kubernetes-dns-label-max-length-limit",
			expected: "this-is-a-very-long-name-that-exceeds-the-kubernetes-dns-label",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeKueueName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeKueueName() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBuildClusterQueueName(t *testing.T) {
	tests := []struct {
		name     string
		owner    string
		expected string
	}{
		{
			name:     "simple owner",
			owner:    "user1",
			expected: "oscar-cq-user1",
		},
		{
			name:     "owner with special chars",
			owner:    "user@test.com",
			expected: "oscar-cq-user-test-com",
		},
		{
			name:     "uppercase owner",
			owner:    "User1",
			expected: "oscar-cq-user1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildClusterQueueName(tt.owner)
			if result != tt.expected {
				t.Errorf("buildClusterQueueName() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBuildLocalQueueName(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		expected    string
	}{
		{
			name:        "simple service name",
			serviceName: "myservice",
			expected:    "oscar-lq-myservice",
		},
		{
			name:        "service with special chars",
			serviceName: "my-service.test",
			expected:    "oscar-lq-my-service-test",
		},
		{
			name:        "uppercase service",
			serviceName: "MyService",
			expected:    "oscar-lq-myservice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildLocalQueueName(tt.serviceName)
			if result != tt.expected {
				t.Errorf("BuildLocalQueueName() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestEnsureKueueUserQueuesDisabled(t *testing.T) {
	cfg := newTestConfig()
	cfg.KueueEnable = false

	err := EnsureKueueUserQueues(context.Background(), cfg, "test-ns", "user1", "service1")
	if err != nil {
		t.Errorf("EnsureKueueUserQueues() with disabled kueue returned error: %v", err)
	}
}

func TestEnsureResourceFlavor(t *testing.T) {
	// This test would require actual kueue client setup
	// For now, we test the logic that doesn't require the client
	t.Skip("Skipping ensureResourceFlavor test - requires kubernetes client setup")
}

func TestEnsureClusterQueue(t *testing.T) {
	t.Skip("Skipping ensureClusterQueue test - requires kubernetes client setup")
}

func TestEnsureLocalQueue(t *testing.T) {
	t.Skip("Skipping ensureLocalQueue test - requires kubernetes client setup")
}

func TestCreateKueueUserQueuesIfDontExistDisabled(t *testing.T) {
	cfg := newTestConfig()
	cfg.KueueEnable = false

	err := CreateKueueUserQueuesIfDontExist(cfg, "user1")
	if err != nil {
		t.Errorf("CreateKueueUserQueuesIfDontExist() with disabled kueue returned error: %v", err)
	}
}

func TestDeleteKueueLocalQueueDisabled(t *testing.T) {
	cfg := newTestConfig()
	cfg.KueueEnable = false

	err := DeleteKueueLocalQueue(context.Background(), cfg, "test-ns", "test-service")
	if err != nil {
		t.Errorf("DeleteKueueLocalQueue() with disabled kueue returned error: %v", err)
	}
}

func TestGetWorkloadSpec(t *testing.T) {
	cfg := newTestConfig()
	service := newTestService("test-service", "testuser")
	namespace := "test-ns"

	// Mock template function
	templateFunc := func(s types.Service, ns string, c *types.Config) v1.PodTemplateSpec {
		return v1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.Name,
				Namespace: ns,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  s.Name,
						Image: s.Image,
					},
				},
			},
		}
	}

	workload := getWorkloadSpec(service, namespace, cfg, templateFunc)

	if workload == nil {
		t.Fatal("getWorkloadSpec() returned nil")
	}

	if workload.Name != service.Name {
		t.Errorf("Expected workload name %s, got %s", service.Name, workload.Name)
	}

	if workload.Namespace != namespace {
		t.Errorf("Expected workload namespace %s, got %s", namespace, workload.Namespace)
	}

	if workload.Spec.QueueName != kueuev1.LocalQueueName(BuildLocalQueueName(service.Name)) {
		t.Errorf("Expected queue name %s, got %s", BuildLocalQueueName(service.Name), workload.Spec.QueueName)
	}

	if len(workload.Spec.PodSets) == 0 {
		t.Error("Expected pod sets to be configured")
	}

	if workload.Spec.PodSets[0].Count != service.Expose.MinScale {
		t.Errorf("Expected pod count %d, got %d", service.Expose.MinScale, workload.Spec.PodSets[0].Count)
	}

	// Verify resource requirements are set
	if workload.Spec.PodSets[0].Template.Spec.Resources == nil {
		t.Error("Expected resource requirements to be set")
	} else {
		cpuReq := workload.Spec.PodSets[0].Template.Spec.Resources.Requests[v1.ResourceCPU]
		memReq := workload.Spec.PodSets[0].Template.Spec.Resources.Requests[v1.ResourceMemory]

		expectedCPU, _ := resource.ParseQuantity(service.CPU)
		expectedMem, _ := resource.ParseQuantity(service.Memory)

		if !cpuReq.Equal(expectedCPU) {
			t.Errorf("Expected CPU %s, got %s", expectedCPU.String(), cpuReq.String())
		}

		if !memReq.Equal(expectedMem) {
			t.Errorf("Expected Memory %s, got %s", expectedMem.String(), memReq.String())
		}
	}
}

func TestGetWorkloadSpecWithoutResources(t *testing.T) {
	cfg := newTestConfig()
	service := newTestService("test-service", "testuser")
	service.CPU = ""
	service.Memory = ""
	namespace := "test-ns"

	templateFunc := func(s types.Service, ns string, c *types.Config) v1.PodTemplateSpec {
		return v1.PodTemplateSpec{
			Spec: v1.PodSpec{
				Containers: []v1.Container{{Name: s.Name, Image: s.Image}},
			},
		}
	}

	workload := getWorkloadSpec(service, namespace, cfg, templateFunc)

	if workload == nil {
		t.Fatal("getWorkloadSpec() returned nil")
	}

	// Should not have resource requirements when CPU/Memory are not specified
	if workload.Spec.PodSets[0].Template.Spec.Resources != nil {
		t.Error("Expected no resource requirements when CPU/Memory are empty")
	}
}

func TestCreateWorkload(t *testing.T) {
	// This test would require in-cluster config, so we test the failure case
	cfg := newTestConfig()
	service := newTestService("test-service", "testuser")
	namespace := "test-ns"

	templateFunc := func(s types.Service, ns string, c *types.Config) v1.PodTemplateSpec {
		return v1.PodTemplateSpec{
			Spec: v1.PodSpec{
				Containers: []v1.Container{{Name: s.Name, Image: s.Image}},
			},
		}
	}

	// Should return false when not in-cluster (test environment)
	result := CreateWorkload(service, namespace, cfg, templateFunc)
	if result {
		t.Error("Expected CreateWorkload() to return false in test environment")
	}
}

func TestDeleteWorkload(t *testing.T) {
	// This test would require in-cluster config, so we test the failure case
	cfg := newTestConfig()
	workloadName := "test-workload"
	namespace := "test-ns"

	// Should return false when not in-cluster (test environment)
	result := DeleteWorkload(workloadName, namespace, cfg)
	if result {
		t.Error("Expected DeleteWorkload() to return false in test environment")
	}
}

func TestUpdateWorkload(t *testing.T) {
	// This test would require in-cluster config, so we test that it doesn't panic
	cfg := newTestConfig()
	service := newTestService("test-service", "testuser")
	namespace := "test-ns"

	templateFunc := func(s types.Service, ns string, c *types.Config) v1.PodTemplateSpec {
		return v1.PodTemplateSpec{
			Spec: v1.PodSpec{
				Containers: []v1.Container{{Name: s.Name, Image: s.Image}},
			},
		}
	}

	// Should not panic when not in-cluster (test environment)
	UpdateWorkload(service, namespace, cfg, templateFunc)
}

func TestVerifyWorkload(t *testing.T) {
	cfg := newTestConfig()
	service := newTestService("test-service", "testuser")
	service.Expose.MinScale = 0 // Test that it gets set to 1

	result := VerifyWorkload(service, "test-ns", cfg)

	// Should return false when not in-cluster (test environment)
	if result {
		t.Error("Expected VerifyWorkload() to return false in test environment")
	}

	// Note: MinScale modification happens inside VerifyWorkload on a copy
	// so we don't test the external modification, just the return value
}

func TestGetResourceOnlyWorkloadSpec(t *testing.T) {
	service := newTestService("test-service", "testuser")
	service.Expose.MinScale = 2
	cfg := newTestConfig()

	workload, err := getResourceOnlyWorkloadSpec(&service, cfg, "test-ns", "verify-test-service", "oscar-lq-test-service")
	if err != nil {
		t.Fatalf("getResourceOnlyWorkloadSpec() returned error: %v", err)
	}

	if workload == nil {
		t.Fatal("getResourceOnlyWorkloadSpec() returned nil workload")
	}

	if workload.Spec.QueueName != kueuev1.LocalQueueName("oscar-lq-test-service") {
		t.Fatalf("QueueName = %q, want %q", workload.Spec.QueueName, "oscar-lq-test-service")
	}

	if len(workload.Spec.PodSets) != 1 {
		t.Fatalf("PodSets length = %d, want 1", len(workload.Spec.PodSets))
	}
	if workload.Spec.PodSets[0].Count != service.Expose.MinScale {
		t.Fatalf("service podset replicas = %d, want %d", workload.Spec.PodSets[0].Count, service.Expose.MinScale)
	}

	resources := workload.Spec.PodSets[0].Template.Spec.Containers[0].Resources

	if _, ok := resources.Requests[v1.ResourceCPU]; !ok {
		t.Fatal("expected CPU request in resource-only workload")
	}

	if _, ok := resources.Requests[v1.ResourceMemory]; !ok {
		t.Fatal("expected memory request in resource-only workload")
	}
}

func TestGetResourceOnlyWorkloadSpecUsesSynchronousMinScale(t *testing.T) {
	service := newTestService("test-service", "testuser")
	service.Expose.APIPort = 0
	service.Synchronous.MinScale = 3
	cfg := newTestConfig()

	workload, err := getResourceOnlyWorkloadSpec(&service, cfg, "test-ns", "verify-test-service", "oscar-lq-test-service")
	if err != nil {
		t.Fatalf("getResourceOnlyWorkloadSpec() returned error: %v", err)
	}

	if workload.Spec.PodSets[0].Count != int32(service.Synchronous.MinScale) {
		t.Fatalf("service podset replicas = %d, want %d", workload.Spec.PodSets[0].Count, service.Synchronous.MinScale)
	}
}

func TestGetResourceOnlyWorkloadSpecSynchronousMinScaleOverflow(t *testing.T) {
	if strconv.IntSize <= 32 {
		t.Skip("int overflow test requires 64-bit int")
	}

	service := newTestService("test-service", "testuser")
	service.Expose.APIPort = 0
	overflowMinScale := int64(math.MaxInt32) + 1
	service.Synchronous.MinScale = int(overflowMinScale)
	cfg := newTestConfig()

	workload, err := getResourceOnlyWorkloadSpec(&service, cfg, "test-ns", "verify-test-service", "oscar-lq-test-service")
	if err == nil {
		t.Fatalf("getResourceOnlyWorkloadSpec() expected overflow error, got workload: %#v", workload)
	}
}

func TestGetResourceOnlyWorkloadSpecWithoutResources(t *testing.T) {
	service := newTestService("test-service", "testuser")
	service.CPU = ""
	service.Memory = ""

	cfg := newTestConfig()
	workload, err := getResourceOnlyWorkloadSpec(&service, cfg, "test-ns", "verify-test-service", "oscar-lq-test-service")
	if err != nil {
		t.Fatalf("getResourceOnlyWorkloadSpec() returned error: %v", err)
	}
	if workload == nil {
		t.Fatal("getResourceOnlyWorkloadSpec() returned nil workload")
	}

	resources := workload.Spec.PodSets[0].Template.Spec.Containers[0].Resources

	cpuReq, ok := resources.Requests[v1.ResourceCPU]
	if !ok {
		t.Fatal("expected CPU request in resource-only workload")
	}
	memReq, ok := resources.Requests[v1.ResourceMemory]
	if !ok {
		t.Fatal("expected memory request in resource-only workload")
	}

	if !cpuReq.Equal(defaultCpuRequest) {
		t.Fatalf("CPU request = %s, want %s", cpuReq.String(), defaultCpuRequest.String())
	}
	if !memReq.Equal(defaultMemoryRequest) {
		t.Fatalf("memory request = %s, want %s", memReq.String(), defaultMemoryRequest.String())
	}
}

func TestGetResourceOnlyWorkloadSpecWithKServePodSet(t *testing.T) {
	service := newTestService("test-service", "testuser")
	service.Expose.MinScale = 2
	service.Kserve = &types.Kserve{
		ModelFormat: "sklearn",
		StorageUri:  "s3://models/sklearn",
		MinScale:    3,
		CPU:         "1",
		Memory:      "2Gi",
		EnableGPU:   true,
	}

	cfg := newTestConfig()
	cfg.KserveEnable = true
	cfg.ExposedServicesRouteKind = "httproute"

	workload, err := getResourceOnlyWorkloadSpec(&service, cfg, "test-ns", "verify-test-service", "oscar-lq-test-service")
	if err != nil {
		t.Fatalf("getResourceOnlyWorkloadSpec() returned error: %v", err)
	}
	if workload == nil {
		t.Fatal("getResourceOnlyWorkloadSpec() returned nil workload")
	}

	if len(workload.Spec.PodSets) != 2 {
		t.Fatalf("PodSets length = %d, want 2", len(workload.Spec.PodSets))
	}

	servicePodSet := workload.Spec.PodSets[0]
	if servicePodSet.Count != service.Expose.MinScale {
		t.Fatalf("service podset replicas = %d, want %d", servicePodSet.Count, service.Expose.MinScale)
	}

	kservePodSet := workload.Spec.PodSets[1]
	if kservePodSet.Count != service.Kserve.MinScale {
		t.Fatalf("kserve podset replicas = %d, want %d", kservePodSet.Count, service.Kserve.MinScale)
	}

	kserveResources := kservePodSet.Template.Spec.Containers[0].Resources

	cpuReq, ok := kserveResources.Requests[v1.ResourceCPU]
	if !ok {
		t.Fatal("expected KServe CPU request in resource-only workload")
	}
	if cpuReq.String() != "1" {
		t.Fatalf("KServe CPU request = %s, want %s", cpuReq.String(), "1")
	}

	memReq, ok := kserveResources.Requests[v1.ResourceMemory]
	if !ok {
		t.Fatal("expected KServe memory request in resource-only workload")
	}
	if memReq.String() != "2Gi" {
		t.Fatalf("KServe memory request = %s, want %s", memReq.String(), "2Gi")
	}

	gpuReq, ok := kserveResources.Requests["nvidia.com/gpu"]
	if !ok {
		t.Fatal("expected KServe GPU request in resource-only workload")
	}
	if gpuReq.String() != "1" {
		t.Fatalf("KServe GPU request = %s, want %s", gpuReq.String(), "1")
	}
}

func TestVerifyWorkloadByResources(t *testing.T) {
	cfg := newTestConfig()
	service := newTestService("test-service", "testuser")

	result := VerifyWorkloadByResources(service, cfg)

	// Should return false when not in-cluster (test environment)
	if result {
		t.Error("Expected VerifyWorkloadByResources() to return false in test environment")
	}
}

func TestWorkloadIsAdmitted(t *testing.T) {
	tests := []struct {
		name string
		wl   *kueuev1.Workload
		want bool
	}{
		{
			name: "admitted",
			wl: &kueuev1.Workload{
				Status: kueuev1.WorkloadStatus{
					Conditions: []metav1.Condition{
						{
							Type:   string(kueuev1.WorkloadAdmitted),
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "not admitted",
			wl: &kueuev1.Workload{
				Status: kueuev1.WorkloadStatus{
					Conditions: []metav1.Condition{
						{
							Type:   string(kueuev1.WorkloadAdmitted),
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "no conditions",
			wl:   &kueuev1.Workload{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := workloadIsAdmitted(tt.wl); got != tt.want {
				t.Fatalf("workloadIsAdmitted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckWorkloadAdmited(t *testing.T) {
	// This test verifies the function doesn't panic when not in-cluster
	cfg := newTestConfig()
	service := newTestService("test-service", "testuser")
	kubeClient := fake.NewSimpleClientset()

	templateFunc := func(s types.Service, ns string, c *types.Config) *apps.Deployment {
		return &apps.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.Name,
				Namespace: ns,
			},
		}
	}

	// Should not panic when not in-cluster (test environment)
	CheckWorkloadAdmited(service, "test-ns", cfg, kubeClient, templateFunc)
}

// Integration test that simulates the full queue setup flow
func TestEnsureKueueUserQueuesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would require actual in-cluster config to work fully
	// For now, we test the disabled case
	cfg := newTestConfig()
	cfg.KueueEnable = false

	err := EnsureKueueUserQueues(context.Background(), cfg, "test-ns", "testuser", "test-service")
	if err != nil {
		t.Errorf("EnsureKueueUserQueues() failed with disabled kueue: %v", err)
	}
}

func TestCreateKueueUserQueuesIfDontExistIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would require actual in-cluster config to work fully
	// For now, we test the disabled case
	cfg := newTestConfig()
	cfg.KueueEnable = false

	err := CreateKueueUserQueuesIfDontExist(cfg, "testuser")
	if err != nil {
		t.Errorf("CreateKueueUserQueuesIfDontExist() failed with disabled kueue: %v", err)
	}
}
