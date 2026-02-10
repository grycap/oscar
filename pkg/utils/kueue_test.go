package utils

import (
	"context"
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

func TestCreateKueueUserQueuesDisabled(t *testing.T) {
	cfg := newTestConfig()
	cfg.KueueEnable = false

	err := CreateKueueUserQueues(context.Background(), cfg, "user1")
	if err != nil {
		t.Errorf("CreateKueueUserQueues() with disabled kueue returned error: %v", err)
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

func TestCreateKueueUserQueuesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would require actual in-cluster config to work fully
	// For now, we test the disabled case
	cfg := newTestConfig()
	cfg.KueueEnable = false

	err := CreateKueueUserQueues(context.Background(), cfg, "testuser")
	if err != nil {
		t.Errorf("CreateKueueUserQueues() failed with disabled kueue: %v", err)
	}
}
