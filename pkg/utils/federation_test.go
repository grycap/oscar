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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grycap/oscar/v4/pkg/types"
)

func TestUpsertFederationCreatesMissingMemberPreservingServiceConfiguration(t *testing.T) {
	putRequests := 0
	postRequests := 0
	var deployed types.Service
	expectedHost := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != expectedHost {
			t.Errorf("expected endpoint host %q to be preserved, got %q", expectedHost, r.Host)
		}
		if r.URL.Path != "/system/services" {
			t.Errorf("expected request path /system/services, got %q", r.URL.Path)
		}
		switch r.Method {
		case http.MethodPut:
			putRequests++
			http.NotFound(w, r)
		case http.MethodPost:
			postRequests++
			if err := json.NewDecoder(r.Body).Decode(&deployed); err != nil {
				t.Errorf("failed to decode deployed service: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	expectedHost = strings.TrimPrefix(server.URL, "http://")

	service := &types.Service{
		Name:      "coordinator",
		ClusterID: "local",
		CPU:       "0.2",
		Memory:    "256Mi",
		Input: []types.StorageIOConfig{
			{Provider: "minio.default", Path: "coordinator/input"},
			{Provider: "webdav.external", Path: "coordinator/shared"},
		},
		Output: []types.StorageIOConfig{
			{Provider: "s3.archive", Path: "coordinator/output"},
		},
		Federation: &types.Federation{
			Topology:   "star",
			Delegation: "static",
			Members: types.ReplicaList{
				{
					Type:        "oscar",
					ClusterID:   "local",
					ServiceName: "coordinator-replica",
				},
			},
		},
		Clusters: map[string]types.Cluster{
			"local": {Endpoint: server.URL, SSLVerify: true},
		},
	}

	if errs := UpsertFederation(service, "", ""); len(errs) > 0 {
		t.Fatalf("expected successful upsert, got %v", errs)
	}
	if putRequests != 1 || postRequests != 1 {
		t.Fatalf("expected one PUT and one POST, got PUT=%d POST=%d", putRequests, postRequests)
	}
	if deployed.Name != "coordinator-replica" {
		t.Errorf("expected replica service name, got %q", deployed.Name)
	}
	if deployed.CPU != service.CPU || deployed.Memory != service.Memory {
		t.Errorf("expected replica to inherit cpu=%q memory=%q, got cpu=%q memory=%q", service.CPU, service.Memory, deployed.CPU, deployed.Memory)
	}
	if deployed.Input[0].Path != "coordinator/input" {
		t.Errorf("expected default MinIO input to keep the origin bucket, got %q", deployed.Input[0].Path)
	}
	if deployed.Output[0].Path != "coordinator/output" {
		t.Errorf("expected S3 output path to remain shared, got %q", deployed.Output[0].Path)
	}
	if deployed.Input[1].Path != "coordinator/shared" {
		t.Errorf("expected non-bucket storage path to remain unchanged, got %q", deployed.Input[1].Path)
	}
}

func TestUpsertFederationDoesNotCreateMemberAfterUpdateFailure(t *testing.T) {
	putRequests := 0
	postRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			putRequests++
			http.Error(w, "remote update failed", http.StatusInternalServerError)
		case http.MethodPost:
			postRequests++
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer server.Close()

	service := &types.Service{
		Name: "coordinator",
		Federation: &types.Federation{
			Topology: "star",
			Members: types.ReplicaList{
				{Type: "oscar", ClusterID: "target", ServiceName: "replica"},
			},
		},
		Clusters: map[string]types.Cluster{
			"target": {Endpoint: server.URL, SSLVerify: true},
		},
	}

	errs := UpsertFederation(service, "", "")
	if len(errs) != 1 {
		t.Fatalf("expected one propagation error, got %v", errs)
	}
	if !strings.Contains(errs[0].Error(), "status 500") {
		t.Fatalf("expected remote status in error, got %v", errs[0])
	}
	if putRequests != 1 || postRequests != 0 {
		t.Fatalf("expected one PUT and no POST, got PUT=%d POST=%d", putRequests, postRequests)
	}
}

func TestUpsertFederationUpdatesExistingMember(t *testing.T) {
	putRequests := 0
	postRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			putRequests++
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPost:
			postRequests++
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer server.Close()

	service := &types.Service{
		Federation: &types.Federation{
			Members: types.ReplicaList{
				{Type: "oscar", ClusterID: "target", ServiceName: "replica"},
			},
		},
		Clusters: map[string]types.Cluster{
			"target": {Endpoint: server.URL, SSLVerify: true},
		},
	}

	if errs := UpsertFederation(service, "", ""); len(errs) > 0 {
		t.Fatalf("expected successful upsert, got %v", errs)
	}
	if putRequests != 1 || postRequests != 0 {
		t.Fatalf("expected one PUT and no POST, got PUT=%d POST=%d", putRequests, postRequests)
	}
}

func TestExpandFederation(t *testing.T) {
	t.Run("nil service", func(t *testing.T) {
		if errs := ExpandFederation(nil, "", "", ""); errs != nil {
			t.Errorf("expected nil, got %v", errs)
		}
	})

	t.Run("nil Federation", func(t *testing.T) {
		svc := &types.Service{Name: "test"}
		if errs := ExpandFederation(svc, "", "", ""); errs != nil {
			t.Errorf("expected nil, got %v", errs)
		}
	})

	t.Run("empty Members", func(t *testing.T) {
		svc := &types.Service{
			Name:       "test",
			Federation: &types.Federation{},
		}
		if errs := ExpandFederation(svc, "", "", ""); errs != nil {
			t.Errorf("expected nil, got %v", errs)
		}
	})

	t.Run("undefined cluster", func(t *testing.T) {
		svc := &types.Service{
			Name: "test-svc",
			Federation: &types.Federation{
				Members: types.ReplicaList{
					{Type: "oscar", ClusterID: "nonexistent", ServiceName: "replica-svc"},
				},
			},
			Clusters: map[string]types.Cluster{},
		}
		errs := ExpandFederation(svc, "", "", "")
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
		if !strings.Contains(errs[0].Error(), `"nonexistent"`) {
			t.Errorf("expected error about cluster nonexistent, got %v", errs[0])
		}
	})
}

func TestVerifyFederationAuth(t *testing.T) {
	t.Run("nil service", func(t *testing.T) {
		if errs := VerifyFederationAuth(nil, ""); errs != nil {
			t.Errorf("expected nil, got %v", errs)
		}
	})

	t.Run("nil Federation", func(t *testing.T) {
		svc := &types.Service{Name: "test"}
		if errs := VerifyFederationAuth(svc, ""); errs != nil {
			t.Errorf("expected nil, got %v", errs)
		}
	})

	t.Run("empty Members", func(t *testing.T) {
		svc := &types.Service{
			Name:       "test",
			Federation: &types.Federation{},
		}
		if errs := VerifyFederationAuth(svc, ""); errs != nil {
			t.Errorf("expected nil, got %v", errs)
		}
	})

	t.Run("non-oscar member skipped", func(t *testing.T) {
		svc := &types.Service{
			Name: "test",
			Federation: &types.Federation{
				Members: types.ReplicaList{
					{Type: "endpoint", ClusterID: "some-cluster", ServiceName: "svc"},
				},
			},
		}
		if errs := VerifyFederationAuth(svc, ""); errs != nil {
			t.Errorf("expected nil, got %v", errs)
		}
	})

	t.Run("oscar member undefined cluster", func(t *testing.T) {
		svc := &types.Service{
			Name: "test",
			Federation: &types.Federation{
				Members: types.ReplicaList{
					{Type: "oscar", ClusterID: "missing", ServiceName: "replica-svc"},
				},
			},
			Clusters: map[string]types.Cluster{},
		}
		errs := VerifyFederationAuth(svc, "")
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
		if !strings.Contains(errs[0].Error(), `"missing"`) {
			t.Errorf("expected error about cluster missing, got %v", errs[0])
		}
	})
}

func TestRollbackFederationCreate(t *testing.T) {
	t.Run("nil service", func(t *testing.T) {
		if errs := RollbackFederationCreate(nil, ""); errs != nil {
			t.Errorf("expected nil, got %v", errs)
		}
	})

	t.Run("nil Federation", func(t *testing.T) {
		svc := &types.Service{Name: "test"}
		if errs := RollbackFederationCreate(svc, ""); errs != nil {
			t.Errorf("expected nil, got %v", errs)
		}
	})

	t.Run("empty Members", func(t *testing.T) {
		svc := &types.Service{
			Name:       "test",
			Federation: &types.Federation{},
		}
		if errs := RollbackFederationCreate(svc, ""); errs != nil {
			t.Errorf("expected nil, got %v", errs)
		}
	})

	t.Run("non-oscar member skipped", func(t *testing.T) {
		svc := &types.Service{
			Name: "test",
			Federation: &types.Federation{
				Members: types.ReplicaList{
					{Type: "endpoint", ClusterID: "c1"},
				},
			},
		}
		if errs := RollbackFederationCreate(svc, ""); errs != nil {
			t.Errorf("expected nil, got %v", errs)
		}
	})

	t.Run("oscar member undefined cluster", func(t *testing.T) {
		svc := &types.Service{
			Name: "test",
			Federation: &types.Federation{
				Members: types.ReplicaList{
					{Type: "oscar", ClusterID: "ghost", ServiceName: "r"},
				},
			},
			Clusters: map[string]types.Cluster{},
		}
		errs := RollbackFederationCreate(svc, "")
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
		if !strings.Contains(errs[0].Error(), `"ghost"`) {
			t.Errorf("expected error about cluster ghost, got %v", errs[0])
		}
	})
}

func TestApplyFederation(t *testing.T) {
	t.Run("nil service does not panic", func(t *testing.T) {
		ApplyFederation(nil)
	})

	t.Run("nil Federation does not panic", func(t *testing.T) {
		svc := &types.Service{Name: "test"}
		ApplyFederation(svc)
	})

	t.Run("empty GroupID set to service name", func(t *testing.T) {
		svc := &types.Service{
			Name: "my-service",
			Federation: &types.Federation{
				GroupID: "",
			},
		}
		ApplyFederation(svc)
		if svc.Federation.GroupID != "my-service" {
			t.Errorf("expected GroupID to be 'my-service', got %q", svc.Federation.GroupID)
		}
	})

	t.Run("non-empty GroupID preserved", func(t *testing.T) {
		svc := &types.Service{
			Name: "my-service",
			Federation: &types.Federation{
				GroupID: "custom-group",
			},
		}
		ApplyFederation(svc)
		if svc.Federation.GroupID != "custom-group" {
			t.Errorf("expected GroupID to be 'custom-group', got %q", svc.Federation.GroupID)
		}
	})
}
