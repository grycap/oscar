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

package utils

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/grycap/oscar/v3/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/fake"
)

func TestBuildUserNamespace(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *types.Config
		owner    string
		expected string
	}{
		{
			name:     "default owner returns services namespace",
			cfg:      &types.Config{ServicesNamespace: "oscar-svc"},
			owner:    types.DefaultOwner,
			expected: "oscar-svc",
		},
		{
			name:     "empty owner returns services namespace",
			cfg:      &types.Config{ServicesNamespace: "oscar-svc"},
			owner:    "",
			expected: "oscar-svc",
		},
		{
			name:     "custom owner with default prefix",
			cfg:      &types.Config{ServicesNamespace: "oscar-svc"},
			owner:    "testuser",
			expected: "oscar-svc-testuser-45c571a156ddcef41351a713bcddee5ba7e95460",
		},
		{
			name:     "custom owner with custom prefix",
			cfg:      &types.Config{ServicesNamespace: "custom-svc"},
			owner:    "testuser",
			expected: "custom-svc-testuser-45c571a156ddcef41351a713bcddee5ba7e95460",
		},
		{
			name:     "nil config",
			cfg:      nil,
			owner:    "testuser",
			expected: "",
		},
		{
			name:     "empty services namespace",
			cfg:      &types.Config{ServicesNamespace: ""},
			owner:    "testuser",
			expected: "oscar-svc-testuser-45c571a156ddcef41351a713bcddee5ba7e95460",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildUserNamespace(tt.cfg, tt.owner)
			if result != tt.expected {
				t.Errorf("Expected namespace %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestSanitizeOwner(t *testing.T) {
	tests := []struct {
		name     string
		owner    string
		expected string
	}{
		{
			name:     "empty owner",
			owner:    "",
			expected: "",
		},
		{
			name:     "simple alphanumeric",
			owner:    "testuser",
			expected: "testuser",
		},
		{
			name:     "uppercase letters",
			owner:    "TestUser",
			expected: "testuser",
		},
		{
			name:     "mixed case",
			owner:    "TeStUsEr",
			expected: "testuser",
		},
		{
			name:     "special characters",
			owner:    "test@user#123",
			expected: "test-user-123",
		},
		{
			name:     "spaces and punctuation",
			owner:    "  test user!  ",
			expected: "test-user",
		},
		{
			name:     "numbers only",
			owner:    "123456",
			expected: "123456",
		},
		{
			name:     "very long name",
			owner:    "very-long-owner-name-that-exceeds-kubernetes-limits",
			expected: "very-long-owner-name-that-exceeds-kubernetes-limits", // No truncation needed (51 < 63)
		},
		{
			name:     "invalid characters only",
			owner:    "!@#$%^&*()",
			expected: "", // All chars invalid, falls back to empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeOwner(tt.owner)
			if result != tt.expected {
				t.Errorf("Expected sanitized owner %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestTruncateLabel(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		max      int
		expected string
	}{
		{
			name:     "short label",
			label:    "test",
			max:      10,
			expected: "test",
		},
		{
			name:     "exact length",
			label:    "testlabel",
			max:      9,
			expected: "testlabel",
		},
		{
			name:     "truncate long",
			label:    "verylonglabel",
			max:      8,
			expected: "verylong",
		},
		{
			name:     "truncate with dashes",
			label:    "very-long-test-label",
			max:      10,
			expected: "very-long", // Dashes trimmed
		},
		{
			name:     "zero max",
			label:    "test",
			max:      0,
			expected: "",
		},
		{
			name:     "negative max",
			label:    "test",
			max:      -5,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateLabel(tt.label, tt.max)
			if result != tt.expected {
				t.Errorf("Expected truncated label %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestOwnerHash(t *testing.T) {
	tests := []struct {
		name     string
		owner    string
		expected string
	}{
		{
			name:     "empty owner",
			owner:    "",
			expected: "",
		},
		{
			name:     "simple owner",
			owner:    "testuser",
			expected: "45c571a156ddcef41351a713bcddee5ba7e95460", // SHA1 of "testuser"
		},
		{
			name:     "different owner",
			owner:    "anotheruser",
			expected: "c8fc279baf0f22a79e7b8483fd9c3360403ec959", // SHA1 of "anotheruser"
		},
		{
			name:     "numeric owner",
			owner:    "123456",
			expected: "7c4a8d09ca3762af61e59520943dc26494f8941b", // SHA1 of "123456"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ownerHash(tt.owner)
			if result != tt.expected {
				t.Errorf("Expected owner hash %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		target   string
		expected bool
	}{
		{
			name:     "target exists",
			values:   []string{"val1", "val2", "val3"},
			target:   "val2",
			expected: true,
		},
		{
			name:     "target does not exist",
			values:   []string{"val1", "val2", "val3"},
			target:   "val4",
			expected: false,
		},
		{
			name:     "empty slice",
			values:   []string{},
			target:   "anything",
			expected: false,
		},
		{
			name:     "empty target",
			values:   []string{"val1", "val2"},
			target:   "",
			expected: false,
		},
		{
			name:     "case sensitive",
			values:   []string{"Val1", "val2"},
			target:   "val1",
			expected: false,
		},
		{
			name:     "partial match",
			values:   []string{"val", "value"},
			target:   "val",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsString(tt.values, tt.target)
			if result != tt.expected {
				t.Errorf("Expected %t, got %t", tt.expected, result)
			}
		})
	}
}

func TestBuildSharedPVName(t *testing.T) {
	tests := []struct {
		name      string
		baseName  string
		namespace string
		expected  string
	}{
		{
			name:      "short namespace",
			baseName:  "test-pv",
			namespace: "user123",
			expected:  "test-pv-95c946bf62", // Hash of "user123"
		},
		{
			name:      "long namespace truncated",
			baseName:  "test-pv",
			namespace: "very-long-namespace-name-that-should-be-truncated",
			expected:  "test-pv-1c3b042dd8", // Hash of long namespace
		},
		{
			name:      "empty base name",
			baseName:  "",
			namespace: "user123",
			expected:  "oscar-pv-95c946bf62", // Default base with hash
		},
		{
			name:      "hash truncation",
			baseName:  "test-pv",
			namespace: strings.Repeat("a", 20), // Long hash
			expected:  "test-pv-38666b8ba5",    // First 10 chars of hash
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSharedPVName(tt.baseName, tt.namespace)
			if result != tt.expected {
				t.Errorf("Expected PV name %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestBuildSharedPVSpec(t *testing.T) {
	tests := []struct {
		name string
		base corev1.PersistentVolumeSpec
	}{
		{
			name: "basic PV spec",
			base: corev1.PersistentVolumeSpec{
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/data",
					},
				},
			},
		},
		{
			name: "PV with node affinity",
			base: corev1.PersistentVolumeSpec{
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("2Gi"),
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteMany,
				},
				NodeAffinity: &corev1.VolumeNodeAffinity{
					Required: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "storage",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"ssd"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSharedPVSpec(tt.base)

			// Check that claimRef is nil
			if result.ClaimRef != nil {
				t.Error("Expected ClaimRef to be nil")
			}

			// Check that StorageClassName is empty
			if result.StorageClassName != "" {
				t.Errorf("Expected empty StorageClassName, got %s", result.StorageClassName)
			}

			// Check capacity is preserved
			if !reflect.DeepEqual(result.Capacity, tt.base.Capacity) {
				t.Error("Expected Capacity to be preserved")
			}

			// Check access modes are preserved
			if !reflect.DeepEqual(result.AccessModes, tt.base.AccessModes) {
				t.Error("Expected AccessModes to be preserved")
			}

			// Check reclaim policy is preserved
			if result.PersistentVolumeReclaimPolicy != tt.base.PersistentVolumeReclaimPolicy {
				t.Error("Expected PersistentVolumeReclaimPolicy to be preserved")
			}

			// Check volume source is preserved
			if !reflect.DeepEqual(result.PersistentVolumeSource, tt.base.PersistentVolumeSource) {
				t.Error("Expected PersistentVolumeSource to be preserved")
			}

			// Check node affinity is preserved
			if !reflect.DeepEqual(result.NodeAffinity, tt.base.NodeAffinity) {
				t.Error("Expected NodeAffinity to be preserved")
			}

			// Check volume mode is preserved
			if !reflect.DeepEqual(result.VolumeMode, tt.base.VolumeMode) {
				t.Error("Expected VolumeMode to be preserved")
			}

			// Check mount options include "ro"
			if !containsString(result.MountOptions, "ro") {
				t.Error("Expected MountOptions to include 'ro'")
			}
		})
	}
}

func TestEnsureUserNamespaceBasic(t *testing.T) {
	// Test basic scenarios that don't require full kubernetes setup
	t.Run("nil clientset", func(t *testing.T) {
		ctx := context.Background()
		cfg := &types.Config{ServicesNamespace: "oscar-svc"}
		_, err := EnsureUserNamespace(ctx, nil, cfg, "testuser")
		if err == nil {
			t.Error("Expected error when clientset is nil")
		}

		expectedErr := "kubernetes clientset cannot be nil"
		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
		}
	})

	t.Run("test namespace generation", func(t *testing.T) {
		cfg := &types.Config{ServicesNamespace: "oscar-svc"}

		// Just test namespace generation, don't try to create resources
		expected := BuildUserNamespace(cfg, "testuser")

		if expected != "oscar-svc-testuser-45c571a156ddcef41351a713bcddee5ba7e95460" {
			t.Errorf("Expected namespace %s, got %s", "oscar-svc-testuser-45c571a156ddcef41351a713bcddee5ba7e95460", expected)
		}
	})

	t.Run("empty config", func(t *testing.T) {
		ctx := context.Background()
		clientset := fake.NewSimpleClientset()

		namespace, err := EnsureUserNamespace(ctx, clientset, nil, "testuser")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if namespace != "" {
			t.Errorf("Expected empty namespace when config is nil, got %s", namespace)
		}
	})
}

func TestNamespaceConstants(t *testing.T) {
	// Test that constants have expected values
	if maxNamespaceLength != 63 {
		t.Errorf("Expected maxNamespaceLength 63, got %d", maxNamespaceLength)
	}

	if controllerRoleName != "oscar-controller" {
		t.Errorf("Expected controllerRoleName 'oscar-controller', got '%s'", controllerRoleName)
	}

	if controllerRoleBindingName != "oscar-controller-binding" {
		t.Errorf("Expected controllerRoleBindingName 'oscar-controller-binding', got '%s'", controllerRoleBindingName)
	}

	if namespaceManagedByLabel != "app.kubernetes.io/managed-by" {
		t.Errorf("Expected namespaceManagedByLabel 'app.kubernetes.io/managed-by', got '%s'", namespaceManagedByLabel)
	}

	if namespaceManagedByValue != "oscar" {
		t.Errorf("Expected namespaceManagedByValue 'oscar', got '%s'", namespaceManagedByValue)
	}

	if namespaceOwnerLabel != "oscar.grycap.upv.es/owner" {
		t.Errorf("Expected namespaceOwnerLabel 'oscar.grycap.upv.es/owner', got '%s'", namespaceOwnerLabel)
	}

	if namespaceLifecycleLabel != "oscar.grycap.upv.es/lifecycle" {
		t.Errorf("Expected namespaceLifecycleLabel 'oscar.grycap.upv.es/lifecycle', got '%s'", namespaceLifecycleLabel)
	}

	if namespaceLifecycleActive != "active" {
		t.Errorf("Expected namespaceLifecycleActive 'active', got '%s'", namespaceLifecycleActive)
	}

	if namespaceHashPaddingDivider != "-" {
		t.Errorf("Expected namespaceHashPaddingDivider '-', got '%s'", namespaceHashPaddingDivider)
	}
}
