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
	"fmt"
	"strings"
	"testing"

	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestMakeCreateHandler(t *testing.T) {
	back := backends.MakeFakeBackend()
	kubeClientset := testclient.NewSimpleClientset()

	// Create a fake MinIO server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {

		if hreq.URL.Path != "/test" && hreq.URL.Path != "/test/input/" && hreq.URL.Path != "/output" && !strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/") {
			t.Errorf("Unexpected path in request, got: %s", hreq.URL.Path)
		}

		fmt.Println(hreq.URL.Path)

		if hreq.URL.Path == "/minio/admin/v3/info" {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"Mode": "local", "Region": "us-east-1"}`))
		} else {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"status": "success"}`))
		}
	}))

	// and set the MinIO endpoint to the fake server
	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}
	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "somelonguid@egi.eu")
		c.Set("multitenancyConfig", auth.NewMultitenancyConfig(kubeClientset, "somelonguid@egi.eu"))
		c.Next()
	})
	r.POST("/system/services", MakeCreateHandler(&cfg, back))

	w := httptest.NewRecorder()
	body := strings.NewReader(`
		{
			"name": "cowsay",
			"cluster_id": "oscar",
			"memory": "1Gi",
			"cpu": "1.0",
			"log_level": "CRITICAL",
			"image": "ghcr.io/grycap/cowsay",
			"alpine": false,
			"script": "test",
			"input": [
				{
				"storage_provider": "minio",
				"path": "/test/input/"
				}
  			],
			"output": [
				{
				"storage_provider": "webdav.id",
				"path": "/output"
				}
  			],
			"storage_providers": {
				"webdav": {
					"id": {
						"hostname": "` + server.URL + `",
						"login": "user",
						"password": "pass"
					}
				}
			},
			"allowed_users": ["somelonguid@egi.eu", "somelonguid2@egi.eu"]
		}
	`)

	req, _ := http.NewRequest("POST", "/system/services", body)
	req.Header.Add("Authorization", "Bearer token")
	r.ServeHTTP(w, req)

	// Close the fake MinIO server
	defer server.Close()

	if w.Code != http.StatusCreated {
		fmt.Println(w.Body)
		t.Errorf("expecting code %d, got %d", http.StatusCreated, w.Code)
	}
}
