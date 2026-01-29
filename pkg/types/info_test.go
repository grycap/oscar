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

func TestInfoJSONSerialization(t *testing.T) {
	backendInfo := &ServerlessBackendInfo{
		Name:    "Knative",
		Version: "1.2.3",
	}

	info := Info{
		Version:               "v3.0.0",
		GitCommit:             "abc123def456",
		Architecture:          "amd64",
		KubeVersion:           "v1.24.0",
		ServerlessBackendInfo: backendInfo,
	}

	// Test JSON marshaling
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal Info: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled Info
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal Info: %v", err)
	}

	if unmarshaled.Version != info.Version {
		t.Errorf("Expected Version %s, got %s", info.Version, unmarshaled.Version)
	}

	if unmarshaled.GitCommit != info.GitCommit {
		t.Errorf("Expected GitCommit %s, got %s", info.GitCommit, unmarshaled.GitCommit)
	}

	if unmarshaled.Architecture != info.Architecture {
		t.Errorf("Expected Architecture %s, got %s", info.Architecture, unmarshaled.Architecture)
	}

	if unmarshaled.KubeVersion != info.KubeVersion {
		t.Errorf("Expected KubeVersion %s, got %s", info.KubeVersion, unmarshaled.KubeVersion)
	}

	if unmarshaled.ServerlessBackendInfo == nil {
		t.Error("Expected ServerlessBackendInfo to be set")
	} else {
		if unmarshaled.ServerlessBackendInfo.Name != backendInfo.Name {
			t.Errorf("Expected Backend Name %s, got %s", backendInfo.Name, unmarshaled.ServerlessBackendInfo.Name)
		}

		if unmarshaled.ServerlessBackendInfo.Version != backendInfo.Version {
			t.Errorf("Expected Backend Version %s, got %s", backendInfo.Version, unmarshaled.ServerlessBackendInfo.Version)
		}
	}
}

func TestInfoWithoutBackend(t *testing.T) {
	info := Info{
		Version:      "v3.0.0",
		GitCommit:    "abc123def456",
		Architecture: "amd64",
		KubeVersion:  "v1.24.0",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal Info without backend: %v", err)
	}

	var unmarshaled Info
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal Info without backend: %v", err)
	}

	if unmarshaled.ServerlessBackendInfo != nil {
		t.Error("Expected ServerlessBackendInfo to be nil")
	}

	// Other fields should be preserved
	if unmarshaled.Version != info.Version {
		t.Errorf("Expected Version %s, got %s", info.Version, unmarshaled.Version)
	}
}

func TestInfoEmptyFields(t *testing.T) {
	info := Info{}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal empty Info: %v", err)
	}

	var unmarshaled Info
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal empty Info: %v", err)
	}

	if unmarshaled.Version != "" {
		t.Errorf("Expected empty Version, got '%s'", unmarshaled.Version)
	}

	if unmarshaled.GitCommit != "" {
		t.Errorf("Expected empty GitCommit, got '%s'", unmarshaled.GitCommit)
	}

	if unmarshaled.Architecture != "" {
		t.Errorf("Expected empty Architecture, got '%s'", unmarshaled.Architecture)
	}

	if unmarshaled.KubeVersion != "" {
		t.Errorf("Expected empty KubeVersion, got '%s'", unmarshaled.KubeVersion)
	}

	if unmarshaled.ServerlessBackendInfo != nil {
		t.Error("Expected ServerlessBackendInfo to be nil")
	}
}

func TestServerlessBackendInfoJSONSerialization(t *testing.T) {
	backend := ServerlessBackendInfo{
		Name:    "Knative",
		Version: "1.2.3",
	}

	data, err := json.Marshal(backend)
	if err != nil {
		t.Fatalf("Failed to marshal ServerlessBackendInfo: %v", err)
	}

	var unmarshaled ServerlessBackendInfo
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal ServerlessBackendInfo: %v", err)
	}

	if unmarshaled.Name != backend.Name {
		t.Errorf("Expected Name %s, got %s", backend.Name, unmarshaled.Name)
	}

	if unmarshaled.Version != backend.Version {
		t.Errorf("Expected Version %s, got %s", backend.Version, unmarshaled.Version)
	}
}

func TestInfoJSONTags(t *testing.T) {
	backendInfo := &ServerlessBackendInfo{
		Name:    "Knative",
		Version: "1.2.3",
	}

	info := Info{
		Version:               "v3.0.0",
		GitCommit:             "abc123def456",
		Architecture:          "amd64",
		KubeVersion:           "v1.24.0",
		ServerlessBackendInfo: backendInfo,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal Info: %v", err)
	}

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	if err != nil {
		t.Fatalf("Failed to unmarshal to raw map: %v", err)
	}

	// Check that JSON field names match the tags
	expectedFields := []string{
		"version", "git_commit", "architecture", "kubernetes_version", "serverless_backend",
	}

	for _, field := range expectedFields {
		if _, exists := raw[field]; !exists {
			t.Errorf("Expected '%s' field in JSON", field)
		}
	}

	// Check nested object
	if backend, exists := raw["serverless_backend"]; exists {
		if backendMap, ok := backend.(map[string]interface{}); ok {
			if _, exists := backendMap["name"]; !exists {
				t.Error("Expected 'name' field in serverless_backend")
			}
			if _, exists := backendMap["version"]; !exists {
				t.Error("Expected 'version' field in serverless_backend")
			}
		} else {
			t.Error("Expected serverless_backend to be an object")
		}
	} else {
		t.Error("Expected 'serverless_backend' field in JSON")
	}
}

func TestServerlessBackendInfoJSONTags(t *testing.T) {
	backend := ServerlessBackendInfo{
		Name:    "Knative",
		Version: "1.2.3",
	}

	data, err := json.Marshal(backend)
	if err != nil {
		t.Fatalf("Failed to marshal ServerlessBackendInfo: %v", err)
	}

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	if err != nil {
		t.Fatalf("Failed to unmarshal to raw map: %v", err)
	}

	// Check that JSON field names match the tags
	if _, exists := raw["name"]; !exists {
		t.Error("Expected 'name' field in JSON")
	}

	if _, exists := raw["version"]; !exists {
		t.Error("Expected 'version' field in JSON")
	}
}

func TestInfoDifferentBackends(t *testing.T) {
	tests := []struct {
		name    string
		backend *ServerlessBackendInfo
	}{
		{
			name: "knative backend",
			backend: &ServerlessBackendInfo{
				Name:    "Knative",
				Version: "1.2.3",
			},
		},
		{
			name: "openfaas backend",
			backend: &ServerlessBackendInfo{
				Name:    "OpenFaaS",
				Version: "0.20.0",
			},
		},
		{
			name:    "no backend",
			backend: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := Info{
				Version:               "v3.0.0",
				GitCommit:             "abc123def456",
				Architecture:          "amd64",
				KubeVersion:           "v1.24.0",
				ServerlessBackendInfo: tt.backend,
			}

			data, err := json.Marshal(info)
			if err != nil {
				t.Fatalf("Failed to marshal Info with %s: %v", tt.name, err)
			}

			var unmarshaled Info
			err = json.Unmarshal(data, &unmarshaled)
			if err != nil {
				t.Fatalf("Failed to unmarshal Info with %s: %v", tt.name, err)
			}

			if tt.backend == nil {
				if unmarshaled.ServerlessBackendInfo != nil {
					t.Error("Expected ServerlessBackendInfo to be nil")
				}
			} else {
				if unmarshaled.ServerlessBackendInfo == nil {
					t.Error("Expected ServerlessBackendInfo to be set")
				} else {
					if unmarshaled.ServerlessBackendInfo.Name != tt.backend.Name {
						t.Errorf("Expected Backend Name %s, got %s", tt.backend.Name, unmarshaled.ServerlessBackendInfo.Name)
					}

					if unmarshaled.ServerlessBackendInfo.Version != tt.backend.Version {
						t.Errorf("Expected Backend Version %s, got %s", tt.backend.Version, unmarshaled.ServerlessBackendInfo.Version)
					}
				}
			}
		})
	}
}

func TestInfoPartialFields(t *testing.T) {
	tests := []struct {
		name string
		info Info
	}{
		{
			name: "only version",
			info: Info{
				Version: "v3.0.0",
			},
		},
		{
			name: "version and git commit",
			info: Info{
				Version:   "v3.0.0",
				GitCommit: "abc123",
			},
		},
		{
			name: "all fields except backend",
			info: Info{
				Version:      "v3.0.0",
				GitCommit:    "abc123def456",
				Architecture: "amd64",
				KubeVersion:  "v1.24.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.info)
			if err != nil {
				t.Fatalf("Failed to marshal partial Info: %v", err)
			}

			var unmarshaled Info
			err = json.Unmarshal(data, &unmarshaled)
			if err != nil {
				t.Fatalf("Failed to unmarshal partial Info: %v", err)
			}

			if unmarshaled.Version != tt.info.Version {
				t.Errorf("Expected Version %s, got %s", tt.info.Version, unmarshaled.Version)
			}

			if unmarshaled.GitCommit != tt.info.GitCommit {
				t.Errorf("Expected GitCommit %s, got %s", tt.info.GitCommit, unmarshaled.GitCommit)
			}

			if unmarshaled.Architecture != tt.info.Architecture {
				t.Errorf("Expected Architecture %s, got %s", tt.info.Architecture, unmarshaled.Architecture)
			}

			if unmarshaled.KubeVersion != tt.info.KubeVersion {
				t.Errorf("Expected KubeVersion %s, got %s", tt.info.KubeVersion, unmarshaled.KubeVersion)
			}

			if unmarshaled.ServerlessBackendInfo != nil {
				t.Error("Expected ServerlessBackendInfo to be nil")
			}
		})
	}
}
