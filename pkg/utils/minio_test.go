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

package utils

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grycap/oscar/v3/pkg/testsupport"
	"github.com/grycap/oscar/v3/pkg/types"
)

func createMinIOConfig() (types.Config, *httptest.Server) {
	// Create a fake MinIO server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		if !strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/") {
			rw.WriteHeader(http.StatusNotFound)
		}

		if hreq.URL.Path == "/minio/admin/v3/info-canned-policy" {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"PolicyName": "testpolicy", "Policy": {"Version": "version","Statement": [{"Resource": ["res"]}]}}`))
		} else {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"Status": "success"}`))
		}
	}))

	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
		Name:        "test",
		Namespace:   "default",
		ServicePort: 8080,
	}

	return cfg, server
}

func TestCreateMinIOUser(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	// Create a fake MinIO server
	cfg, server := createMinIOConfig()

	client, err := MakeMinIOAdminClient(&cfg)

	if err != nil {
		t.Errorf("Error creating MinIO client: %v", err)
	}

	err = client.CreateMinIOUser("testuser", "testpassword")

	if err != nil {
		t.Errorf("Error creating MinIO user: %v", err)
	}

	// Close the fake MinIO server
	defer server.Close()
}

func TestCreateServiceGroup(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	// Create a fake MinIO server
	cfg, server := createMinIOConfig()

	client, _ := MakeMinIOAdminClient(&cfg)
	err := client.CreateAddGroup("bucket", []string{}, false)

	if err != nil {
		t.Errorf("Error creating MinIO user: %v", err)
	}

	// Close the fake MinIO server
	defer server.Close()
}
