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
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grycap/oscar/v3/pkg/testsupport"
	"github.com/grycap/oscar/v3/pkg/types"
)

func TestDelegateJob(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

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
		err := DelegateJob(service, event, "", logger)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})

	t.Run("Replica type oscar with delegation random", func(t *testing.T) {
		service.Delegation = "random"
		err := DelegateJob(service, event, "", logger)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})

	t.Run("Replica type oscar with delegation load-based", func(t *testing.T) {
		service.Delegation = "load-based"
		err := DelegateJob(service, event, "", logger)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})

	t.Run("Replica type endpoint", func(t *testing.T) {
		service.Replicas[0].Type = "endpoint"
		service.Replicas[0].URL = server.URL
		err := DelegateJob(service, event, "", logger)
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
	testsupport.SkipIfCannotListen(t)

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
	testsupport.SkipIfCannotListen(t)

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

func TestWeightMatrix(t *testing.T) {
	matrix := [][]float64{{1, 2}, {3, 4}}
	weight := []float64{0.5, 0.25}
	weighted := weightMatrix(matrix, weight)
	expected := [][]float64{{0.5, 0.5}, {1.5, 1.0}}
	for i := range expected {
		for j := range expected[i] {
			if math.Abs(weighted[i][j]-expected[i][j]) > 1e-9 {
				t.Fatalf("unexpected weight at (%d,%d): got %f, want %f", i, j, weighted[i][j], expected[i][j])
			}
		}
	}
}

func TestMapToRange(t *testing.T) {
	if v := mapToRange(50, 0, 100, 100, 0); v != 50 {
		t.Fatalf("expected 50, got %d", v)
	}
	if v := mapToRange(-10, 0, 100, 100, 0); v != 100 {
		t.Fatalf("expected clamp to 100, got %d", v)
	}
	if v := mapToRange(120, 0, 100, 100, 0); v != 0 {
		t.Fatalf("expected clamp to 0, got %d", v)
	}
}

func TestTopsisMethod(t *testing.T) {
	results := [][]float64{
		{10, 9, 8, 7, 1, 2},
		{9, 7, 6, 5, 2, 3},
		{5, 4, 7, 8, 3, 4},
	}
	weight := []float64{0.2, 0.25, 0.2, 0.15, 0.1, 0.1}
	prefs := topsisMethod(results, weight)
	if len(prefs) != len(results) {
		t.Fatalf("expected %d preferences, got %d", len(results), len(prefs))
	}
	if prefs[0] < prefs[1] {
		t.Fatalf("expected first alternative to rank higher; prefs=%v", prefs)
	}
}

func TestSortByThreshold(t *testing.T) {
	rand.Seed(0)
	preferences := []float64{0.9, 0.86, 0.2}
	sorted := sortbyThreshold(preferences, 5)
	if len(sorted) != len(preferences) {
		t.Fatalf("expected %d alternatives, got %d", len(preferences), len(sorted))
	}
	for _, alt := range sorted {
		if alt.Preference < 0 || alt.Preference > 100 {
			t.Fatalf("preference not mapped to range: %v", alt)
		}
	}
}

func TestCountJobs(t *testing.T) {
	now := time.Now().UTC()
	jobs := map[string]JobStatus{
		"a": {Status: "Succeeded", CreationTime: now.Add(-2 * time.Minute).Format(time.RFC3339), FinishTime: now.Format(time.RFC3339)},
		"b": {Status: "Failed"},
		"c": {Status: "Pending"},
	}
	avg, pending := countJobs(jobs)
	if pending != 1 {
		t.Fatalf("expected one pending job, got %d", pending)
	}
	if avg <= 0 || avg > 120 {
		t.Fatalf("unexpected average execution time: %f", avg)
	}
}

func TestCreateParameters(t *testing.T) {
	var results [][]float64
	duration := 2 * time.Minute
	cluster := GeneralInfo{CPUMaxFree: 4000, CPUFreeTotal: 8000, MemoryMaxFree: 16 * 1024 * 1024 * 1024, MemoryFreeTotal: 32 * 1024 * 1024 * 1024}
	params := createParameters(results, duration, cluster, 1.0, 30.0, 2)
	if len(params) != 1 {
		t.Fatalf("expected single parameter slice, got %d", len(params))
	}
	if len(params[0]) != 6 {
		t.Fatalf("expected six metrics, got %d", len(params[0]))
	}
}

func TestNormalizeMatrix(t *testing.T) {
	matrix := [][]float64{{3, 4}, {0, 5}}
	normalized := normalizeMatrix(matrix)
	for j := 0; j < len(matrix[0]); j++ {
		sum := 0.0
		for i := 0; i < len(matrix); i++ {
			sum += normalized[i][j] * normalized[i][j]
		}
		if math.Abs(sum-1.0) > 1e-9 {
			t.Fatalf("expected column %d to be normalized, got %f", j, sum)
		}
	}
}

func TestCalculateSolutions(t *testing.T) {
	matrix := [][]float64{{1, 9}, {2, 8}, {3, 7}}
	ideal, anti := calculateSolutions(matrix)
	if ideal[0] != 1 || anti[0] != 3 {
		t.Fatalf("unexpected solutions for minimization criterion: ideal=%v anti=%v", ideal, anti)
	}
	if ideal[1] != 9 || anti[1] != 7 {
		t.Fatalf("unexpected solutions for maximization criterion: ideal=%v anti=%v", ideal, anti)
	}
}

func TestCalculatePreferences(t *testing.T) {
	matrix := [][]float64{{0.9, 0.9}, {0.1, 0.1}}
	ideal := []float64{1, 1}
	anti := []float64{0, 0}
	prefs := calculatePreferences(matrix, ideal, anti)
	if prefs[0] <= prefs[1] {
		t.Fatalf("expected first alternative to have higher preference, got %v", prefs)
	}
}

func TestReorganizeIfNearby(t *testing.T) {
	rand.Seed(1)
	alternatives := []Alternative{{Index: 1, Preference: 0.9}, {Index: 2, Preference: 0.88}, {Index: 3, Preference: 0.5}}
	dists := []float64{0.02, 0.4}
	threshold := 0.05
	reordered := reorganizeIfNearby(alternatives, dists, threshold)
	if len(reordered) != len(alternatives) {
		t.Fatalf("expected %d alternatives, got %d", len(alternatives), len(reordered))
	}
	indices := map[int]bool{}
	for _, alt := range reordered {
		indices[alt.Index] = true
	}
	for _, alt := range alternatives {
		if !indices[alt.Index] {
			t.Fatalf("alternative %d missing after reordering", alt.Index)
		}
	}
}

func TestEventBuild(t *testing.T) {
	raw := `{"event":"payload","storage_provider":"s3"}`
	eventJSON, storage := eventBuild(raw, "minio")
	if storage != "s3" {
		t.Fatalf("expected storage from event, got %s", storage)
	}
	if string(eventJSON) == "" {
		t.Fatalf("expected delegated event to be returned")
	}

	eventJSON, storage = eventBuild(`{"event":"payload"}`, "onedata")
	if storage != "onedata" {
		t.Fatalf("expected fallback storage provider, got %s", storage)
	}
	if string(eventJSON) == "" {
		t.Fatalf("expected delegated event for fallback storage")
	}
}

func TestCountJobsAggregation(t *testing.T) {
	now := time.Now()
	jobStatuses := map[string]JobStatus{
		"a": {Status: "Succeeded", CreationTime: now.Add(-2 * time.Minute).Format(time.RFC3339), FinishTime: now.Format(time.RFC3339)},
		"b": {Status: "Failed"},
		"c": {Status: "Pending"},
	}

	avg, pending := countJobs(jobStatuses)
	if pending != 1 {
		t.Fatalf("expected 1 pending job, got %d", pending)
	}
	if avg <= 0 {
		t.Fatalf("expected average execution time to be > 0")
	}
}

func TestCreateParametersConstraints(t *testing.T) {
	results := createParameters(nil, 5*time.Second, GeneralInfo{CPUMaxFree: 2000, NumberNodes: 2, MemoryFreeTotal: 1024, CPUFreeTotal: 4000}, 0.5, 10, 0)
	if len(results) == 0 || len(results[0]) != 6 {
		t.Fatalf("expected populated parameter slice, got %v", results)
	}

	results = createParameters(nil, 5*time.Second, GeneralInfo{CPUMaxFree: 100, NumberNodes: 2, MemoryFreeTotal: 1024, CPUFreeTotal: 4000}, 2.0, 10, 0)
	if results[0][1] != 0 {
		t.Fatalf("expected zeroed values when insufficient CPU, got %v", results)
	}
}
