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
	"strings"
	"testing"

	"github.com/grycap/oscar/v4/pkg/types"
)

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
