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

package objectstorage

import (
	"context"
	"fmt"
	"strings"

	"github.com/grycap/oscar/v4/pkg/types"
)

// MakeObjectStorageIAM creates the IAM implementation selected by
// OBJECT_STORAGE_TYPE. RustFS currently exposes the MinIO-compatible admin
// operations needed by OSCAR, but has its own adapter so vendor differences can
// be handled without changing the authentication flow.
func MakeObjectStorageIAM(cfg *types.Config) (ObjectStorageIAM, error) {
	minIOAdminClient, err := types.MakeMinIOAdminClient(cfg)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(strings.TrimSpace(cfg.ObjectStorageType)) {
	case "", types.ObjectStorageMinIO:
		return types.NewMINIOFSIAM(minIOAdminClient), nil
	case types.ObjectStorageRustFS:
		return types.NewRustFSIAM(cfg, minIOAdminClient)
	default:
		return nil, fmt.Errorf("unsupported object storage type %q", cfg.ObjectStorageType)
	}
}

// ObjectStorageIAM contains the identity operations OSCAR needs from its
// S3-compatible object storage. Keeping this contract separate prevents the
// authentication middleware from depending directly on a vendor admin client.
type ObjectStorageIAM interface {
	CreateUser(ctx context.Context, accessKey, secretKey string) error
	CreateGroup(ctx context.Context, group string) error
	UpdateGroupMembers(ctx context.Context, group string, users []string, remove bool) error
	GetClient(ctx context.Context) *types.MinIOAdminClient
}
