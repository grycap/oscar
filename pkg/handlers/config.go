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

package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
)

var (
	getUIDFromContextFn                = auth.GetUIDFromContext
	getMultitenancyConfigFromContextFn = auth.GetMultitenancyConfigFromContext
)

type configForUser struct {
	Cfg                      *types.Config        `json:"config"`
	MinIOProvider            *types.MinIOProvider `json:"minio_provider"`
	AllowedImageRepositories []string             `json:"allowed_image_repositories"`
}

// MakeConfigHandler makes a handler for getting server's configuration
func MakeConfigHandler(cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfgSnapshot := cfg.Clone()
		if cfgSnapshot == nil {
			c.String(http.StatusInternalServerError, "configuration is not available")
			return
		}

		allowedRepos := cfgSnapshot.GetAllowedImageRepositories()
		if allowedRepos == nil {
			allowedRepos = []string{}
		}
		cfgSnapshot.AllowedImageRepositories = allowedRepos

		// Return configForUser
		var conf configForUser
		minIOProvider := cfgSnapshot.MinIOProvider
		authHeader := c.GetHeader("Authorization")
		if len(strings.Split(authHeader, "Bearer")) == 1 {
			conf = configForUser{cfgSnapshot, minIOProvider, allowedRepos}
		} else {

			// Get MinIO credentials from k8s secret for user

			uid, err := getUIDFromContextFn(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
			}

			mc, err := getMultitenancyConfigFromContextFn(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
			}

			ak, sk, err := mc.GetUserCredentials(uid)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
			}

			userMinIOProvider := &types.MinIOProvider{
				Endpoint:  minIOProvider.Endpoint,
				Verify:    minIOProvider.Verify,
				AccessKey: ak,
				SecretKey: sk,
				Region:    minIOProvider.Region,
			}

			conf = configForUser{cfgSnapshot, userMinIOProvider, allowedRepos}
		}

		c.JSON(http.StatusOK, conf)
	}
}

// MakeConfigUpdateHandler allows updating mutable configuration fields (Basic auth only).
func MakeConfigUpdateHandler(cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			c.String(http.StatusForbidden, "Updating the system configuration is restricted to the oscar user")
			return
		}

		userValue, userExists := c.Get(gin.AuthUserKey)
		user, ok := userValue.(string)
		if !userExists || !ok || user != "oscar" {
			c.String(http.StatusForbidden, "Updating the system configuration is restricted to the oscar user")
			return
		}

		var payload types.UpdateConfigRequest
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The configuration update request is not valid: %v", err))
			return
		}

		if err := cfg.SetAllowedImageRepositories(payload.AllowedImageRepositories); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The configuration update request is not valid: %v", err))
			return
		}

		cfgSnapshot := cfg.Clone()
		if cfgSnapshot == nil {
			c.String(http.StatusInternalServerError, "configuration is not available")
			return
		}

		allowedRepos := cfgSnapshot.GetAllowedImageRepositories()
		if allowedRepos == nil {
			allowedRepos = []string{}
		}
		cfgSnapshot.AllowedImageRepositories = allowedRepos

		conf := configForUser{
			Cfg:                      cfgSnapshot,
			MinIOProvider:            cfgSnapshot.MinIOProvider,
			AllowedImageRepositories: allowedRepos,
		}

		c.JSON(http.StatusOK, conf)
	}
}
