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
	"strings"

	"github.com/grycap/oscar/v4/pkg/types"
)

const (
	bucketVisibilityTag   = "visibility"
	bucketAllowedUsersTag = "allowed_users"
	bucketStorageQuotaTag = "storage_quota"
	bucketAllowedUsersSep = " "
)

func IsRustFSConfig(cfg *types.Config) bool {
	return cfg != nil && strings.EqualFold(strings.TrimSpace(cfg.ObjectStorageType), types.ObjectStorageRustFS)
}

func VisibilityFromObjectStorageTags(metadata map[string]string) string {
	if metadata == nil {
		return types.PRIVATE
	}
	return normalizeObjectStorageVisibility(metadata["visibility"])
}

func normalizeObjectStorageVisibility(visibility string) string {
	switch strings.ToLower(strings.TrimSpace(visibility)) {
	case types.PUBLIC:
		return types.PUBLIC
	case types.RESTRICTED:
		return types.RESTRICTED
	default:
		return types.PRIVATE
	}
}

func BucketTags(bucket types.MinIOBucket, ownerName string) map[string]string {
	tags := map[string]string{
		"owner":             bucket.Owner,
		"owner_name":        ownerName,
		bucketVisibilityTag: normalizeObjectStorageVisibility(bucket.Visibility),
	}
	if len(bucket.AllowedUsers) > 0 {
		tags[bucketAllowedUsersTag] = strings.Join(bucket.AllowedUsers, bucketAllowedUsersSep)
	}
	if bucket.StorageQuota != nil && bucket.StorageQuota.Max != "" {
		tags[bucketStorageQuotaTag] = bucket.StorageQuota.Max
	}
	return tags
}

func storageQuotaFromTags(metadata map[string]string) *types.MinIOQuota {
	if metadata == nil {
		return nil
	}
	max := strings.TrimSpace(metadata[bucketStorageQuotaTag])
	if max == "" {
		return nil
	}
	return &types.MinIOQuota{Max: max, Source: "tag"}
}

func RustFSBucketStorageQuota(adminClient *types.MinIOAdminClient, bucketName string, metadata map[string]string) *types.MinIOQuota {
	quota, err := adminClient.GetBucketStorageQuota(bucketName)
	if err == nil && quota != nil && quota.Max != "" && quota.Source != "unsupported" && quota.Source != "unset" {
		return quota
	}
	if taggedQuota := storageQuotaFromTags(metadata); taggedQuota != nil {
		return taggedQuota
	}
	if err == nil && quota != nil {
		return quota
	}
	return &types.MinIOQuota{Max: "0", Source: "unsupported"}
}

func AllowedUsersFromTags(metadata map[string]string) []string {
	if metadata == nil {
		return nil
	}
	raw := strings.TrimSpace(metadata[bucketAllowedUsersTag])
	if raw == "" {
		return nil
	}
	values := strings.Fields(strings.ReplaceAll(raw, ",", bucketAllowedUsersSep))
	users := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			users = append(users, value)
		}
	}
	return users
}

func UserAllowedByTags(cfg *types.Config, uid string, metadata map[string]string) bool {
	if uid == cfg.Username {
		return true
	}
	if metadata["owner"] == uid {
		return true
	}
	if VisibilityFromObjectStorageTags(metadata) == types.PUBLIC {
		return true
	}
	for _, allowedUser := range AllowedUsersFromTags(metadata) {
		if allowedUser == uid {
			return true
		}
	}
	return false
}
