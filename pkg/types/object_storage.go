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
	"context"
	"fmt"
	"time"

	"github.com/minio/madmin-go"
)

const (
	ObjectStorageMinIO  = "minio"
	ObjectStorageRustFS = "rustfs"
	RustFSIAMAttempts   = 240
	RustFSIAMRetryDelay = 500 * time.Millisecond
)

type MinIOIAM struct {
	client *MinIOAdminClient
}

func NewMINIOFSIAM(minIOAdminClient *MinIOAdminClient) *MinIOIAM {
	return &MinIOIAM{client: minIOAdminClient}
}

func (m *MinIOIAM) CreateUser(ctx context.Context, accessKey, secretKey string) error {
	return createIAMUser(ctx, m.client.GetAdminClient(), accessKey, secretKey, "enable")
}

func (m *MinIOIAM) CreateGroup(ctx context.Context, group string) error {
	return createIAMGroup(ctx, m.client.GetAdminClient(), group, "enable")
}

func (m *MinIOIAM) UpdateGroupMembers(ctx context.Context, group string, users []string, remove bool) error {
	return updateIAMGroupMembers(ctx, m.client.GetAdminClient(), group, users, remove, "enable")
}

func (m *MinIOIAM) GetClient(ctx context.Context) *MinIOAdminClient {
	return m.client
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
