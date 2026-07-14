package buckets

import (
	"strings"

	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
)

const (
	bucketVisibilityTag   = "visibility"
	bucketAllowedUsersTag = "allowed_users"
	bucketStorageQuotaTag = "storage_quota"
	bucketAllowedUsersSep = " "
)

func isRustFSConfig(cfg *types.Config) bool {
	return cfg != nil && strings.EqualFold(strings.TrimSpace(cfg.ObjectStorageType), utils.ObjectStorageRustFS)
}

func normalizeBucketVisibility(visibility string) string {
	switch strings.ToLower(strings.TrimSpace(visibility)) {
	case utils.PUBLIC:
		return utils.PUBLIC
	case utils.RESTRICTED:
		return utils.RESTRICTED
	default:
		return utils.PRIVATE
	}
}

func bucketTags(bucket utils.MinIOBucket, ownerName string) map[string]string {
	tags := map[string]string{
		"owner":             bucket.Owner,
		"owner_name":        ownerName,
		bucketVisibilityTag: normalizeBucketVisibility(bucket.Visibility),
	}
	if len(bucket.AllowedUsers) > 0 {
		tags[bucketAllowedUsersTag] = strings.Join(bucket.AllowedUsers, bucketAllowedUsersSep)
	}
	if bucket.StorageQuota != nil && bucket.StorageQuota.Max != "" {
		tags[bucketStorageQuotaTag] = bucket.StorageQuota.Max
	}
	return tags
}

func storageQuotaFromTags(metadata map[string]string) *utils.MinIOQuota {
	if metadata == nil {
		return nil
	}
	max := strings.TrimSpace(metadata[bucketStorageQuotaTag])
	if max == "" {
		return nil
	}
	return &utils.MinIOQuota{Max: max, Source: "tag"}
}

func rustFSBucketStorageQuota(adminClient *utils.MinIOAdminClient, bucketName string, metadata map[string]string) *utils.MinIOQuota {
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
	return &utils.MinIOQuota{Max: "0", Source: "unsupported"}
}

func visibilityFromTags(metadata map[string]string) string {
	if metadata == nil {
		return utils.PRIVATE
	}
	return normalizeBucketVisibility(metadata[bucketVisibilityTag])
}

func allowedUsersFromTags(metadata map[string]string) []string {
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

func userAllowedByTags(uid string, metadata map[string]string) bool {
	if uid == types.DefaultOwner {
		return true
	}
	if metadata["owner"] == uid {
		return true
	}
	if visibilityFromTags(metadata) == utils.PUBLIC {
		return true
	}
	for _, allowedUser := range allowedUsersFromTags(metadata) {
		if allowedUser == uid {
			return true
		}
	}
	return false
}
