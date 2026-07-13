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
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grycap/oscar/v4/pkg/testsupport"
	"github.com/grycap/oscar/v4/pkg/types"
)

func TestMakeObjectStorageIAM(t *testing.T) {
	tests := []struct {
		name        string
		storageType string
		assertType  func(ObjectStorageIAM) bool
	}{
		{
			name:        "default is MinIO",
			storageType: "",
			assertType: func(iam ObjectStorageIAM) bool {
				_, ok := iam.(*minIOIAM)
				return ok
			},
		},
		{
			name:        "explicit MinIO",
			storageType: "MINIO",
			assertType: func(iam ObjectStorageIAM) bool {
				_, ok := iam.(*minIOIAM)
				return ok
			},
		},
		{
			name:        "RustFS",
			storageType: "rustfs",
			assertType: func(iam ObjectStorageIAM) bool {
				_, ok := iam.(*rustFSIAM)
				return ok
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := objectStorageIAMTestConfig(tt.storageType)
			iam, err := MakeObjectStorageIAM(cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.assertType(iam) {
				t.Fatalf("unexpected IAM implementation %T", iam)
			}
		})
	}
}

func TestRustFSWebhookConfiguration(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	type receivedRequest struct {
		method string
		path   string
		body   string
		auth   string
	}
	requests := make(chan receivedRequest, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}
		requests <- receivedRequest{
			method: r.Method,
			path:   r.URL.Path,
			body:   string(body),
			auth:   r.Header.Get("Authorization"),
		}
		if r.Method == http.MethodGet && r.URL.Path == "/rustfs/admin/v3/module-switches" {
			_, _ = w.Write([]byte(`{"notify_enabled":false,"audit_enabled":true}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := objectStorageIAMTestConfig(ObjectStorageRustFS)
	cfg.MinIOProvider.Endpoint = server.URL

	if err := RegisterObjectStorageWebhook(context.Background(), cfg, "service-one", "token-one"); err != nil {
		t.Fatalf("registering RustFS webhook: %v", err)
	}
	moduleState := <-requests
	if moduleState.method != http.MethodGet || moduleState.path != "/rustfs/admin/v3/module-switches" {
		t.Fatalf("unexpected module state request: %s %s", moduleState.method, moduleState.path)
	}
	moduleUpdate := <-requests
	if moduleUpdate.method != http.MethodPut || moduleUpdate.path != "/rustfs/admin/v3/module-switches" {
		t.Fatalf("unexpected module update request: %s %s", moduleUpdate.method, moduleUpdate.path)
	}
	if moduleUpdate.body != `{"notify_enabled":true,"audit_enabled":true}` {
		t.Fatalf("unexpected module update body: %q", moduleUpdate.body)
	}
	register := <-requests
	if register.method != http.MethodPut || register.path != "/rustfs/admin/v3/target/notify_webhook/service-one" {
		t.Fatalf("unexpected register request: %s %s", register.method, register.path)
	}
	wantBody := `{"key_values":[{"key":"endpoint","value":"http://oscar.oscar:8080/job/service-one"},{"key":"auth_token","value":"token-one"},{"key":"queue_dir","value":""}]}`
	if register.body != wantBody {
		t.Fatalf("unexpected register body: %q", register.body)
	}
	if !strings.HasPrefix(register.auth, "AWS4-HMAC-SHA256 ") {
		t.Fatalf("register request was not signed: %q", register.auth)
	}

	if err := RemoveObjectStorageWebhook(context.Background(), cfg, "service-one"); err != nil {
		t.Fatalf("removing RustFS webhook: %v", err)
	}
	remove := <-requests
	if remove.method != http.MethodDelete || remove.path != "/rustfs/admin/v3/target/notify_webhook/service-one/reset" {
		t.Fatalf("unexpected remove request: %s %s", remove.method, remove.path)
	}
	if remove.body != "" {
		t.Fatalf("unexpected remove body: %q", remove.body)
	}
}

func TestMakeObjectStorageIAMRejectsUnknownType(t *testing.T) {
	cfg := objectStorageIAMTestConfig("unknown")
	if _, err := MakeObjectStorageIAM(cfg); err == nil {
		t.Fatal("expected unsupported object storage type error")
	}
}

func objectStorageIAMTestConfig(storageType string) *types.Config {
	return &types.Config{
		ObjectStorageType: storageType,
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  "http://object-storage.example.test:9000",
			AccessKey: "admin",
			SecretKey: "secret",
			Region:    "us-east-1",
		},
		Name:        "oscar",
		Namespace:   "oscar",
		ServicePort: 8080,
	}
}
