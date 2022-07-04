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

package auth

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v2/pkg/types"
)

// GetAuthMiddleware returns the appropriate gin auth middleware
func GetAuthMiddleware(cfg *types.Config) gin.HandlerFunc {
	if !cfg.OIDCEnable {
		return gin.BasicAuth(gin.Accounts{
			// Use the config's username and password for basic auth
			cfg.Username: cfg.Password,
		})
	}
	return CustomAuth(cfg)
}

// CustomAuth returns a custom auth handler (gin middleware)
func CustomAuth(cfg *types.Config) gin.HandlerFunc {
	basicAuthHandler := gin.BasicAuth(gin.Accounts{
		// Use the config's username and password for basic auth
		cfg.Username: cfg.Password,
	})

	oidcHandler := getOIDCMiddleware(cfg.OIDCIssuer, cfg.OIDCSubject, cfg.OIDCGroups)

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			oidcHandler(c)
		} else {
			basicAuthHandler(c)
		}
	}
}
