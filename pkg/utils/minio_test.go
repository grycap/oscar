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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grycap/oscar/v3/pkg/testsupport"
	"github.com/grycap/oscar/v3/pkg/types"
	madmin "github.com/minio/madmin-go"
)

type minioMock struct {
	policies     map[string][]string
	groupMembers map[string][]string
	configWrites []string
}

func newMinioMock() *minioMock {
	return &minioMock{
		policies:     map[string][]string{},
		groupMembers: map[string][]string{},
	}
}

func (m *minioMock) policyResponse(name string) []byte {
	resources := m.policies[name]
	resJSON, _ := json.Marshal(resources)
	return []byte(fmt.Sprintf(`{"PolicyName":"%s","Policy":{"Version":"version","Statement":[{"Resource":%s}]}}`, name, string(resJSON)))
}

func (m *minioMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(r.URL.Path, "/minio/admin/v3/info-canned-policy"):
		name := r.URL.Query().Get("name")
		if _, ok := m.policies[name]; !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(m.policyResponse(name))
	case strings.HasPrefix(r.URL.Path, "/minio/admin/v3/add-canned-policy"):
		name := r.URL.Query().Get("name")
		body, _ := io.ReadAll(r.Body)
		var policy Policy
		if err := json.Unmarshal(body, &policy); err == nil {
			if len(policy.Statement) > 0 {
				m.policies[name] = append([]string(nil), policy.Statement[0].Resource...)
			} else {
				m.policies[name] = []string{}
			}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Status":"success"}`))
	case strings.HasPrefix(r.URL.Path, "/minio/admin/v3/set-user-or-group-policy"):
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Status":"success"}`))
	case strings.HasPrefix(r.URL.Path, "/minio/admin/v3/remove-canned-policy"):
		name := r.URL.Query().Get("name")
		delete(m.policies, name)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Status":"success"}`))
	case strings.HasPrefix(r.URL.Path, "/minio/admin/v3/update-group-members"):
		body, _ := io.ReadAll(r.Body)
		var group madmin.GroupAddRemove
		if err := json.Unmarshal(body, &group); err == nil {
			current := append([]string(nil), m.groupMembers[group.Group]...)
			if group.IsRemove {
				m.groupMembers[group.Group] = filterMembers(current, group.Members)
			} else {
				m.groupMembers[group.Group] = mergeMembers(current, group.Members)
			}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Status":"success"}`))
	case strings.HasPrefix(r.URL.Path, "/minio/admin/v3/group"):
		group := r.URL.Query().Get("group")
		members := m.groupMembers[group]
		membersJSON, _ := json.Marshal(members)
		fmt.Fprintf(w, `{"name":"%s","status":"enable","members":%s,"policy":""}`, group, string(membersJSON))
	case strings.HasPrefix(r.URL.Path, "/minio/admin/v3/set-config-kv"):
		body, _ := io.ReadAll(r.Body)
		m.configWrites = append(m.configWrites, string(body))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Status":"success"}`))
	case strings.HasPrefix(r.URL.Path, "/minio/admin/v3/del-config-kv"):
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Status":"success"}`))
	default:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Status":"success"}`))
	}
}

func filterMembers(current []string, removals []string) []string {
	removalSet := map[string]struct{}{}
	for _, r := range removals {
		removalSet[r] = struct{}{}
	}
	result := make([]string, 0, len(current))
	for _, member := range current {
		if _, remove := removalSet[member]; !remove {
			result = append(result, member)
		}
	}
	return result
}

func mergeMembers(current []string, additions []string) []string {
	seen := map[string]struct{}{}
	for _, member := range current {
		seen[member] = struct{}{}
	}
	result := append([]string{}, current...)
	for _, add := range additions {
		if _, ok := seen[add]; !ok {
			seen[add] = struct{}{}
			result = append(result, add)
		}
	}
	return result
}

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

func TestResourceInPolicy(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	mock := newMinioMock()
	mock.policies["owner"] = []string{"arn:aws:s3:::bucket/*"}
	server := httptest.NewServer(mock)
	defer server.Close()

	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}

	client, err := MakeMinIOAdminClient(&cfg)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	if !client.ResourceInPolicy("owner", "bucket") {
		t.Fatalf("expected resource to be present in owner policy")
	}
	if client.ResourceInPolicy("missing", "bucket") {
		t.Fatalf("expected missing policy to return false")
	}
}

func TestGetCurrentResourceVisibility(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	mock := newMinioMock()
	server := httptest.NewServer(mock)
	defer server.Close()

	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}

	client, err := MakeMinIOAdminClient(&cfg)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	tests := []struct {
		name     string
		policies map[string][]string
		expected string
	}{
		{
			name: "private",
			policies: map[string][]string{
				"owner": []string{"arn:aws:s3:::bucket/*"},
			},
			expected: PRIVATE,
		},
		{
			name: "restricted",
			policies: map[string][]string{
				"owner":  []string{"arn:aws:s3:::bucket/*"},
				"bucket": []string{"arn:aws:s3:::bucket/*"},
			},
			expected: RESTRICTED,
		},
		{
			name: "public",
			policies: map[string][]string{
				ALL_USERS_GROUP: []string{"arn:aws:s3:::bucket/*"},
			},
			expected: PUBLIC,
		},
		{
			name:     "none",
			policies: map[string][]string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.policies = map[string][]string{}
			for k, v := range tt.policies {
				mock.policies[k] = append([]string(nil), v...)
			}

			visibility := client.GetCurrentResourceVisibility(MinIOBucket{BucketName: "bucket", Owner: "owner"})
			if visibility != tt.expected {
				t.Fatalf("expected visibility %q, got %q", tt.expected, visibility)
			}
		})
	}
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

func TestSetAndUnsetPolicies(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	mock := newMinioMock()
	server := httptest.NewServer(mock)
	defer server.Close()

	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}

	client, err := MakeMinIOAdminClient(&cfg)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	bucket := MinIOBucket{
		BucketName:   "bucket",
		Owner:        "owner",
		Visibility:   RESTRICTED,
		AllowedUsers: []string{"alice"},
	}

	if err := client.SetPolicies(bucket); err != nil {
		t.Fatalf("unexpected error setting policies: %v", err)
	}
	if len(mock.policies["owner"]) == 0 || len(mock.policies["bucket"]) == 0 {
		t.Fatalf("expected policies to be created: %#v", mock.policies)
	}

	if err := client.UnsetPolicies(bucket); err != nil {
		t.Fatalf("unexpected error unsetting policies: %v", err)
	}
	if _, ok := mock.policies["bucket"]; ok {
		t.Fatalf("expected bucket policy to be removed, got %#v", mock.policies)
	}
}

func TestUpdateServiceGroup(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	mock := newMinioMock()
	mock.groupMembers["bucket"] = []string{"alice", "bob"}
	server := httptest.NewServer(mock)
	defer server.Close()

	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}

	client, err := MakeMinIOAdminClient(&cfg)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	if err := client.UpdateServiceGroup("bucket", []string{"alice", "carol"}); err != nil {
		t.Fatalf("unexpected error updating service group: %v", err)
	}

	members := mock.groupMembers["bucket"]
	if len(members) != 2 || members[0] != "alice" || members[1] != "carol" {
		t.Fatalf("unexpected group members: %v", members)
	}
}

func TestRegisterAndRemoveWebhook(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	mock := newMinioMock()
	server := httptest.NewServer(mock)
	defer server.Close()

	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
		Name:        "oscar",
		Namespace:   "default",
		ServicePort: 8080,
	}

	client, err := MakeMinIOAdminClient(&cfg)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	if err := client.RegisterWebhook("bucket", "token"); err != nil {
		t.Fatalf("unexpected error registering webhook: %v", err)
	}
	if len(mock.configWrites) == 0 {
		t.Fatalf("expected config writes to be recorded")
	}

	if err := client.RemoveWebhook("bucket"); err != nil {
		t.Fatalf("unexpected error removing webhook: %v", err)
	}
}
