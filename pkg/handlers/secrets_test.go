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
	"github.com/grycap/oscar/v4/pkg/utils/auth"
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
	r.GET("/system/secrets", MakeGetSecretsHandler(back, back.GetKubeClientset(), cfg))
	r.PUT("/system/secrets", MakeUpdateSecretsHandler(back, back.GetKubeClientset(), cfg))
	r.GET("/system/services/:serviceName/secrets", MakeGetServiceSecretHandler(back, back.GetKubeClientset(), cfg))
	r.PUT("/system/services/:serviceName/secrets", MakeUpdateServiceSecretHandler(back, back.GetKubeClientset(), cfg))
	return r
}

func TestGetSecretsHandlerByKey(t *testing.T) {
	back, client, cfg := newSecretsTestBackend("user@example.org")
	namespace := utils.BuildUserNamespace(cfg, "user@example.org")
	_, _ = client.CoreV1().Secrets(namespace).Create(context.TODO(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: auth.FormatUID("user@example.org"), Namespace: namespace},
		Data:       map[string][]byte{"API_KEY": []byte("user-secret-value")},
	}, metav1.CreateOptions{})
	r := newSecretsRouter(back, cfg, "user@example.org")

	req := httptest.NewRequest(http.MethodGet, "/system/secrets?key=API_KEY", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected get secrets status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if resp.Body.String() != "user-secret-value" {
		t.Fatalf("expected get secrets response to be the secret value, got %q", resp.Body.String())
	}
}

func TestGetSecretsHandlerRequiresKey(t *testing.T) {
	back, client, cfg := newSecretsTestBackend("user@example.org")
	namespace := utils.BuildUserNamespace(cfg, "user@example.org")
	_, _ = client.CoreV1().Secrets(namespace).Create(context.TODO(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: auth.FormatUID("user@example.org"), Namespace: namespace},
		Data:       map[string][]byte{"API_KEY": []byte("user-secret-value")},
	}, metav1.CreateOptions{})
	r := newSecretsRouter(back, cfg, "user@example.org")

	req := httptest.NewRequest(http.MethodGet, "/system/secrets", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected get secrets status 404 without key, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUpdateSecretsHandler(t *testing.T) {
	back, client, cfg := newSecretsTestBackend("user@example.org")
	namespace := utils.BuildUserNamespace(cfg, "user@example.org")
	_, _ = client.CoreV1().Secrets(namespace).Create(context.TODO(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: auth.FormatUID("user@example.org"), Namespace: namespace},
		Data:       map[string][]byte{"API_KEY": []byte("user-secret-value")},
	}, metav1.CreateOptions{})
	r := newSecretsRouter(back, cfg, "user@example.org")

	req := httptest.NewRequest(http.MethodPut, "/system/secrets", strings.NewReader(`{"NEW_KEY":"new-value"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected update secrets status 204, got %d: %s", resp.Code, resp.Body.String())
	}

	secret, err := client.CoreV1().Secrets(namespace).Get(t.Context(), auth.FormatUID("user@example.org"), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed retrieving secret after update: %v", err)
	}
	if string(secret.Data["API_KEY"]) != "user-secret-value" {
		t.Fatalf("expected existing key to be preserved, got %q", string(secret.Data["API_KEY"]))
	}
	if string(secret.Data["NEW_KEY"]) != "new-value" {
		t.Fatalf("expected new key to be stored, got %q", string(secret.Data["NEW_KEY"]))
	}
}

func TestGetServiceSecretHandlerRequiresKey(t *testing.T) {
	back, _, cfg := newSecretsTestBackend("user@example.org")
	r := newSecretsRouter(back, cfg, "user@example.org")

	req := httptest.NewRequest(http.MethodGet, "/system/services/cowsay/secrets", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected get service secret status 404 without key, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestGetServiceSecretHandlerByKey(t *testing.T) {
	back, _, cfg := newSecretsTestBackend("user@example.org")
	r := newSecretsRouter(back, cfg, "user@example.org")

	req := httptest.NewRequest(http.MethodGet, "/system/services/cowsay/secrets?key=API_KEY", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected get service secret status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if resp.Body.String() != "secret-value" {
		t.Fatalf("expected get response to be the secret value, got %q", resp.Body.String())
	}
}

func TestGetServiceSecretHandlerByMissingKey(t *testing.T) {
	back, _, cfg := newSecretsTestBackend("user@example.org")
	r := newSecretsRouter(back, cfg, "user@example.org")

	req := httptest.NewRequest(http.MethodGet, "/system/services/cowsay/secrets?key=NONEXISTENT", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected get service secret status 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestGetServiceSecretHandlerNotFoundForNonOwner(t *testing.T) {
	back, _, cfg := newSecretsTestBackend("user@example.org")
	r := newSecretsRouter(back, cfg, "other@example.org")

	req := httptest.NewRequest(http.MethodGet, "/system/services/cowsay/secrets", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/system/services/missing/secrets", nil)
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

	req := httptest.NewRequest(http.MethodPut, "/system/services/cowsay/secrets", strings.NewReader(`{"NEW_KEY":"new-value"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected update service secret status 204, got %d: %s", resp.Code, resp.Body.String())
	}
	if resp.Body.Len() != 0 {
		t.Fatalf("expected empty response body, got %s", resp.Body.String())
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

	req := httptest.NewRequest(http.MethodPut, "/system/services/cowsay/secrets", strings.NewReader(`{"refresh_token":"token"}`))
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

	req := httptest.NewRequest(http.MethodPut, "/system/services/cowsay/secrets", strings.NewReader(`{}`))
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

	req := httptest.NewRequest(http.MethodPut, "/system/services/cowsay/secrets", strings.NewReader(`{"NEW_KEY":"value"}`))
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

	req := httptest.NewRequest(http.MethodPut, "/system/services/missing/secrets", strings.NewReader(`{"K":"V"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected update service secret status 404, got %d: %s", resp.Code, resp.Body.String())
	}
}
