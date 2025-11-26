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
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awscreds "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"slices"

	"github.com/grycap/oscar/v3/pkg/testsupport"
	"github.com/grycap/oscar/v3/pkg/types"
	madmin "github.com/minio/madmin-go"
	"github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
)

type minioMock struct {
	policies     map[string][]string
	groupMembers map[string][]string
	bucketTags   map[string]map[string]string
	configWrites []string
}

func newMinioMock() *minioMock {
	return &minioMock{
		policies:     map[string][]string{},
		groupMembers: map[string][]string{},
		bucketTags:   map[string]map[string]string{},
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
	case strings.HasPrefix(r.URL.Path, "/minio/admin/v3/service/restart"):
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
		if _, ok := r.URL.Query()["tagging"]; ok || strings.Contains(r.URL.RawQuery, "tagging") {
			bucket := strings.TrimPrefix(r.URL.Path, "/")
			switch r.Method {
			case http.MethodPut:
				body, _ := io.ReadAll(r.Body)
				m.bucketTags[bucket] = parseBucketTags(body)
				w.WriteHeader(http.StatusNoContent)
			case http.MethodGet:
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(buildBucketTagsResponse(m.bucketTags[bucket])))
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"Status":"success"}`))
		}
	}
}

type bucketTagsXML struct {
	TagSet []struct {
		Key   string `xml:"Key"`
		Value string `xml:"Value"`
	} `xml:"TagSet>Tag"`
}

type fakeMinioRoundTripper struct {
	tags  map[string]map[string]string
	calls int
}

func (f *fakeMinioRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	f.calls++
	bucket := strings.TrimPrefix(req.URL.Path, "/")
	if strings.Contains(req.URL.RawQuery, "tagging") {
		switch req.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(req.Body)
			if f.tags == nil {
				f.tags = map[string]map[string]string{}
			}
			f.tags[bucket] = parseBucketTags(body)
			return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		case http.MethodGet:
			resp := buildBucketTagsResponse(f.tags[bucket])
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(resp)), Header: http.Header{}}, nil
		default:
			return &http.Response{StatusCode: http.StatusMethodNotAllowed, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		}
	}

	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"Status":"success"}`)), Header: http.Header{}}, nil
}

func parseBucketTags(data []byte) map[string]string {
	var tagSet bucketTagsXML
	result := map[string]string{}
	if err := xml.Unmarshal(data, &tagSet); err != nil {
		return result
	}
	for _, tag := range tagSet.TagSet {
		result[tag.Key] = tag.Value
	}
	return result
}

func buildBucketTagsResponse(tags map[string]string) string {
	builder := strings.Builder{}
	builder.WriteString(`<Tagging><TagSet>`)
	for k, v := range tags {
		builder.WriteString(fmt.Sprintf("<Tag><Key>%s</Key><Value>%s</Value></Tag>", k, v))
	}
	builder.WriteString(`</TagSet></Tagging>`)
	return builder.String()
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

func TestBucketMetadataHelpers(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	rt := &fakeMinioRoundTripper{tags: map[string]map[string]string{}}
	simpleClient, err := minio.New("localhost:9000", &minio.Options{
		Creds:     miniocreds.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure:    false,
		Transport: rt,
	})
	if err != nil {
		t.Fatalf("unexpected error creating minio client: %v", err)
	}

	client := &MinIOAdminClient{simpleClient: simpleClient}

	if err := client.SetTags("bucket", map[string]string{"uid": "alice"}); err == nil {
		t.Fatalf("expected error setting tags with fake transport")
	}

	if _, err := client.GetTaggedMetadata("bucket"); err == nil {
		t.Fatalf("expected error getting tags with fake transport")
	}
}

func TestGetBucketMembers(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	mock := newMinioMock()
	mock.groupMembers["bucket"] = []string{"user1", "user2"}
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

	members, err := client.GetBucketMembers("bucket")
	if err != nil {
		t.Fatalf("unexpected error getting bucket members: %v", err)
	}
	if len(members) != 2 || members[0] != "user1" {
		t.Fatalf("unexpected members: %v", members)
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

func TestRemoveFromPolicy(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	mock := newMinioMock()
	mock.policies["owner"] = []string{"arn:aws:s3:::bucket/*", "arn:aws:s3:::other/*"}
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

	if err := client.RemoveFromPolicy("bucket", "owner", false); err != nil {
		t.Fatalf("unexpected error removing from policy: %v", err)
	}
	if slices.Contains(mock.policies["owner"], "arn:aws:s3:::bucket/*") {
		t.Fatalf("expected bucket resource to be removed from policy")
	}
}

func TestCreateAllUsersGroup(t *testing.T) {
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

	if err := client.CreateAllUsersGroup(); err != nil {
		t.Fatalf("unexpected error creating group: %v", err)
	}

	if _, ok := mock.groupMembers[ALL_USERS_GROUP]; !ok {
		t.Fatalf("expected all users group to be created")
	}
}

func TestGetSimpleClientAndRestartServer(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	cfg, server := createMinIOConfig()
	defer server.Close()

	client, err := MakeMinIOAdminClient(&cfg)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	if client.GetSimpleClient() == nil {
		t.Fatalf("expected simple client to be initialized")
	}

	// RestartServer includes a small wait to verify the server is reachable again.
	if err := client.RestartServer(); err != nil {
		t.Fatalf("unexpected error restarting server: %v", err)
	}
}

// S3 path helpers (merged from minio_s3_test.go)

type fakeS3Client struct {
	client        *s3.S3
	buckets       map[string]struct{}
	notifications map[string][]*s3.QueueConfiguration
}

func newFakeS3Client(t *testing.T) *fakeS3Client {
	t.Helper()

	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Credentials: awscreds.NewStaticCredentials("ak", "sk", ""),
	}))

	client := s3.New(sess)
	client.Handlers.Send.Clear()
	client.Handlers.Unmarshal.Clear()
	client.Handlers.UnmarshalMeta.Clear()
	client.Handlers.UnmarshalError.Clear()
	client.Handlers.ValidateResponse.Clear()

	f := &fakeS3Client{
		client:        client,
		buckets:       map[string]struct{}{},
		notifications: map[string][]*s3.QueueConfiguration{},
	}

	client.Handlers.Send.PushBack(func(r *request.Request) {
		r.HTTPResponse = &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     http.Header{},
		}

		switch r.Operation.Name {
		case "CreateBucket":
			name := aws.StringValue(r.Params.(*s3.CreateBucketInput).Bucket)
			if _, ok := f.buckets[name]; ok {
				r.Error = awserr.New(s3.ErrCodeBucketAlreadyOwnedByYou, "bucket exists", nil)
				return
			}
			f.buckets[name] = struct{}{}
			r.Data = &s3.CreateBucketOutput{}
		case "PutObject":
			r.Data = &s3.PutObjectOutput{}
		case "GetBucketNotificationConfiguration":
			input := r.Params.(*s3.GetBucketNotificationConfigurationRequest)
			bucket := aws.StringValue(input.Bucket)
			r.Data = &s3.NotificationConfiguration{
				QueueConfigurations: f.notifications[bucket],
			}
		case "PutBucketNotificationConfiguration":
			input := r.Params.(*s3.PutBucketNotificationConfigurationInput)
			bucket := aws.StringValue(input.Bucket)
			f.notifications[bucket] = input.NotificationConfiguration.QueueConfigurations
			r.Data = &s3.PutBucketNotificationConfigurationOutput{}
		case "ListObjects":
			r.Data = &s3.ListObjectsOutput{}
		case "ListObjectsV2":
			r.Data = &s3.ListObjectsV2Output{}
		case "DeleteObjects":
			r.Data = &s3.DeleteObjectsOutput{}
		case "DeleteBucket":
			name := aws.StringValue(r.Params.(*s3.DeleteBucketInput).Bucket)
			delete(f.buckets, name)
			delete(f.notifications, name)
			r.Data = &s3.DeleteBucketOutput{}
		default:
			r.Error = fmt.Errorf("unexpected operation %s", r.Operation.Name)
		}
	})

	return f
}

func TestCreateS3PathWithWebhook(t *testing.T) {
	fake := newFakeS3Client(t)
	client := &MinIOAdminClient{}

	err := client.CreateS3PathWithWebhook(fake.client, []string{"bucket", "folder"}, "arn:aws:sqs:us-east-1:1234:queue", false)
	if err != nil {
		t.Fatalf("unexpected error creating path with webhook: %v", err)
	}

	if _, ok := fake.buckets["bucket"]; !ok {
		t.Fatalf("expected bucket to be created")
	}

	ncfg := fake.notifications["bucket"]
	if len(ncfg) != 1 {
		t.Fatalf("expected a single notification configuration, got %d", len(ncfg))
	}
}

func TestCreateS3PathWithWebhookMissingFolder(t *testing.T) {
	fake := newFakeS3Client(t)
	client := &MinIOAdminClient{}

	err := client.CreateS3PathWithWebhook(fake.client, []string{"bucket"}, "arn:aws:sqs:us-east-1:1234:queue", false)
	if err == nil {
		t.Fatalf("expected error for missing folder in path")
	}
}

func TestCreateS3PathBucketExists(t *testing.T) {
	fake := newFakeS3Client(t)
	client := &MinIOAdminClient{}
	fake.buckets["bucket"] = struct{}{}

	if err := client.CreateS3Path(fake.client, []string{"bucket"}, true); err != nil {
		t.Fatalf("unexpected error when bucket already exists: %v", err)
	}
}

func TestCreateS3PathDuplicateBucket(t *testing.T) {
	fake := newFakeS3Client(t)
	client := &MinIOAdminClient{}
	fake.buckets["bucket"] = struct{}{}

	err := client.CreateS3Path(fake.client, []string{"bucket"}, false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate bucket error, got %v", err)
	}
}

func TestDisableInputNotificationsRemovesQueue(t *testing.T) {
	fake := newFakeS3Client(t)
	fake.notifications["bucket"] = []*s3.QueueConfiguration{
		{QueueArn: aws.String("arn:aws:sqs:us-east-1:1234:queue")},
	}

	if err := disableInputNotifications(fake.client, "arn:aws:sqs:us-east-1:1234:queue", "bucket"); err != nil {
		t.Fatalf("unexpected error disabling notifications: %v", err)
	}

	if len(fake.notifications["bucket"]) != 0 {
		t.Fatalf("expected notifications to be cleared, got %v", fake.notifications["bucket"])
	}
}

func TestDeleteBucketRemovesResources(t *testing.T) {
	fake := newFakeS3Client(t)
	client := &MinIOAdminClient{}
	fake.buckets["bucket"] = struct{}{}

	if err := client.DeleteBucket(fake.client, "bucket"); err != nil {
		t.Fatalf("unexpected error deleting bucket: %v", err)
	}

	if _, ok := fake.buckets["bucket"]; ok {
		t.Fatalf("expected bucket to be deleted")
	}
}
