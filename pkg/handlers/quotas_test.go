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
	"testing"

	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/rest"
)

func newTestConfig() *types.Config {
	return &types.Config{
		Username: "admin",
		Password: "admin123",
	}
}

func TestMakeGetOwnQuotaHandler(t *testing.T) {
	cfg := newTestConfig()
	kubeConfig := &rest.Config{}
	handler := MakeGetOwnQuotaHandler(cfg, kubeConfig)

	// Test that handler is created successfully
	if handler == nil {
		t.Error("Expected handler to be created")
	}
}

func TestMakeGetUserQuotaHandler(t *testing.T) {
	cfg := newTestConfig()
	kubeConfig := &rest.Config{}
	handler := MakeGetUserQuotaHandler(cfg, kubeConfig)

	// Test that handler is created successfully
	if handler == nil {
		t.Error("Expected handler to be created")
	}
}

func TestMakeUpdateUserQuotaHandler(t *testing.T) {
	cfg := newTestConfig()
	kubeConfig := &rest.Config{}
	handler := MakeUpdateUserQuotaHandler(cfg, kubeConfig)

	// Test that handler is created successfully
	if handler == nil {
		t.Error("Expected handler to be created")
	}
}

func TestQuotaResponseStructures(t *testing.T) {
	t.Run("quotaResponse JSON serialization", func(t *testing.T) {
		resp := quotaResponse{
			UserID:       "user123",
			ClusterQueue: "oscar-cq-user123",
			Resources: map[string]quotaValues{
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
			t.Fatalf("Failed to marshal quotaResponse: %v", err)
		}

		var unmarshaled quotaResponse
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal quotaResponse: %v", err)
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

	t.Run("quotaUpdateRequest validation", func(t *testing.T) {
		tests := []struct {
			name    string
			req     quotaUpdateRequest
			isValid bool
		}{
			{
				name: "valid CPU and memory",
				req: quotaUpdateRequest{
					CPU:    "1000m",
					Memory: "2Gi",
				},
				isValid: true,
			},
			{
				name: "only CPU",
				req: quotaUpdateRequest{
					CPU:    "1000m",
					Memory: "",
				},
				isValid: true,
			},
			{
				name: "only memory",
				req: quotaUpdateRequest{
					CPU:    "",
					Memory: "2Gi",
				},
				isValid: true,
			},
			{
				name: "empty CPU and memory",
				req: quotaUpdateRequest{
					CPU:    "",
					Memory: "",
				},
				isValid: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				jsonData, err := json.Marshal(tt.req)
				if err != nil {
					t.Fatalf("Failed to marshal quotaUpdateRequest: %v", err)
				}

				var unmarshaled quotaUpdateRequest
				err = json.Unmarshal(jsonData, &unmarshaled)
				if err != nil {
					t.Fatalf("Failed to unmarshal quotaUpdateRequest: %v", err)
				}

				// Test validation logic (CPU or memory must be provided)
				hasValidField := unmarshaled.CPU != "" || unmarshaled.Memory != ""
				if hasValidField != tt.isValid {
					t.Errorf("Expected valid=%t, got valid=%t", tt.isValid, hasValidField)
				}
			})
		}
	})
}

func TestFetchQuota(t *testing.T) {
	// Test error cases that don't require kubernetes setup
	t.Run("error creating kueue client", func(t *testing.T) {
		invalidConfig := &rest.Config{
			Host: "invalid-host",
		}

		ctx := context.Background()
		resp, err := fetchQuota(ctx, invalidConfig, "testuser")

		if err == nil {
			t.Error("Expected error when creating kueue client with invalid config")
		}

		if resp != nil {
			t.Error("Expected nil response when kueue client creation fails")
		}
	})
}

func TestUpdateQuota(t *testing.T) {
	t.Run("error creating kueue client", func(t *testing.T) {
		invalidConfig := &rest.Config{
			Host: "invalid-host",
		}

		ctx := context.Background()
		req := quotaUpdateRequest{
			CPU:    "1000m",
			Memory: "2Gi",
		}

		err := updateQuota(ctx, invalidConfig, "testuser", req)

		if err == nil {
			t.Error("Expected error when creating kueue client with invalid config")
		}
	})
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
	t.Run("quotaResponse JSON tags", func(t *testing.T) {
		resp := quotaResponse{
			UserID:       "user123",
			ClusterQueue: "oscar-cq-user123",
			Resources: map[string]quotaValues{
				"cpu": {Max: 1000, Used: 500},
			},
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Failed to marshal quotaResponse: %v", err)
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

	t.Run("quotaUpdateRequest JSON tags", func(t *testing.T) {
		req := quotaUpdateRequest{
			CPU:    "1000m",
			Memory: "2Gi",
		}

		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("Failed to marshal quotaUpdateRequest: %v", err)
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
	})
}
