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
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/grycap/oscar/v4/pkg/types"
)

// TestRustFSIAMIntegration exercises the RustFS admin API against a live
// instance. It is skipped unless all RUSTFS_TEST_* variables are provided.
func TestRustFSIAMIntegration(t *testing.T) {
	endpoint := os.Getenv("RUSTFS_TEST_ENDPOINT")
	adminAccessKey := os.Getenv("RUSTFS_TEST_ACCESS_KEY")
	adminSecretKey := os.Getenv("RUSTFS_TEST_SECRET_KEY")
	if endpoint == "" || adminAccessKey == "" || adminSecretKey == "" {
		t.Skip("live RustFS credentials not configured")
	}

	cfg := &types.Config{
		ObjectStorageType: ObjectStorageRustFS,
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  endpoint,
			AccessKey: adminAccessKey,
			SecretKey: adminSecretKey,
			Region:    "us-east-1",
			Verify:    true,
		},
		Name:        "oscar",
		Namespace:   "oscar",
		ServicePort: 8080,
	}

	iam, err := MakeObjectStorageIAM(cfg)
	if err != nil {
		t.Fatalf("creating RustFS IAM adapter: %v", err)
	}
	rustFS, ok := iam.(*rustFSIAM)
	if !ok {
		t.Fatalf("expected RustFS IAM adapter, got %T", iam)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	accessKey := "oscit" + strconv.FormatInt(time.Now().UnixNano(), 36)
	secretKey := fmt.Sprintf("OscarIntegrationSecret%d", time.Now().UnixNano())

	if err := iam.CreateGroup(ctx, ALL_USERS_GROUP); err != nil {
		t.Fatalf("ensuring default group: %v", err)
	}
	if err := iam.CreateUser(ctx, accessKey, secretKey); err != nil {
		t.Fatalf("creating temporary RustFS user: %v", err)
	}
	users, err := rustFS.client.ListUsers(ctx)
	if err != nil {
		t.Fatalf("listing RustFS users: %v", err)
	}
	userInfo, found := users[accessKey]
	if !found {
		t.Fatalf("temporary RustFS user %q not found in user list", accessKey)
	}
	if userInfo.Status != "enabled" {
		t.Fatalf("temporary RustFS user has status %q", userInfo.Status)
	}
	foundDefaultGroup := false
	for _, group := range userInfo.MemberOf {
		if group == ALL_USERS_GROUP {
			foundDefaultGroup = true
			break
		}
	}
	if !foundDefaultGroup {
		t.Fatalf("temporary RustFS user is not a member of %s", ALL_USERS_GROUP)
	}
}
