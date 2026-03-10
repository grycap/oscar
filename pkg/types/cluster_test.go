/*
Copyright (C) GRyCAP - I3M - UPV

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"encoding/json"
	"testing"
)

func TestClusterJSONSerialization(t *testing.T) {
	cluster := Cluster{
		Endpoint:     "https://oscar.example.com",
		AuthUser:     "admin",
		AuthPassword: "secret123",
		SSLVerify:    true,
	}

	// Test JSON marshaling
	data, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("Failed to marshal Cluster: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled Cluster
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal Cluster: %v", err)
	}

	if unmarshaled.Endpoint != cluster.Endpoint {
		t.Errorf("Expected Endpoint %s, got %s", cluster.Endpoint, unmarshaled.Endpoint)
	}

	if unmarshaled.AuthUser != cluster.AuthUser {
		t.Errorf("Expected AuthUser %s, got %s", cluster.AuthUser, unmarshaled.AuthUser)
	}

	if unmarshaled.AuthPassword != cluster.AuthPassword {
		t.Errorf("Expected AuthPassword %s, got %s", cluster.AuthPassword, unmarshaled.AuthPassword)
	}

	if unmarshaled.SSLVerify != cluster.SSLVerify {
		t.Errorf("Expected SSLVerify %t, got %t", cluster.SSLVerify, unmarshaled.SSLVerify)
	}
}

func TestClusterPartialFields(t *testing.T) {
	tests := []struct {
		name    string
		cluster Cluster
	}{
		{
			name: "only endpoint",
			cluster: Cluster{
				Endpoint: "https://test.example.com",
			},
		},
		{
			name: "endpoint with auth",
			cluster: Cluster{
				Endpoint:     "https://test.example.com",
				AuthUser:     "user",
				AuthPassword: "pass",
			},
		},
		{
			name: "all false except endpoint",
			cluster: Cluster{
				Endpoint:     "https://test.example.com",
				AuthUser:     "",
				AuthPassword: "",
				SSLVerify:    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.cluster)
			if err != nil {
				t.Fatalf("Failed to marshal partial Cluster: %v", err)
			}

			var unmarshaled Cluster
			err = json.Unmarshal(data, &unmarshaled)
			if err != nil {
				t.Fatalf("Failed to unmarshal partial Cluster: %v", err)
			}

			if unmarshaled.Endpoint != tt.cluster.Endpoint {
				t.Errorf("Expected Endpoint %s, got %s", tt.cluster.Endpoint, unmarshaled.Endpoint)
			}

			if unmarshaled.AuthUser != tt.cluster.AuthUser {
				t.Errorf("Expected AuthUser %s, got %s", tt.cluster.AuthUser, unmarshaled.AuthUser)
			}

			if unmarshaled.AuthPassword != tt.cluster.AuthPassword {
				t.Errorf("Expected AuthPassword %s, got %s", tt.cluster.AuthPassword, unmarshaled.AuthPassword)
			}

			if unmarshaled.SSLVerify != tt.cluster.SSLVerify {
				t.Errorf("Expected SSLVerify %t, got %t", tt.cluster.SSLVerify, unmarshaled.SSLVerify)
			}
		})
	}
}

func TestClusterJSONTags(t *testing.T) {
	cluster := Cluster{
		Endpoint:     "https://oscar.example.com",
		AuthUser:     "admin",
		AuthPassword: "secret123",
		SSLVerify:    true,
	}

	data, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("Failed to marshal Cluster: %v", err)
	}

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	if err != nil {
		t.Fatalf("Failed to unmarshal to raw map: %v", err)
	}

	// Check that JSON field names match the tags
	if _, exists := raw["endpoint"]; !exists {
		t.Error("Expected 'endpoint' field in JSON")
	}

	if _, exists := raw["auth_user"]; !exists {
		t.Error("Expected 'auth_user' field in JSON")
	}

	if _, exists := raw["auth_password"]; !exists {
		t.Error("Expected 'auth_password' field in JSON")
	}

	if _, exists := raw["ssl_verify"]; !exists {
		t.Error("Expected 'ssl_verify' field in JSON")
	}

	// Check field values
	if raw["endpoint"] != cluster.Endpoint {
		t.Errorf("Expected endpoint value %s, got %v", cluster.Endpoint, raw["endpoint"])
	}

	if raw["auth_user"] != cluster.AuthUser {
		t.Errorf("Expected auth_user value %s, got %v", cluster.AuthUser, raw["auth_user"])
	}

	if raw["auth_password"] != cluster.AuthPassword {
		t.Errorf("Expected auth_password value %s, got %v", cluster.AuthPassword, raw["auth_password"])
	}

	if raw["ssl_verify"] != cluster.SSLVerify {
		t.Errorf("Expected ssl_verify value %t, got %v", cluster.SSLVerify, raw["ssl_verify"])
	}
}

func TestClusterEmptyFields(t *testing.T) {
	cluster := Cluster{}

	data, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("Failed to marshal empty Cluster: %v", err)
	}

	var unmarshaled Cluster
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal empty Cluster: %v", err)
	}

	if unmarshaled.Endpoint != "" {
		t.Errorf("Expected empty Endpoint, got '%s'", unmarshaled.Endpoint)
	}

	if unmarshaled.AuthUser != "" {
		t.Errorf("Expected empty AuthUser, got '%s'", unmarshaled.AuthUser)
	}

	if unmarshaled.AuthPassword != "" {
		t.Errorf("Expected empty AuthPassword, got '%s'", unmarshaled.AuthPassword)
	}

	if unmarshaled.SSLVerify != false {
		t.Errorf("Expected SSLVerify false, got %t", unmarshaled.SSLVerify)
	}
}

func TestClusterZeroValues(t *testing.T) {
	// Test with zero values explicitly set
	cluster := Cluster{
		Endpoint:     "",
		AuthUser:     "",
		AuthPassword: "",
		SSLVerify:    false,
	}

	data, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("Failed to marshal zero-value Cluster: %v", err)
	}

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	if err != nil {
		t.Fatalf("Failed to unmarshal to raw map: %v", err)
	}

	// All fields should be present even with zero values
	if _, exists := raw["endpoint"]; !exists {
		t.Error("Expected 'endpoint' field to be present")
	}

	if _, exists := raw["auth_user"]; !exists {
		t.Error("Expected 'auth_user' field to be present")
	}

	if _, exists := raw["auth_password"]; !exists {
		t.Error("Expected 'auth_password' field to be present")
	}

	if _, exists := raw["ssl_verify"]; !exists {
		t.Error("Expected 'ssl_verify' field to be present")
	}

	if raw["endpoint"] != "" {
		t.Errorf("Expected endpoint empty string, got %v", raw["endpoint"])
	}

	if raw["auth_user"] != "" {
		t.Errorf("Expected auth_user empty string, got %v", raw["auth_user"])
	}

	if raw["auth_password"] != "" {
		t.Errorf("Expected auth_password empty string, got %v", raw["auth_password"])
	}

	if raw["ssl_verify"] != false {
		t.Errorf("Expected ssl_verify false, got %v", raw["ssl_verify"])
	}
}

func TestClusterURLFormats(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{
			name:     "https with port",
			endpoint: "https://oscar.example.com:8443",
		},
		{
			name:     "https without port",
			endpoint: "https://oscar.example.com",
		},
		{
			name:     "http with port",
			endpoint: "http://oscar.example.com:8080",
		},
		{
			name:     "http without port",
			endpoint: "http://oscar.example.com",
		},
		{
			name:     "localhost with port",
			endpoint: "http://localhost:8080",
		},
		{
			name:     "localhost https",
			endpoint: "https://localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := Cluster{
				Endpoint:  tt.endpoint,
				AuthUser:  "test",
				SSLVerify: true,
			}

			data, err := json.Marshal(cluster)
			if err != nil {
				t.Fatalf("Failed to marshal Cluster with endpoint %s: %v", tt.endpoint, err)
			}

			var unmarshaled Cluster
			err = json.Unmarshal(data, &unmarshaled)
			if err != nil {
				t.Fatalf("Failed to unmarshal Cluster with endpoint %s: %v", tt.endpoint, err)
			}

			if unmarshaled.Endpoint != tt.endpoint {
				t.Errorf("Expected Endpoint %s, got %s", tt.endpoint, unmarshaled.Endpoint)
			}
		})
	}
}
