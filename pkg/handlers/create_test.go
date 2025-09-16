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
		fmt.Printf("Received request: %s %s\n", hreq.Method, hreq.URL.String())
		if hreq.URL.Path != "/test" && hreq.URL.Path != "/" && hreq.URL.Path != "/test/" && hreq.URL.Path != "/test/input/" && hreq.URL.Path != "/test/output/" && hreq.URL.Path != "/test/mount/" && !strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/") && !strings.HasPrefix(hreq.URL.Path, "/test-somelongui") {
			t.Errorf("Unexpected path in request, got: %s", hreq.URL.Path)
		}

		if hreq.URL.Path == "/minio/admin/v3/info" {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"Mode": "local", "Region": "us-east-1"}`))
		} else if strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/info-canned-policy") {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{
				"Version": "2012-10-17",
				"Statement": [
					{
						"Effect": "Allow",
						"Action": [
							"s3:*"
						],
						"Resource": [
							"arn:aws:s3:::test/*",
						]
					}
				]
			}`))
		} else if strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/set-user-or-group-policy") {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"status":"success","binding":"done"}`))
		} else if hreq.Method == http.MethodGet && strings.HasPrefix(hreq.URL.Path, "/test") && hreq.URL.RawQuery == "location=" {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/"/>`))
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

	scenarios := []struct {
		name           string
		visibility     string
		allowedUsers   []string
		expectedStatus int
	}{
		{"PublicVisibility", "public", []string{}, http.StatusCreated},
		{"InvalidVisibility", "private", []string{}, http.StatusCreated},
		{"EmptyVisibility", "", []string{}, http.StatusCreated}, // Assuming default is allowed
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			allowedUsersJSON := "["
			for i, user := range s.allowedUsers {
				if i > 0 {
					allowedUsersJSON += ","
				}
				allowedUsersJSON += `"` + user + `"`
			}
			allowedUsersJSON += "]"

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
					],
					"output": [
					],
					"mount": {
						"storage_provider": "minio",
						"path": "test/mount"
					},
					"storage_providers": {
						"webdav": {
							"id": {
								"hostname": "` + server.URL + `",
								"login": "user",
								"password": "pass"
							}
						}
					},
					"isolation_level": "",
					"bucket_list": [],
					"visibility": "` + s.visibility + `",
					"allowed_users": []
				}`)

			req, _ := http.NewRequest("POST", "/system/services", body)
			req.Header.Add("Authorization", "Bearer token")
			r.ServeHTTP(w, req)

			if w.Code != s.expectedStatus {
				fmt.Println("response: ", w.Body)
				t.Errorf("expecting code %d, got %d", s.expectedStatus, w.Code)
			}
		})
	}

	// Close the fake MinIO server
	defer server.Close()
}
