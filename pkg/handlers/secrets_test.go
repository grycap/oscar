package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/backends"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
)

const testSecretServiceName = "cowsay"

func newSecretsTestBackend(owner string) (*backends.FakeBackend, *fake.Clientset, *types.Config) {
	cfg := &types.Config{ServicesNamespace: "oscar-svc"}
	namespace := utils.BuildUserNamespace(cfg, owner)
	client := fake.NewSimpleClientset(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testSecretServiceName, Namespace: namespace},
		Data:       map[string][]byte{"API_KEY": []byte("secret-value")},
	})
	_, _ = client.CoreV1().ConfigMaps(namespace).Create(context.TODO(), &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: testSecretServiceName, Namespace: namespace},
		Data: map[string]string{
			types.FDLFileName:    "name: cowsay\nimage: ghcr.io/grycap/cowsay\nenvironment:\n  secrets:\n    API_KEY: \"\"\n",
			types.ScriptFileName: "#!/bin/sh\necho ok\n",
		},
	}, metav1.CreateOptions{})
	back := backends.MakeFakeBackend()
	back.SetKubeClientset(client)
	service := &types.Service{Name: testSecretServiceName, Namespace: namespace, Owner: owner, Visibility: types.PUBLIC}
	back.Service = service
	back.Services = []*types.Service{service}
	return back, client, cfg
}

func newSecretsRouter(back types.ServerlessBackend, cfg *types.Config, uid string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", uid)
		c.Next()
	})
	r.GET("/system/secrets", MakeListSecretsHandler(back, back.GetKubeClientset(), cfg))
	r.GET("/system/secrets/:serviceName", MakeGetServiceSecretHandler(back, back.GetKubeClientset(), cfg))
	r.PUT("/system/secrets/:serviceName", MakeUpdateServiceSecretHandler(back, back.GetKubeClientset(), cfg))
	return r
}

func TestListSecretsHandler(t *testing.T) {
	back, _, cfg := newSecretsTestBackend("user@example.org")
	r := newSecretsRouter(back, cfg, "user@example.org")

	req := httptest.NewRequest(http.MethodGet, "/system/secrets", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected list secrets status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), testSecretServiceName) {
		t.Fatalf("expected list response to include service, got %s", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "secret-value") {
		t.Fatalf("expected list response to include secret value for owner, got %s", resp.Body.String())
	}
}

func TestListSecretsHandlerEmptyForNonOwnerNamespace(t *testing.T) {
	back, _, cfg := newSecretsTestBackend("user@example.org")
	r := newSecretsRouter(back, cfg, "other@example.org")

	req := httptest.NewRequest(http.MethodGet, "/system/secrets", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected list secrets status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if resp.Body.String() != "[]" {
		t.Fatalf("expected empty list for non-owner namespace, got %s", resp.Body.String())
	}
}

func TestGetServiceSecretHandler(t *testing.T) {
	back, _, cfg := newSecretsTestBackend("user@example.org")
	r := newSecretsRouter(back, cfg, "user@example.org")

	req := httptest.NewRequest(http.MethodGet, "/system/secrets/cowsay", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected get service secret status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "secret-value") {
		t.Fatalf("expected get response to include secret value, got %s", resp.Body.String())
	}
}

func TestGetServiceSecretHandlerNotFoundForNonOwner(t *testing.T) {
	back, _, cfg := newSecretsTestBackend("user@example.org")
	r := newSecretsRouter(back, cfg, "other@example.org")

	req := httptest.NewRequest(http.MethodGet, "/system/secrets/cowsay", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected get service secret status 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestGetServiceSecretHandlerNotFound(t *testing.T) {
	back, _, cfg := newSecretsTestBackend("user@example.org")
	back.AddError("ReadService", apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "missing"))
	r := newSecretsRouter(back, cfg, "user@example.org")

	req := httptest.NewRequest(http.MethodGet, "/system/secrets/missing", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected get service secret status 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUpdateServiceSecretHandler(t *testing.T) {
	back, _, cfg := newSecretsTestBackend("user@example.org")
	namespace := utils.BuildUserNamespace(cfg, "user@example.org")
	r := newSecretsRouter(back, cfg, "user@example.org")

	req := httptest.NewRequest(http.MethodPut, "/system/secrets/cowsay", strings.NewReader(`{"secrets":{"NEW_KEY":"new-value"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected update service secret status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "NEW_KEY") || !strings.Contains(resp.Body.String(), "new-value") || strings.Contains(resp.Body.String(), "secret-value") {
		t.Fatalf("expected only request keys in response, got %s", resp.Body.String())
	}

	secret, err := back.GetKubeClientset().CoreV1().Secrets(namespace).Get(t.Context(), testSecretServiceName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed retrieving secret after update: %v", err)
	}
	if string(secret.Data["API_KEY"]) != "secret-value" {
		t.Fatalf("expected existing key to be preserved, got %q", string(secret.Data["API_KEY"]))
	}
	if string(secret.Data["NEW_KEY"]) != "new-value" {
		t.Fatalf("expected new key to be stored, got %q", string(secret.Data["NEW_KEY"]))
	}

	cm, err := back.GetKubeClientset().CoreV1().ConfigMaps(namespace).Get(t.Context(), testSecretServiceName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed retrieving configmap after update: %v", err)
	}
	if !strings.Contains(cm.Data[types.FDLFileName], "NEW_KEY") {
		t.Fatalf("expected configmap FDL to include new secret key, got %s", cm.Data[types.FDLFileName])
	}
	if !strings.Contains(cm.Data[types.FDLFileName], "API_KEY") {
		t.Fatalf("expected configmap FDL to keep existing secret key, got %s", cm.Data[types.FDLFileName])
	}
	if strings.Contains(cm.Data[types.FDLFileName], "new-value") {
		t.Fatalf("expected configmap FDL to not include secret values, got %s", cm.Data[types.FDLFileName])
	}
}

func TestUpdateServiceSecretHandlerRejectsReservedKey(t *testing.T) {
	back, _, cfg := newSecretsTestBackend("user@example.org")
	r := newSecretsRouter(back, cfg, "user@example.org")

	req := httptest.NewRequest(http.MethodPut, "/system/secrets/cowsay", strings.NewReader(`{"secrets":{"refresh_token":"token"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected update service secret status 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUpdateServiceSecretHandlerRejectsEmptyBody(t *testing.T) {
	back, _, cfg := newSecretsTestBackend("user@example.org")
	r := newSecretsRouter(back, cfg, "user@example.org")

	req := httptest.NewRequest(http.MethodPut, "/system/secrets/cowsay", strings.NewReader(`{"secrets":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected update service secret status 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUpdateServiceSecretHandlerForbiddenForNonOwner(t *testing.T) {
	back, _, cfg := newSecretsTestBackend("user@example.org")
	r := newSecretsRouter(back, cfg, "other@example.org")

	req := httptest.NewRequest(http.MethodPut, "/system/secrets/cowsay", strings.NewReader(`{"secrets":{"NEW_KEY":"value"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected update service secret status 403, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUpdateServiceSecretHandlerNotFound(t *testing.T) {
	back, _, cfg := newSecretsTestBackend("user@example.org")
	back.AddError("ReadService", apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "missing"))
	r := newSecretsRouter(back, cfg, "user@example.org")

	req := httptest.NewRequest(http.MethodPut, "/system/secrets/missing", strings.NewReader(`{"secrets":{"K":"V"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected update service secret status 404, got %d: %s", resp.Code, resp.Body.String())
	}
}
