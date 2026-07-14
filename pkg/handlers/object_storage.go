package handlers

import (
	"strings"

	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
)

func isRustFSConfig(cfg *types.Config) bool {
	return cfg != nil && strings.EqualFold(strings.TrimSpace(cfg.ObjectStorageType), utils.ObjectStorageRustFS)
}
