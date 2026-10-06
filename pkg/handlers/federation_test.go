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
	"k8s.io/client-go/kubernetes/fake"
)

func TestFederationPostUpdatesServiceWithoutFederation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "oscar-svc-test",
		Federation: &types.Federation{
			Members: types.ReplicaList{},
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
}

func TestMakeFederationGetHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "oscar-svc-test",
		Federation: &types.Federation{
			Topology: "star",
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

	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "oscar-svc-test",
		Federation: &types.Federation{
			Members: types.ReplicaList{
				{Type: "oscar", ClusterID: "cluster-a", ServiceName: "svc-a"},
			},
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

	body := `{"members":[{"type":"oscar","cluster_id":"cluster-a","service_name":"svc-a"}],"update":[{"type":"oscar","cluster_id":"cluster-a","service_name":"svc-a-updated"}]}`
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
}

func TestMakeFederationDeleteHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

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

func TestFederationHandlersRequireServiceOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)

	methods := []struct {
		name    string
		method  string
		handler func(types.ServerlessBackend) gin.HandlerFunc
	}{
		{"get", http.MethodGet, MakeFederationGetHandler},
		{"post", http.MethodPost, MakeFederationPostHandler},
		{"put", http.MethodPut, MakeFederationPutHandler},
		{"delete", http.MethodDelete, MakeFederationDeleteHandler},
	}
	identities := []struct {
		name       string
		header     string
		uid        string
		wantStatus int
	}{
		{"owner", "Bearer owner-token", "owner", http.StatusOK},
		{"other tenant", "Bearer other-token", "other", http.StatusForbidden},
		{"missing identity", "Bearer unknown-token", "", http.StatusUnauthorized},
		{"basic admin", "Basic admin-credentials", "", http.StatusOK},
	}

	for _, method := range methods {
		for _, identity := range identities {
			t.Run(method.name+"/"+identity.name, func(t *testing.T) {
				back := backends.MakeFakeBackend()
				back.Service = &types.Service{
					Name: "svc", Namespace: "oscar-svc-test", Owner: "owner",
					Federation: &types.Federation{Topology: "star"},
				}
				r := gin.New()
				r.Use(func(c *gin.Context) {
					if identity.uid != "" {
						c.Set("uidOrigin", identity.uid)
					}
					c.Next()
				})
				r.Handle(method.method, "/system/federation/:serviceName", method.handler(back))

				payload := `{"members":[]}`
				if identity.wantStatus != http.StatusOK {
					payload = `{"members":[{"type":"oscar","cluster_id":"remote","service_name":"victim"}],"delete":true,"clusters":{"remote":{}}}`
				}
				req := httptest.NewRequest(method.method, "/system/federation/svc", strings.NewReader(payload))
				req.Header.Set("Authorization", identity.header)
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				if w.Code != identity.wantStatus {
					t.Fatalf("status = %d, want %d; body: %s", w.Code, identity.wantStatus, w.Body.String())
				}
				if identity.wantStatus != http.StatusOK {
					if back.UpdatedService != nil || back.DeletedService != nil {
						t.Fatal("unauthorized request modified a service")
					}
					if len(back.GetKubeClientset().(*fake.Clientset).Actions()) != 0 {
						t.Fatal("unauthorized request accessed Kubernetes resources")
					}
					if strings.Contains(w.Body.String(), "star") {
						t.Fatal("unauthorized response exposed federation topology")
					}
				} else if method.method != http.MethodGet && back.UpdatedService == nil {
					t.Fatal("authorized request did not update the service")
				}
			})
		}
	}
}
