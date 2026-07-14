package handlers

import (
	"strings"

	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
)

func isRustFSConfig(cfg *types.Config) bool {
	return cfg != nil && strings.EqualFold(strings.TrimSpace(cfg.ObjectStorageType), utils.ObjectStorageRustFS)
}

func visibilityFromObjectStorageTags(metadata map[string]string) string {
	if metadata == nil {
		return utils.PRIVATE
	}
	switch strings.ToLower(strings.TrimSpace(metadata["visibility"])) {
	case utils.PUBLIC:
		return utils.PUBLIC
	case utils.RESTRICTED:
		return utils.RESTRICTED
	default:
		return utils.PRIVATE
	}
}
