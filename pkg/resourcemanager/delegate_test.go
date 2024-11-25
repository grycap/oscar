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

package resourcemanager

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grycap/oscar/v3/pkg/types"
)

func TestDelegateJob(t *testing.T) {
	logger := log.New(bytes.NewBuffer([]byte{}), "", log.LstdFlags)
	event := "test-event"

	// Mock server to simulate the cluster endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/job/test-service" {
			w.WriteHeader(http.StatusCreated)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/system/services/test-service" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(&types.Service{Token: "test-token"})
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/system/status" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(&GeneralInfo{
				CPUMaxFree:   1000,
				CPUFreeTotal: 2000,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	service := &types.Service{
		Name:       "test-service",
		ClusterID:  "test-cluster",
		CPU:        "1",
		Delegation: "static",
		Replicas: []types.Replica{
			{
				Type:        "oscar",
				ClusterID:   "test-cluster",
				ServiceName: "test-service",
				Priority:    50,
				Headers:     map[string]string{"Content-Type": "application/json"},
			},
		},
		Clusters: map[string]types.Cluster{
			"test-cluster": {
				Endpoint:     server.URL,
				AuthUser:     "user",
				AuthPassword: "password",
				SSLVerify:    false,
			},
		},
	}

	t.Run("Replica type oscar", func(t *testing.T) {
		err := DelegateJob(service, event, logger)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})

	t.Run("Replica type oscar with delegation random", func(t *testing.T) {
		service.Delegation = "random"
		err := DelegateJob(service, event, logger)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})

	t.Run("Replica type oscar with delegation load-based", func(t *testing.T) {
		service.Delegation = "load-based"
		err := DelegateJob(service, event, logger)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})

	t.Run("Replica type endpoint", func(t *testing.T) {
		service.Replicas[0].Type = "endpoint"
		service.Replicas[0].URL = server.URL
		err := DelegateJob(service, event, logger)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})
}

func TestWrapEvent(t *testing.T) {
	providerID := "test-provider"
	event := "test-event"

	expected := DelegatedEvent{
		StorageProviderID: providerID,
		Event:             event,
	}

	result := WrapEvent(providerID, event)

	if result != expected {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestGetServiceToken(t *testing.T) {
	replica := types.Replica{
		ServiceName: "test-service",
	}
	cluster := types.Cluster{
		Endpoint:     "http://localhost:8080",
		AuthUser:     "user",
		AuthPassword: "password",
		SSLVerify:    false,
	}

	// Mock server to simulate the cluster endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/system/services/test-service" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(&types.Service{Token: "test-token"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Update the cluster endpoint to the mock server URL
	cluster.Endpoint = server.URL

	token, err := getServiceToken(replica, cluster)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expectedToken := "test-token"
	if token != expectedToken {
		t.Errorf("Expected %v, got %v", expectedToken, token)
	}
}

func TestUpdateServiceToken(t *testing.T) {
	replica := types.Replica{
		ServiceName: "test-service",
	}
	cluster := types.Cluster{
		Endpoint:     "http://localhost:8080",
		AuthUser:     "user",
		AuthPassword: "password",
		SSLVerify:    false,
	}

	// Mock server to simulate the cluster endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/system/services/test-service" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(&types.Service{Token: "test-token"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Update the cluster endpoint to the mock server URL
	cluster.Endpoint = server.URL

	token, err := updateServiceToken(replica, cluster)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expectedToken := "test-token"
	if token != expectedToken {
		t.Errorf("Expected %v, got %v", expectedToken, token)
	}
}
