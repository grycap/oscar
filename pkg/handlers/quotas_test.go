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
	"errors"
	"testing"

	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newTestConfig() *types.Config {
	return &types.Config{
		Username: "admin",
		Password: "admin123",
	}
}

func TestMakeGetOwnQuotaHandler(t *testing.T) {
	cfg := newTestConfig()
	qb := types.QuotaBackend{}
	handler := MakeGetOwnQuotaHandler(qb, cfg)

	if handler == nil {
		t.Error("Expected handler to be created")
	}
}

func TestMakeGetUserQuotaHandler(t *testing.T) {
	cfg := newTestConfig()
	qb := types.QuotaBackend{}
	handler := MakeGetUserQuotaHandler(qb, cfg)

	if handler == nil {
		t.Error("Expected handler to be created")
	}
}

func TestMakeUpdateUserQuotaHandler(t *testing.T) {
	cfg := newTestConfig()
	qb := types.QuotaBackend{}
	handler := MakeUpdateUserQuotaHandler(qb, cfg)

	if handler == nil {
		t.Error("Expected handler to be created")
	}
}

func TestQuotaResponseStructures(t *testing.T) {
	t.Run("types.QuotaResponse JSON serialization", func(t *testing.T) {
		resp := types.QuotaResponse{
			UserID:       "user123",
			ClusterQueue: "oscar-cq-user123",
			Resources: map[string]types.QuotaValues{
				"cpu": {
					Max:  1000,
					Used: 500,
				},
				"memory": {
					Max:  2 * 1024 * 1024 * 1024,
					Used: 1 * 1024 * 1024 * 1024,
				},
			},
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Failed to marshal types.QuotaResponse: %v", err)
		}

		var unmarshaled types.QuotaResponse
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal types.QuotaResponse: %v", err)
		}

		if unmarshaled.UserID != resp.UserID {
			t.Errorf("Expected UserID %s, got %s", resp.UserID, unmarshaled.UserID)
		}

		if unmarshaled.ClusterQueue != resp.ClusterQueue {
			t.Errorf("Expected ClusterQueue %s, got %s", resp.ClusterQueue, unmarshaled.ClusterQueue)
		}

		if len(unmarshaled.Resources) != len(resp.Resources) {
			t.Errorf("Expected %d resources, got %d", len(resp.Resources), len(unmarshaled.Resources))
		}

		if unmarshaled.Resources["cpu"].Max != 1000 {
			t.Errorf("Expected CPU Max 1000, got %d", unmarshaled.Resources["cpu"].Max)
		}

		if unmarshaled.Resources["memory"].Max != 2*1024*1024*1024 {
			t.Errorf("Expected Memory Max %d, got %d", 2*1024*1024*1024, unmarshaled.Resources["memory"].Max)
		}
	})

	t.Run("types.QuotaUpdateRequest validation", func(t *testing.T) {
		tests := []struct {
			name    string
			req     types.QuotaUpdateRequest
			isValid bool
		}{
			{
				name: "valid CPU and memory",
				req: types.QuotaUpdateRequest{
					CPU:    "1000m",
					Memory: "2Gi",
				},
				isValid: true,
			},
			{
				name: "only CPU",
				req: types.QuotaUpdateRequest{
					CPU:    "1000m",
					Memory: "",
				},
				isValid: true,
			},
			{
				name: "only memory",
				req: types.QuotaUpdateRequest{
					CPU:    "",
					Memory: "2Gi",
				},
				isValid: true,
			},
			{
				name: "empty CPU and memory",
				req: types.QuotaUpdateRequest{
					CPU:    "",
					Memory: "",
				},
				isValid: false,
			},
			{
				name: "only volumes",
				req: types.QuotaUpdateRequest{
					Volumes: &types.VolumeQuotaUpdate{MaxDiskperVolume: "5Gi"},
				},
				isValid: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				jsonData, err := json.Marshal(tt.req)
				if err != nil {
					t.Fatalf("Failed to marshal types.QuotaUpdateRequest: %v", err)
				}

				var unmarshaled types.QuotaUpdateRequest
				err = json.Unmarshal(jsonData, &unmarshaled)
				if err != nil {
					t.Fatalf("Failed to unmarshal types.QuotaUpdateRequest: %v", err)
				}

				// Test validation logic (CPU, memory or volume quotas must be provided)
				hasValidField := unmarshaled.CPU != "" || unmarshaled.Memory != "" || hasVolumeQuotaUpdate(unmarshaled.Volumes)
				if hasValidField != tt.isValid {
					t.Errorf("Expected valid=%t, got valid=%t", tt.isValid, hasValidField)
				}
			})
		}
	})
}

func TestFetchQuotaSkipped(t *testing.T) {
	t.Skip("fetchQuota requires a valid Kueue client to be initialized")
}

func TestFetchQuotaIncludesVolumeQuotas(t *testing.T) {
	user := "user@example.org"
	cfg := &types.Config{
		ServicesNamespace: "oscar-svc",
		VolumeEnable:      true,
		VolumeAvailable:   "7Gi",
		VolumeMax:         "5",
		VolumeMaxDisk:     "5Gi",
		VolumeMinDisk:     "200Mi",
	}
	namespace := utils.BuildUserNamespace(cfg, user)
	quotaName := auth.FormatUID(user)
	client := fake.NewSimpleClientset(
		&corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{Name: quotaName, Namespace: namespace},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourceRequestsStorage:        resource.MustParse("7Gi"),
					corev1.ResourcePersistentVolumeClaims: resource.MustParse("5"),
				},
			},
			Status: corev1.ResourceQuotaStatus{
				Used: corev1.ResourceList{
					corev1.ResourceRequestsStorage:        resource.MustParse("2Gi"),
					corev1.ResourcePersistentVolumeClaims: resource.MustParse("1"),
				},
			},
		},
		&corev1.LimitRange{
			ObjectMeta: metav1.ObjectMeta{Name: quotaName, Namespace: namespace},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{
					{
						Type: corev1.LimitTypePersistentVolumeClaim,
						Max:  corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("5Gi")},
						Min:  corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("200Mi")},
					},
				},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "oscar-pvc",
				Namespace: namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")},
				},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "legacy-workspace",
				Namespace: namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
				},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managed-volume",
				Namespace: namespace,
				Labels: map[string]string{
					types.ManagedVolumeLabel: "true",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
				},
			},
		},
	)

	resp, err := fetchQuota(t.Context(), cfg, types.QuotaBackend{KubeClientset: client}, user)
	if err != nil {
		t.Fatalf("unexpected fetchQuota error: %v", err)
	}
	if resp.Volumes == nil {
		t.Fatalf("expected volume quotas in response")
	}
	if resp.Volumes.Disk.Max != "4Gi" || resp.Volumes.Disk.Used != "1Gi" {
		t.Fatalf("unexpected disk quota values: %+v", resp.Volumes.Disk)
	}
	if resp.Volumes.Volumes.Max != "3" || resp.Volumes.Volumes.Used != "1" {
		t.Fatalf("unexpected pvc quota values: %+v", resp.Volumes.Volumes)
	}
	if resp.Volumes.MaxDiskperVolume != "5Gi" || resp.Volumes.MinDiskperVolume != "200Mi" {
		t.Fatalf("unexpected per-volume quota values: %+v", resp.Volumes)
	}
}

func TestUpdateVolumeQuotaMergesPartialUpdate(t *testing.T) {
	user := "user@example.org"
	cfg := &types.Config{
		ServicesNamespace: "oscar-svc",
		VolumeEnable:      true,
		VolumeAvailable:   "7Gi",
		VolumeMax:         "5",
		VolumeMaxDisk:     "5Gi",
		VolumeMinDisk:     "200Mi",
	}
	namespace := utils.BuildUserNamespace(cfg, user)
	quotaName := auth.FormatUID(user)
	client := fake.NewSimpleClientset(
		&corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{Name: quotaName, Namespace: namespace},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourceRequestsStorage:        resource.MustParse("7Gi"),
					corev1.ResourcePersistentVolumeClaims: resource.MustParse("5"),
				},
			},
		},
		&corev1.LimitRange{
			ObjectMeta: metav1.ObjectMeta{Name: quotaName, Namespace: namespace},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{
					{
						Type: corev1.LimitTypePersistentVolumeClaim,
						Max:  corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("5Gi")},
						Min:  corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("200Mi")},
					},
				},
			},
		},
	)

	err := updateVolumeQuota(user, &types.VolumeQuotaUpdate{MaxDiskperVolume: "10Gi"}, cfg, types.QuotaBackend{KubeClientset: client})
	if err != nil {
		t.Fatalf("unexpected updateVolumeQuota error: %v", err)
	}

	quota, err := client.CoreV1().ResourceQuotas(namespace).Get(t.Context(), quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected resource quota get error: %v", err)
	}
	totalStorage := quota.Spec.Hard[corev1.ResourceRequestsStorage]
	if totalStorage != resource.MustParse("7Gi") {
		t.Fatalf("expected total storage quota to be preserved, got %s", totalStorage.String())
	}
	limit, err := client.CoreV1().LimitRanges(namespace).Get(t.Context(), quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected limit range get error: %v", err)
	}
	maxStorage := limit.Spec.Limits[0].Max[corev1.ResourceStorage]
	if maxStorage != resource.MustParse("10Gi") {
		t.Fatalf("expected per-volume max to be updated, got %s", maxStorage.String())
	}
	minStorage := limit.Spec.Limits[0].Min[corev1.ResourceStorage]
	if minStorage != resource.MustParse("200Mi") {
		t.Fatalf("expected per-volume min to be preserved, got %s", minStorage.String())
	}
}

func TestUpdateVolumeQuotaStoresUserVisibleDiskQuota(t *testing.T) {
	user := "user@example.org"
	cfg := &types.Config{
		ServicesNamespace: "oscar-svc",
		VolumeEnable:      true,
		VolumeAvailable:   "7Gi",
		VolumeMax:         "5",
		VolumeMaxDisk:     "5Gi",
		VolumeMinDisk:     "200Mi",
	}
	namespace := utils.BuildUserNamespace(cfg, user)
	quotaName := auth.FormatUID(user)
	client := fake.NewSimpleClientset(
		&corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{Name: quotaName, Namespace: namespace},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourceRequestsStorage:        resource.MustParse("7Gi"),
					corev1.ResourcePersistentVolumeClaims: resource.MustParse("5"),
				},
			},
		},
		&corev1.LimitRange{
			ObjectMeta: metav1.ObjectMeta{Name: quotaName, Namespace: namespace},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{
					{
						Type: corev1.LimitTypePersistentVolumeClaim,
						Max:  corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("5Gi")},
						Min:  corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("200Mi")},
					},
				},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "oscar-pvc",
				Namespace: namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")},
				},
			},
		},
	)

	err := updateVolumeQuota(user, &types.VolumeQuotaUpdate{Disk: "10Gi", Volumes: "3"}, cfg, types.QuotaBackend{KubeClientset: client})
	if err != nil {
		t.Fatalf("unexpected updateVolumeQuota error: %v", err)
	}

	quota, err := client.CoreV1().ResourceQuotas(namespace).Get(t.Context(), quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected resource quota get error: %v", err)
	}
	rawStorage := quota.Spec.Hard[corev1.ResourceRequestsStorage]
	if rawStorage != resource.MustParse("12Gi") {
		t.Fatalf("expected raw storage quota to include non-managed PVC usage, got %s", rawStorage.String())
	}
	rawVolumes := quota.Spec.Hard[corev1.ResourcePersistentVolumeClaims]
	if rawVolumes.Value() != 4 {
		t.Fatalf("expected raw volume quota to include non-managed PVC count, got %s", rawVolumes.String())
	}

	resp, err := fetchQuota(t.Context(), cfg, types.QuotaBackend{KubeClientset: client}, user)
	if err != nil {
		t.Fatalf("unexpected fetchQuota error: %v", err)
	}
	if resp.Volumes.Disk.Max != "10Gi" {
		t.Fatalf("expected visible disk quota 10Gi, got %s", resp.Volumes.Disk.Max)
	}
	if resp.Volumes.Volumes.Max != "3" {
		t.Fatalf("expected visible volume quota 3, got %s", resp.Volumes.Volumes.Max)
	}
}

func TestEnsureKueueQuotasEnabled(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		err := ensureKueueQuotasEnabled(&types.Config{KueueEnable: false})
		if !errors.Is(err, errKueueDisabled) {
			t.Fatalf("expected errKueueDisabled, got %v", err)
		}
	})

	t.Run("enabled", func(t *testing.T) {
		if err := ensureKueueQuotasEnabled(&types.Config{KueueEnable: true}); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})
}

func TestIsMissingKueueAPI(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "server cannot find resource",
			err:  errors.New("the server could not find the requested resource (get clusterqueues.kueue.x-k8s.io test)"),
			want: true,
		},
		{
			name: "no kind match",
			err:  errors.New("no matches for kind \"ClusterQueue\" in version \"kueue.x-k8s.io/v1beta2\""),
			want: true,
		},
		{
			name: "ordinary error",
			err:  errors.New("clusterqueues.kueue.x-k8s.io \"missing\" not found"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMissingKueueAPI(tt.err); got != tt.want {
				t.Fatalf("isMissingKueueAPI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateQuotaSkipped(t *testing.T) {
	t.Skip("updateQuota requires a valid Kueue client to be initialized")
}

func TestUtilityFunctions(t *testing.T) {
	t.Run("cluster queue name generation", func(t *testing.T) {
		user := "testuser"
		expected := utils.BuildClusterQueueName(user)
		actual := "oscar-cq-" + user

		// Test that expected pattern is followed
		if actual != expected {
			t.Errorf("Expected cluster queue name %s, got %s", expected, actual)
		}
	})

	t.Run("resource quantity parsing", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
			valid    bool
		}{
			{"1000m", "1", true},
			{"1", "1", true},
			{"1Gi", "1Gi", true},
			{"invalid", "", false},
			{"", "", true},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				if tt.input == "" || tt.input == "invalid" {
					// Test error cases
					if tt.valid {
						q, err := resource.ParseQuantity(tt.input)
						if err != nil {
							t.Logf("Expected error for invalid input %s: %v", tt.input, err)
						} else {
							t.Errorf("Expected error for invalid input %s, but got quantity %s", tt.input, q.String())
						}
					}
				} else {
					// Test valid cases
					q, err := resource.ParseQuantity(tt.input)
					if err != nil {
						t.Errorf("Expected success for input %s, got error: %v", tt.input, err)
					} else if q.String() != tt.expected {
						t.Errorf("Expected quantity %s, got %s", tt.expected, q.String())
					}
				}
			})
		}
	})
}

func TestQuotaJSONTags(t *testing.T) {
	t.Run("types.QuotaResponse JSON tags", func(t *testing.T) {
		resp := types.QuotaResponse{
			UserID:       "user123",
			ClusterQueue: "oscar-cq-user123",
			Resources: map[string]types.QuotaValues{
				"cpu": {Max: 1000, Used: 500},
			},
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Failed to marshal types.QuotaResponse: %v", err)
		}

		var raw map[string]interface{}
		err = json.Unmarshal(data, &raw)
		if err != nil {
			t.Fatalf("Failed to unmarshal to raw map: %v", err)
		}

		// Check JSON field names match tags
		expectedFields := []string{"user_id", "cluster_queue", "resources"}
		for _, field := range expectedFields {
			if _, exists := raw[field]; !exists {
				t.Errorf("Expected '%s' field in JSON", field)
			}
		}
	})

	t.Run("types.QuotaUpdateRequest JSON tags", func(t *testing.T) {
		req := types.QuotaUpdateRequest{
			CPU:    "1000m",
			Memory: "2Gi",
			Volumes: &types.VolumeQuotaUpdate{
				Disk:             "7Gi",
				Volumes:          "5",
				MaxDiskperVolume: "5Gi",
				MinDiskperVolume: "200Mi",
			},
		}

		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("Failed to marshal types.QuotaUpdateRequest: %v", err)
		}

		var raw map[string]interface{}
		err = json.Unmarshal(data, &raw)
		if err != nil {
			t.Fatalf("Failed to unmarshal to raw map: %v", err)
		}

		// Check JSON field names match tags
		if _, exists := raw["cpu"]; !exists {
			t.Error("Expected 'cpu' field in JSON")
		}

		if _, exists := raw["memory"]; !exists {
			t.Error("Expected 'memory' field in JSON")
		}
		volumes, ok := raw["volumes"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected 'volumes' field in JSON")
		}
		for _, field := range []string{"disk", "volumes", "max_disk_per_volume", "min_disk_per_volume"} {
			if _, exists := volumes[field]; !exists {
				t.Errorf("Expected volumes.%s field in JSON", field)
			}
		}
	})
}
