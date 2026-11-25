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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func createExpectedBody(access_key string, secret_key string, cfg *types.Config) map[string]interface{} {
	return map[string]interface{}{
		"config": map[string]interface{}{
			"name":                                 "",
			"namespace":                            "",
			"services_namespace":                   "",
			"controller_service_account":           "",
			"pod_cidr":                             "",
			"service_cidr":                         "",
			"gpu_available":                        false,
			"interLink_available":                  false,
			"yunikorn_enable":                      false,
			"kueue_enable":                         false,
			"volume_enable":                        false,
			"oidc_groups":                          nil,
			"minio_quota_enabled":                  false,
			"exposed_services_use_subdomain_route": false,
		},
		"minio_provider": map[string]interface{}{
			"endpoint":   cfg.MinIOProvider.Endpoint,
			"verify":     cfg.MinIOProvider.Verify,
			"access_key": access_key,
			"secret_key": secret_key,
			"region":     cfg.MinIOProvider.Region,
		},
		"allowed_image_repositories": []interface{}{},
	}
}

func TestMakeConfigHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &types.Config{
		// Initialize with necessary fields
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  "http://minio.example.com",
			Verify:    true,
			Region:    "us-east-1",
			AccessKey: "accessKey1",
			SecretKey: "secretKey1",
		},
	}

	kubeClientset := testclient.NewSimpleClientset()
	t.Run("Without Authorization Header", func(t *testing.T) {
		router := gin.New()
		router.GET("/config", MakeConfigHandler(cfg, kubeClientset))

		req, _ := http.NewRequest("GET", "/config", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status code 200, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "http://minio.example.com") {
			t.Fatalf("Unexpected response body")
		}

	})

	K8sObjects := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "somelonguserid",
				Namespace: auth.ServicesNamespace,
			},
			Data: map[string][]byte{
				"accessKey": []byte("accessKey"),
				"secretKey": []byte("secretKey"),
			},
		},
	}

	kubeClientset = testclient.NewSimpleClientset(K8sObjects...)
	t.Run("With Bearer Authorization Header", func(t *testing.T) {
		router := gin.New()
		router.GET("/config", MakeConfigHandler(cfg, kubeClientset))

		req, _ := http.NewRequest("GET", "/config", nil)
		req.Header.Set("Authorization", "Bearer some-token")
		w := httptest.NewRecorder()

		// Mock auth helpers using test hook variables
		origGetUID := getUIDFromContextFn
		origGetMultitenancy := getMultitenancyConfigFromContextFn
		getUIDFromContextFn = func(*gin.Context) (string, error) {
			return "somelonguserid@egi.eu", nil
		}
		getMultitenancyConfigFromContextFn = func(*gin.Context) (*auth.MultitenancyConfig, error) {
			return auth.NewMultitenancyConfig(kubeClientset, "somelonguserid@egi.eu"), nil
		}
		t.Cleanup(func() {
			getUIDFromContextFn = origGetUID
			getMultitenancyConfigFromContextFn = origGetMultitenancy
		})

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status code 200, got %d", w.Code)
		}

		expected_body := createExpectedBody("accessKey", "secretKey", cfg)

		var responseBody map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
			t.Fatalf("Failed to parse response body: %v", err)
		}

		if !reflect.DeepEqual(responseBody, expected_body) {
			t.Fatalf("Unexpected response body: %s", w.Body.String())
		}

	})

	t.Run("With Token Authorization Header", func(t *testing.T) {
		router := gin.New()
		router.GET("/config", MakeConfigHandler(cfg, kubeClientset))

		req, _ := http.NewRequest("GET", "/config", nil)
		req.Header.Set("Authorization", "SomeToken")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status code 200, got %d", w.Code)
		}

		expected_body := createExpectedBody("accessKey1", "secretKey1", cfg)

		var responseBody map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
			t.Fatalf("Failed to parse response body: %v", err)
		}

		if !reflect.DeepEqual(responseBody, expected_body) {
			t.Fatalf("Unexpected response body: %s", w.Body.String())
		}
	})
}

func TestMakeConfigUpdateHandlerRejectsBearerWithoutMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &types.Config{
		Namespace:            "oscar",
		AdditionalConfigPath: "oscar-config",
		MinIOProvider:        &types.MinIOProvider{},
	}
	kubeClientset := testclient.NewSimpleClientset()

	router := gin.New()
	router.PUT("/config", MakeConfigUpdateHandler(cfg, kubeClientset))

	body := `{"allowed_image_repositories":["registry.example.com/"]}`
	req := httptest.NewRequest(http.MethodPut, "/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status code 401, got %d", w.Code)
	}

	_, err := kubeClientset.CoreV1().ConfigMaps(cfg.Namespace).Get(
		req.Context(),
		cfg.AdditionalConfigPath,
		metav1.GetOptions{},
	)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected config map not to be created, got err=%v", err)
	}
}

func TestMakeConfigUpdateHandlerAllowedImageRepositories(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validEntries := []string{"ghcr.io/grycap", "registry.example.org/team"}

	t.Run("Accepts valid entries and creates the ConfigMap", func(t *testing.T) {
		cfg := &types.Config{
			Namespace:            "oscar",
			AdditionalConfigPath: "oscar-config",
			MinIOProvider:        &types.MinIOProvider{},
		}
		kubeClientset := testclient.NewSimpleClientset()

		body := `{"allowed_image_repositories":["ghcr.io/grycap","registry.example.org/team"]}`
		req := httptest.NewRequest(http.MethodPut, "/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router := gin.New()
		router.PUT("/config", MakeConfigUpdateHandler(cfg, kubeClientset))
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status code 200, got %d: %s", w.Code, w.Body.String())
		}
		resp := newAIRResponse(t, w)
		assertAIRsEqual(t, resp, validEntries)

		cm, err := kubeClientset.CoreV1().ConfigMaps(cfg.Namespace).Get(
			req.Context(), cfg.AdditionalConfigPath, metav1.GetOptions{},
		)
		if err != nil {
			t.Fatalf("expected config map to exist: %v", err)
		}
		var stored []string
		if err := json.Unmarshal([]byte(cm.Data[types.AIR]), &stored); err != nil {
			t.Fatalf("cannot parse stored policy %q: %v", cm.Data[types.AIR], err)
		}
		assertAIRsEqual(t, stored, validEntries)
	})

	t.Run("Accepts an empty list", func(t *testing.T) {
		cfg := &types.Config{
			Namespace:            "oscar",
			AdditionalConfigPath: "oscar-config",
			MinIOProvider:        &types.MinIOProvider{},
		}
		kubeClientset := testclient.NewSimpleClientset()

		body := `{"allowed_image_repositories":[]}`
		req := httptest.NewRequest(http.MethodPut, "/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router := gin.New()
		router.PUT("/config", MakeConfigUpdateHandler(cfg, kubeClientset))
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status code 200, got %d: %s", w.Code, w.Body.String())
		}
		resp := newAIRResponse(t, w)
		if len(resp) != 0 {
			t.Fatalf("expected an empty list, got %v", resp)
		}
	})

	t.Run("Rejects malformed entries with 400", func(t *testing.T) {
		cfg := &types.Config{
			Namespace:            "oscar",
			AdditionalConfigPath: "oscar-config",
			MinIOProvider:        &types.MinIOProvider{},
		}
		kubeClientset := testclient.NewSimpleClientset()

		body := `{"allowed_image_repositories":["ghcr.io/grycap","test/test","test"]}`
		req := httptest.NewRequest(http.MethodPut, "/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router := gin.New()
		router.PUT("/config", MakeConfigUpdateHandler(cfg, kubeClientset))
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status code 400, got %d: %s", w.Code, w.Body.String())
		}
		resp := w.Body.String()
		for _, expected := range []string{
			`invalid allowed image repository "test/test": strict validation requires the registry to be explicitly defined`,
			`invalid allowed image repository "test": strict validation requires the registry to be explicitly defined`,
		} {
			if !strings.Contains(resp, expected) {
				t.Fatalf("response %q does not mention %q", resp, expected)
			}
		}

		_, err := kubeClientset.CoreV1().ConfigMaps(cfg.Namespace).Get(
			req.Context(), cfg.AdditionalConfigPath, metav1.GetOptions{},
		)
		if !apierrors.IsNotFound(err) {
			t.Fatalf("expected config map not to be created, got err=%v", err)
		}
	})

	t.Run("Updates an existing ConfigMap", func(t *testing.T) {
		cfg := &types.Config{
			Namespace:            "oscar",
			AdditionalConfigPath: "oscar-config",
			MinIOProvider:        &types.MinIOProvider{},
		}
		existing := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            cfg.AdditionalConfigPath,
				Namespace:       cfg.Namespace,
				ResourceVersion: "1",
			},
			Data: map[string]string{types.AIR: `["old.example/old"]`},
		}
		kubeClientset := testclient.NewSimpleClientset(existing)

		body := `{"allowed_image_repositories":["ghcr.io/grycap"]}`
		req := httptest.NewRequest(http.MethodPut, "/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router := gin.New()
		router.PUT("/config", MakeConfigUpdateHandler(cfg, kubeClientset))
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status code 200, got %d: %s", w.Code, w.Body.String())
		}
		cm, err := kubeClientset.CoreV1().ConfigMaps(cfg.Namespace).Get(
			req.Context(), cfg.AdditionalConfigPath, metav1.GetOptions{},
		)
		if err != nil {
			t.Fatalf("expected config map to exist: %v", err)
		}
		var stored []string
		if err := json.Unmarshal([]byte(cm.Data[types.AIR]), &stored); err != nil {
			t.Fatalf("cannot parse stored policy %q: %v", cm.Data[types.AIR], err)
		}
		assertAIRsEqual(t, stored, []string{"ghcr.io/grycap"})
	})
}

func newAIRResponse(t *testing.T, w *httptest.ResponseRecorder) []string {
	t.Helper()
	var resp []string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("cannot parse response body %q: %v", w.Body.String(), err)
	}
	return resp
}

func assertAIRsEqual(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
