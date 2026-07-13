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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/minio/madmin-go"
)

const (
	ObjectStorageMinIO  = "minio"
	ObjectStorageRustFS = "rustfs"
	rustFSIAMAttempts   = 240
	rustFSIAMRetryDelay = 500 * time.Millisecond
)

// ObjectStorageIAM contains the identity operations OSCAR needs from its
// S3-compatible object storage. Keeping this contract separate prevents the
// authentication middleware from depending directly on a vendor admin client.
type ObjectStorageIAM interface {
	CreateUser(ctx context.Context, accessKey, secretKey string) error
	CreateGroup(ctx context.Context, group string) error
	UpdateGroupMembers(ctx context.Context, group string, users []string, remove bool) error
}

type minIOIAM struct {
	client *madmin.AdminClient
}

type rustFSIAM struct {
	client     *madmin.AdminClient
	endpoint   *url.URL
	httpClient *http.Client
	signer     *v4.Signer
	region     string
}

// MakeObjectStorageIAM creates the IAM implementation selected by
// OBJECT_STORAGE_TYPE. RustFS currently exposes the MinIO-compatible admin
// operations needed by OSCAR, but has its own adapter so vendor differences can
// be handled without changing the authentication flow.
func MakeObjectStorageIAM(cfg *types.Config) (ObjectStorageIAM, error) {
	adminClient, err := MakeMinIOAdminClient(cfg)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(strings.TrimSpace(cfg.ObjectStorageType)) {
	case "", ObjectStorageMinIO:
		return &minIOIAM{client: adminClient.adminClient}, nil
	case ObjectStorageRustFS:
		return newRustFSIAM(cfg, adminClient.adminClient)
	default:
		return nil, fmt.Errorf("unsupported object storage type %q", cfg.ObjectStorageType)
	}
}

func newRustFSIAM(cfg *types.Config, adminClient *madmin.AdminClient) (*rustFSIAM, error) {
	endpoint, err := url.Parse(cfg.MinIOProvider.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing RustFS endpoint: %w", err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !cfg.MinIOProvider.Verify {
		// #nosec G402 -- explicitly controlled by MINIO_TLS_VERIFY.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	region := cfg.MinIOProvider.Region
	if region == "" {
		region = "us-east-1"
	}
	return &rustFSIAM{
		client:     adminClient,
		endpoint:   endpoint,
		httpClient: &http.Client{Transport: transport, Timeout: 30 * time.Second},
		signer: v4.NewSigner(credentials.NewStaticCredentials(
			cfg.MinIOProvider.AccessKey,
			cfg.MinIOProvider.SecretKey,
			"",
		)),
		region: region,
	}, nil
}

func createIAMUser(ctx context.Context, client *madmin.AdminClient, accessKey, secretKey, groupStatus string) error {
	if err := client.AddUser(ctx, accessKey, secretKey); err != nil {
		return fmt.Errorf("creating object-storage user: %w", err)
	}
	if err := updateIAMGroupMembers(ctx, client, ALL_USERS_GROUP, []string{accessKey}, false, groupStatus); err != nil {
		_ = client.RemoveUser(ctx, accessKey)
		return fmt.Errorf("adding object-storage user to default group: %w", err)
	}
	return nil
}

func createIAMGroup(ctx context.Context, client *madmin.AdminClient, group, status string) error {
	return updateIAMGroupMembers(ctx, client, group, []string{}, false, status)
}

func updateIAMGroupMembers(ctx context.Context, client *madmin.AdminClient, group string, users []string, remove bool, status string) error {
	request := madmin.GroupAddRemove{
		Group:    group,
		Members:  users,
		Status:   madmin.GroupStatus(status),
		IsRemove: remove,
	}
	if err := client.UpdateGroupMembers(ctx, request); err != nil {
		return fmt.Errorf("updating object-storage group %s: %w", group, err)
	}
	return nil
}

func (m *minIOIAM) CreateUser(ctx context.Context, accessKey, secretKey string) error {
	return createIAMUser(ctx, m.client, accessKey, secretKey, "enable")
}

func (m *minIOIAM) CreateGroup(ctx context.Context, group string) error {
	return createIAMGroup(ctx, m.client, group, "enable")
}

func (m *minIOIAM) UpdateGroupMembers(ctx context.Context, group string, users []string, remove bool) error {
	return updateIAMGroupMembers(ctx, m.client, group, users, remove, "enable")
}

func (r *rustFSIAM) CreateUser(ctx context.Context, accessKey, secretKey string) error {
	body := struct {
		SecretKey string `json:"secretKey"`
		Status    string `json:"status"`
	}{SecretKey: secretKey, Status: "enabled"}
	err := r.request(ctx, http.MethodPut, "/add-user", url.Values{"accessKey": []string{accessKey}}, body)
	if err != nil && strings.Contains(err.Error(), "decrypt MinIO admin payload") {
		err = r.client.AddUser(ctx, accessKey, secretKey)
	}
	if err != nil {
		return fmt.Errorf("creating RustFS user: %w", err)
	}

	var userErr error
	consecutiveSuccesses := 0
	for attempt := 0; attempt < rustFSIAMAttempts; attempt++ {
		var users map[string]madmin.UserInfo
		users, userErr = r.client.ListUsers(ctx)
		_, found := users[accessKey]
		if userErr == nil && found {
			consecutiveSuccesses++
			if consecutiveSuccesses == 3 {
				break
			}
		} else {
			consecutiveSuccesses = 0
			if userErr == nil {
				userErr = fmt.Errorf("user not present in RustFS user list")
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for RustFS user: %w", ctx.Err())
		case <-time.After(rustFSIAMRetryDelay):
		}
	}
	if userErr != nil || consecutiveSuccesses < 3 {
		_ = r.client.RemoveUser(ctx, accessKey)
		if userErr == nil {
			return fmt.Errorf("RustFS user did not remain visible after creation")
		}
		return fmt.Errorf("RustFS user was not visible after creation: %w", userErr)
	}

	var groupErr error
	for attempt := 0; attempt < rustFSIAMAttempts; attempt++ {
		groupErr = r.UpdateGroupMembers(ctx, ALL_USERS_GROUP, []string{accessKey}, false)
		if groupErr == nil {
			var users map[string]madmin.UserInfo
			users, groupErr = r.client.ListUsers(ctx)
			if groupErr == nil {
				for _, group := range users[accessKey].MemberOf {
					if group == ALL_USERS_GROUP {
						return nil
					}
				}
				groupErr = fmt.Errorf("user is not yet a member of %s", ALL_USERS_GROUP)
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("adding RustFS user to default group: %w", ctx.Err())
		case <-time.After(rustFSIAMRetryDelay):
		}
	}
	return fmt.Errorf("adding RustFS user to default group: %w", groupErr)
}

func (r *rustFSIAM) CreateGroup(ctx context.Context, group string) error {
	return r.UpdateGroupMembers(ctx, group, []string{}, false)
}

func (r *rustFSIAM) UpdateGroupMembers(ctx context.Context, group string, users []string, remove bool) error {
	body := struct {
		Group    string   `json:"group"`
		Members  []string `json:"members"`
		IsRemove bool     `json:"isRemove"`
		Status   string   `json:"groupStatus"`
	}{Group: group, Members: users, IsRemove: remove, Status: "enabled"}
	return r.request(ctx, http.MethodPut, "/update-group-members", nil, body)
}

func (r *rustFSIAM) request(ctx context.Context, method, path string, query url.Values, body any) error {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding RustFS admin request: %w", err)
		}
	}
	return r.requestPayload(ctx, method, "/minio/admin/v3"+path, query, payload, "application/json")
}

func (r *rustFSIAM) requestPayload(ctx context.Context, method, path string, query url.Values, payload []byte, contentType string) error {
	_, err := r.requestPayloadResponse(ctx, method, path, query, payload, contentType)
	return err
}

func (r *rustFSIAM) requestPayloadResponse(ctx context.Context, method, path string, query url.Values, payload []byte, contentType string) ([]byte, error) {
	requestURL := *r.endpoint
	requestURL.Path = strings.TrimRight(requestURL.Path, "/") + path
	requestURL.RawQuery = query.Encode()

	reader := bytes.NewReader(payload)
	req, err := http.NewRequestWithContext(ctx, method, requestURL.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("creating RustFS admin request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if _, err := r.signer.Sign(req, reader, "s3", r.region, time.Now()); err != nil {
		return nil, fmt.Errorf("signing RustFS admin request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending RustFS admin request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("RustFS admin API returned %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
	}
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading RustFS admin response: %w", err)
	}
	return responseBody, nil
}
