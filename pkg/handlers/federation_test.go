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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/backends"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFederationPostUpdatesServiceWithoutFederation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	methods := []string{}
	var deployedService types.Service
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if r.Method == http.MethodPut {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&deployedService); err != nil {
			t.Errorf("failed to decode deployed service: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer remote.Close()

	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "oscar-svc-test",
		Clusters: map[string]types.Cluster{
			"cluster-a": {Endpoint: remote.URL, SSLVerify: true},
		},
	}
	kubeClient := back.GetKubeClientset()
	_, _ = kubeClient.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: back.Service.Namespace},
	}, metav1.CreateOptions{})
	_, _ = kubeClient.CoreV1().Secrets(back.Service.Namespace).Create(context.TODO(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RefreshTokenSecretName(back.Service.Name),
			Namespace: back.Service.Namespace,
		},
		Data: map[string][]byte{
			types.RefreshTokenSecretKey: []byte("refresh-token"),
		},
	}, metav1.CreateOptions{})

	r := gin.New()
	r.POST("/system/federation/:serviceName", MakeFederationPostHandler(back))

	body := `{"members":[{"type":"oscar","cluster_id":"cluster-a","service_name":"svc-a"}]}`
	req := httptest.NewRequest(http.MethodPost, "/system/federation/svc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if back.UpdatedService == nil {
		t.Fatalf("expected service update via backend")
	}
	if back.UpdatedService.Federation == nil || len(back.UpdatedService.Federation.Members) != 1 {
		t.Fatalf("expected 1 federation member, got %d", len(back.UpdatedService.Federation.Members))
	}
	if back.UpdatedService.Federation.Topology != "star" || back.UpdatedService.Federation.Delegation != "static" {
		t.Fatalf("expected default star/static federation, got %#v", back.UpdatedService.Federation)
	}
	if len(methods) != 2 || methods[0] != http.MethodPut || methods[1] != http.MethodPost {
		t.Fatalf("expected PUT followed by POST, got %v", methods)
	}
	if deployedService.Name != "svc-a" || deployedService.Federation == nil || deployedService.Federation.Topology != "star" {
		t.Fatalf("expected a star federation worker named svc-a, got %#v", deployedService)
	}
}

func TestFederationPostRequiresRefreshTokenForStarFederation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "oscar-svc-test",
		Clusters: map[string]types.Cluster{
			"cluster-a": {Endpoint: "https://oscar.example.org", SSLVerify: true},
		},
	}

	r := gin.New()
	r.POST("/system/federation/:serviceName", MakeFederationPostHandler(back))
	req := httptest.NewRequest(http.MethodPost, "/system/federation/svc", strings.NewReader(`{"members":[{"type":"oscar","cluster_id":"cluster-a","service_name":"svc-a"}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
	if back.UpdatedService != nil {
		t.Fatal("service must not be updated without a refresh token")
	}
}

func TestFederationPostStoresRefreshTokenFromRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer remote.Close()

	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "oscar-svc-test",
		Clusters: map[string]types.Cluster{
			"cluster-a": {Endpoint: remote.URL, SSLVerify: true},
		},
	}

	r := gin.New()
	r.POST("/system/federation/:serviceName", MakeFederationPostHandler(back))
	body := `{"members":[{"type":"oscar","cluster_id":"cluster-a","service_name":"svc-a"}],"refresh_token":"request-refresh-token"}`
	req := httptest.NewRequest(http.MethodPost, "/system/federation/svc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	secret, err := back.GetKubeClientset().CoreV1().Secrets(back.Service.Namespace).Get(
		context.TODO(), utils.RefreshTokenSecretName(back.Service.Name), metav1.GetOptions{},
	)
	if err != nil {
		t.Fatalf("expected refresh-token secret to be created: %v", err)
	}
	if got := secret.StringData[types.RefreshTokenSecretKey]; got != "request-refresh-token" {
		t.Fatalf("expected request refresh token to be stored, got %q", got)
	}
}

func TestFederationPostDoesNotDuplicateExistingMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer remote.Close()

	member := types.Replica{Type: "oscar", ClusterID: "cluster-a", ServiceName: "svc-a"}
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "oscar-svc-test",
		Federation: &types.Federation{
			Topology:   "star",
			Delegation: "static",
			Members:    types.ReplicaList{member},
		},
		Clusters: map[string]types.Cluster{
			"cluster-a": {Endpoint: remote.URL, SSLVerify: true},
		},
	}
	_, _ = back.GetKubeClientset().CoreV1().Secrets(back.Service.Namespace).Create(
		context.TODO(),
		&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      utils.RefreshTokenSecretName(back.Service.Name),
				Namespace: back.Service.Namespace,
			},
			Data: map[string][]byte{types.RefreshTokenSecretKey: []byte("refresh-token")},
		},
		metav1.CreateOptions{},
	)

	r := gin.New()
	r.POST("/system/federation/:serviceName", MakeFederationPostHandler(back))
	body := `{"members":[{"type":"oscar","cluster_id":"cluster-a","service_name":"svc-a"}]}`
	req := httptest.NewRequest(http.MethodPost, "/system/federation/svc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := len(back.UpdatedService.Federation.Members); got != 1 {
		t.Fatalf("expected duplicate member to be ignored, got %d members", got)
	}
}

func TestFederationPostReturnsErrorWhenRemoteDeploymentFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "remote failure", http.StatusInternalServerError)
	}))
	defer remote.Close()

	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "oscar-svc-test",
		Clusters: map[string]types.Cluster{
			"cluster-a": {Endpoint: remote.URL, SSLVerify: true},
		},
	}

	r := gin.New()
	r.POST("/system/federation/:serviceName", MakeFederationPostHandler(back))
	body := `{"members":[{"type":"oscar","cluster_id":"cluster-a","service_name":"svc-a"}],"refresh_token":"request-refresh-token"}`
	req := httptest.NewRequest(http.MethodPost, "/system/federation/svc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d: %s", w.Code, w.Body.String())
	}
	if back.UpdatedService == nil || len(back.UpdatedService.Federation.Members) != 1 {
		t.Fatal("expected the local federation state to retain the failed member")
	}
}

func TestFederationPostRejectsBearerForServiceOwnedByAnotherUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "oscar-svc-test",
		Owner:     "owner@example.org",
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "other@example.org")
	})
	r.POST("/system/federation/:serviceName", MakeFederationPostHandler(back))
	req := httptest.NewRequest(http.MethodPost, "/system/federation/svc", strings.NewReader(`{"members":[]}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
	if back.UpdatedService != nil {
		t.Fatal("service owned by another user must not be updated")
	}
}

func TestMakeFederationGetHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "oscar-svc-test",
		Federation: &types.Federation{
			Topology:   "star",
			Delegation: "load-based",
			Members: types.ReplicaList{
				{Type: "oscar", ClusterID: "cluster-a", ServiceName: "svc-a"},
			},
		},
	}

	r := gin.New()
	r.GET("/system/federation/:serviceName", MakeFederationGetHandler(back))

	req := httptest.NewRequest(http.MethodGet, "/system/federation/svc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp types.FederationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Topology != "star" {
		t.Errorf("expected topology 'star', got %q", resp.Topology)
	}
	if resp.Delegation != "load-based" {
		t.Errorf("expected delegation 'load-based', got %q", resp.Delegation)
	}
	if len(resp.Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(resp.Members))
	}
	if resp.Members[0].ServiceName != "svc-a" {
		t.Errorf("expected member service name 'svc-a', got %q", resp.Members[0].ServiceName)
	}
}

func TestMakeFederationGetHandlerNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	back := backends.MakeFakeBackend()
	back.AddError("ReadService", k8serr.NewGone("Not Found"))

	r := gin.New()
	r.GET("/system/federation/:serviceName", MakeFederationGetHandler(back))

	req := httptest.NewRequest(http.MethodGet, "/system/federation/svc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestMakeFederationPutHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer remote.Close()

	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "oscar-svc-test",
		Federation: &types.Federation{
			Members: types.ReplicaList{
				{Type: "oscar", ClusterID: "cluster-a", ServiceName: "svc-a"},
			},
		},
		Clusters: map[string]types.Cluster{
			"cluster-a": {Endpoint: remote.URL, SSLVerify: true},
		},
	}

	kubeClient := back.GetKubeClientset()
	_, _ = kubeClient.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: back.Service.Namespace},
	}, metav1.CreateOptions{})
	_, _ = kubeClient.CoreV1().Secrets(back.Service.Namespace).Create(context.TODO(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RefreshTokenSecretName(back.Service.Name),
			Namespace: back.Service.Namespace,
		},
		Data: map[string][]byte{
			types.RefreshTokenSecretKey: []byte("refresh-token"),
		},
	}, metav1.CreateOptions{})

	r := gin.New()
	r.PUT("/system/federation/:serviceName", MakeFederationPutHandler(back))

	body := `{"members":[{"type":"oscar","cluster_id":"cluster-a","service_name":"svc-a"}],"update":[{"type":"oscar","cluster_id":"cluster-a","service_name":"svc-a-updated"}],"delegation":"random","topology":"none"}`
	req := httptest.NewRequest(http.MethodPut, "/system/federation/svc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if back.UpdatedService == nil {
		t.Fatalf("expected service update via backend")
	}
	if back.UpdatedService.Federation == nil || len(back.UpdatedService.Federation.Members) != 1 {
		t.Fatalf("expected 1 federation member, got %d", len(back.UpdatedService.Federation.Members))
	}
	if back.UpdatedService.Federation.Members[0].ServiceName != "svc-a-updated" {
		t.Errorf("expected updated service name 'svc-a-updated', got %q", back.UpdatedService.Federation.Members[0].ServiceName)
	}
	if back.UpdatedService.Federation.Delegation != "random" {
		t.Errorf("expected delegation random, got %q", back.UpdatedService.Federation.Delegation)
	}
	if back.UpdatedService.Federation.Topology != "none" {
		t.Errorf("expected topology none, got %q", back.UpdatedService.Federation.Topology)
	}
}

func TestMakeFederationPutHandlerRejectsUnsupportedDelegation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{Name: "svc", Namespace: "oscar-svc-test"}

	r := gin.New()
	r.PUT("/system/federation/:serviceName", MakeFederationPutHandler(back))
	req := httptest.NewRequest(http.MethodPut, "/system/federation/svc", strings.NewReader(`{"members":[],"delegation":"topsis"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
	if back.UpdatedService != nil {
		t.Fatalf("unsupported delegation must not update the service")
	}
}

func TestMakeFederationPutHandlerRejectsUnsupportedTopology(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{Name: "svc", Namespace: "oscar-svc-test"}

	r := gin.New()
	r.PUT("/system/federation/:serviceName", MakeFederationPutHandler(back))
	req := httptest.NewRequest(http.MethodPut, "/system/federation/svc", strings.NewReader(`{"members":[],"topology":"ring"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
	if back.UpdatedService != nil {
		t.Fatalf("unsupported topology must not update the service")
	}
}

func TestMakeFederationPutHandlerConfiguresMeshCoordinator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{Name: "svc", Namespace: "oscar-svc-test"}

	r := gin.New()
	r.PUT("/system/federation/:serviceName", MakeFederationPutHandler(back))
	body := `{"members":[],"topology":"mesh","cluster_id":"origin","clusters":{"origin":{"endpoint":"https://oscar.example.org","ssl_verify":true}}}`
	req := httptest.NewRequest(http.MethodPut, "/system/federation/svc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if back.UpdatedService == nil || back.UpdatedService.ClusterID != "origin" {
		t.Fatalf("expected coordinator cluster ID to be stored")
	}
	if back.UpdatedService.Federation == nil || back.UpdatedService.Federation.Topology != "mesh" {
		t.Fatalf("expected mesh topology to be stored")
	}
	if back.UpdatedService.Clusters["origin"].Endpoint != "https://oscar.example.org" {
		t.Fatalf("expected coordinator endpoint to be stored")
	}
}

func TestMakeFederationDeleteHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer remote.Close()

	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "oscar-svc-test",
		Federation: &types.Federation{
			Members: types.ReplicaList{
				{Type: "oscar", ClusterID: "cluster-a", ServiceName: "svc-a"},
				{Type: "oscar", ClusterID: "cluster-b", ServiceName: "svc-b"},
			},
		},
		Clusters: map[string]types.Cluster{
			"cluster-b": {Endpoint: remote.URL, SSLVerify: true},
		},
	}

	kubeClient := back.GetKubeClientset()
	_, _ = kubeClient.CoreV1().Namespaces().Create(context.TODO(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: back.Service.Namespace},
	}, metav1.CreateOptions{})
	_, _ = kubeClient.CoreV1().Secrets(back.Service.Namespace).Create(context.TODO(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.RefreshTokenSecretName(back.Service.Name),
			Namespace: back.Service.Namespace,
		},
		Data: map[string][]byte{
			types.RefreshTokenSecretKey: []byte("refresh-token"),
		},
	}, metav1.CreateOptions{})

	r := gin.New()
	r.DELETE("/system/federation/:serviceName", MakeFederationDeleteHandler(back))

	body := `{"members":[{"type":"oscar","cluster_id":"cluster-a","service_name":"svc-a"}],"delete":true}`
	req := httptest.NewRequest(http.MethodDelete, "/system/federation/svc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	if back.UpdatedService == nil {
		t.Fatalf("expected service update via backend")
	}
	if back.UpdatedService.Federation == nil {
		t.Fatalf("expected federation in updated service")
	}
	if len(back.UpdatedService.Federation.Members) != 1 {
		t.Fatalf("expected 1 remaining federation member, got %d", len(back.UpdatedService.Federation.Members))
	}
	if back.UpdatedService.Federation.Members[0].ServiceName != "svc-b" {
		t.Errorf("expected remaining member 'svc-b', got %q", back.UpdatedService.Federation.Members[0].ServiceName)
	}
}
