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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
)

func TestMakeConfigHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &types.Config{
		// Initialize with necessary fields
		MinIOProvider: &types.MinIOProvider{
			Endpoint: "http://minio.example.com",
			Verify:   true,
			Region:   "us-east-1",
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

	/*
		K8sObjects := []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: "namespace",
				},
			},
		}

		kubeClientset := testclient.NewSimpleClientset(K8sObjects...)
		t.Run("With Authorization Header", func(t *testing.T) {
			router := gin.New()
			router.GET("/config", MakeConfigHandler(cfg))

			req, _ := http.NewRequest("GET", "/config", nil)
			req.Header.Set("Authorization", "Bearer some-token")
			w := httptest.NewRecorder()

			// Mocking auth functions
			monkey.Patch(auth.GetUIDFromContext, func(c *gin.Context) (string, error) {
				return "somelonguserid@egi.eu", nil
			})

			monkey.Patch(auth.GetMultitenancyConfigFromContext, func(c *gin.Context) (*auth.MultitenancyConfig, error) {
				return auth.NewMultitenancyConfig(kubeClientset, "somelonguserid@egi.eu"), nil
			})

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status code 200, got %d", w.Code)
			}
		})
	*/

	t.Run("With Invalid Authorization Header", func(t *testing.T) {
		router := gin.New()
		router.GET("/config", MakeConfigHandler(cfg))

		req, _ := http.NewRequest("GET", "/config", nil)
		req.Header.Set("Authorization", "InvalidToken")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status code 200, got %d", w.Code)
		}
		//assert.Contains(t, w.Body.String(), "http://minio.example.com")
	})
}
