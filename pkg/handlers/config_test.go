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
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func createExpectedBody(access_key string, secret_key string, cfg *types.Config) map[string]interface{} {
	return map[string]interface{}{
		"config": map[string]interface{}{
			"name":                       "",
			"namespace":                  "",
			"services_namespace":         "",
			"controller_service_account": "",
			"gpu_available":              false,
			"interLink_available":        false,
			"yunikorn_enable":            false,
			"oidc_groups":                nil,
		},
		"minio_provider": map[string]interface{}{
			"endpoint":   cfg.MinIOProvider.Endpoint,
			"verify":     cfg.MinIOProvider.Verify,
			"access_key": access_key,
			"secret_key": secret_key,
			"region":     cfg.MinIOProvider.Region,
		},
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

	t.Run("Without Authorization Header", func(t *testing.T) {
		router := gin.New()
		router.GET("/config", MakeConfigHandler(cfg))

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

	kubeClientset := testclient.NewSimpleClientset(K8sObjects...)
	t.Run("With Bearer Authorization Header", func(t *testing.T) {
		router := gin.New()
		router.GET("/config", MakeConfigHandler(cfg))

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
		router.GET("/config", MakeConfigHandler(cfg))

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
