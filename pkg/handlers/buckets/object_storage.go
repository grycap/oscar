package buckets

import (
	"strings"

	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
)

const (
	bucketVisibilityTag   = "visibility"
	bucketAllowedUsersTag = "allowed_users"
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
	return map[string]string{
		"owner":               bucket.Owner,
		"owner_name":          ownerName,
		bucketVisibilityTag:   normalizeBucketVisibility(bucket.Visibility),
		bucketAllowedUsersTag: strings.Join(bucket.AllowedUsers, ","),
	}
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
	values := strings.Split(raw, ",")
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
