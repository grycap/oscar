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

package types

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/grycap/oscar/v4/pkg/testsupport"
)

// Both backends must produce the same policy documents and associations. Only
// the group request format differs for RustFS.
func TestRustFSPoliciesMatchMinIO(t *testing.T) {
	testsupport.SkipIfCannotListen(t)
	var minioPolicies, minioBindings []string
	for _, backend := range []string{ObjectStorageMinIO, ObjectStorageRustFS} {
		t.Run(backend, func(t *testing.T) {
			mock := newMinioMock()
			var policies, bindings []string
			groupRequests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/minio/admin/v3/info-canned-policy":
					if backend == ObjectStorageRustFS {
						name := r.URL.Query().Get("name")
						if _, exists := mock.policies[name]; !exists {
							http.Error(w, "not found", http.StatusNotFound)
							return
						}
						var info struct{ Policy json.RawMessage }
						if err := json.Unmarshal(mock.policyResponse(name), &info); err != nil {
							t.Error(err)
						}
						json.NewEncoder(w).Encode(map[string]any{"policy_name": name, "policy": info.Policy})
						return
					}
				case "/minio/admin/v3/add-canned-policy":
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					policies = append(policies, r.URL.Query().Get("name")+":"+string(body))
					r.Body = io.NopCloser(bytes.NewReader(body))
				case "/minio/admin/v3/set-user-or-group-policy":
					bindings = append(bindings, r.URL.RawQuery)
				case "/minio/admin/v3/update-group-members":
					groupRequests++
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					var group struct {
						Status string `json:"groupStatus"`
					}
					if err := json.Unmarshal(body, &group); err != nil {
						t.Error(err)
					}
					want := "enable"
					if backend == ObjectStorageRustFS {
						want = "enabled"
					}
					if group.Status != want {
						t.Errorf("group status = %q, want %q", group.Status, want)
					}
					r.Body = io.NopCloser(bytes.NewReader(body))
				}
				mock.ServeHTTP(w, r)
			}))
			defer server.Close()
			client, err := MakeMinIOAdminClient(&Config{ObjectStorageType: backend, MinIOProvider: &MinIOProvider{Endpoint: server.URL, AccessKey: "YOUR_ACCESS_KEY", SecretKey: "YOUR_SECRET_KEY"}})
			if err != nil {
				t.Fatal(err)
			}
			buckets := []MinIOBucket{
				{BucketName: "first", Owner: "alice", Visibility: PRIVATE},
				{BucketName: "second", Owner: "alice", Visibility: PRIVATE},
				{BucketName: "shared", Owner: "alice", Visibility: RESTRICTED, AllowedUsers: []string{"bob", "carol"}},
				{BucketName: "public", Owner: "alice", Visibility: PUBLIC},
			}
			for _, bucket := range buckets {
				if err := client.SetPolicies(bucket); err != nil {
					t.Fatal(err)
				}
				if got := client.GetCurrentResourceVisibility(bucket); got != bucket.Visibility {
					t.Fatalf("visibility = %q, want %q", got, bucket.Visibility)
				}
			}
			if err := client.UpdateServiceGroup("shared", []string{"carol"}); err != nil {
				t.Fatal(err)
			}
			if got := mock.groupMembers["shared"]; !reflect.DeepEqual(got, []string{"carol"}) {
				t.Fatalf("members = %v", got)
			}
			for _, bucket := range buckets {
				if err := client.UnsetPolicies(bucket); err != nil {
					t.Fatal(err)
				}
			}
			if groupRequests == 0 {
				t.Fatal("no group requests received")
			}
			if backend == ObjectStorageMinIO {
				minioPolicies = policies
				minioBindings = bindings
			} else {
				if !reflect.DeepEqual(policies, minioPolicies) {
					t.Fatalf("RustFS policy documents differ from MinIO:\ngot %v\nwant %v", policies, minioPolicies)
				}
				if !reflect.DeepEqual(bindings, minioBindings) {
					t.Fatalf("RustFS policy assignments differ from MinIO:\ngot %v\nwant %v", bindings, minioBindings)
				}
			}
		})
	}
}
