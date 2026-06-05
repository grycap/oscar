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
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
)

// GetServicePermissionsMiddleware returns a gin middleware that checks if the request has permissions to access the service
// STRICTLY after the request is authenticated, either by service token, OIDC or basic auth. It checks the service visibility and permissions according to the following rules:
// - If the service is public, it allows access to everyone.
// - If the service is private, it allows access only to the owner of the service.
// - If the service is restricted, it allows access to the owner and the users in the allowed users list.
func GetServicePermissionsMiddleware(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If autenticated with service token
		if isServiceToken, exists := c.Get(isServiceTokenKey); exists {
			if validServiceToken, ok := isServiceToken.(bool); ok && validServiceToken {
				c.Next()
				return
			}
		}

		// Get service if exists
		service, err := back.ReadService("", c.Param("serviceName"))
		if err != nil {
			// Check if error is caused because the service is not found
			if errors.IsNotFound(err) || errors.IsGone(err) {
				c.AbortWithStatus(http.StatusNotFound)
			} else {
				c.AbortWithStatus(http.StatusInternalServerError)
			}
			return
		}

		// If admin
		if _, ok := c.Get(gin.AuthUserKey); ok && isBasicAuth(c) {
			c.Next()
			return
		}

		// If authenticated with OIDC
		// Check permissions to access the service
		if _, ok := isAuthBearer(c); ok {
			uid, err := GetUIDFromContext(c)
			if err != nil {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}

			if hasPermission(service, uid) {
				c.Next()
				return
			}
		}

		c.AbortWithStatus(http.StatusForbidden)
		return
	}
}

func hasPermission(service *types.Service, uid string) bool {
	switch service.Visibility {
	case utils.PUBLIC:
		return true
	case utils.PRIVATE:
		if service.Owner == uid {
			return true
		}
	case utils.RESTRICTED:
		if service.Owner == uid || slices.Contains(service.AllowedUsers, uid) {
			return true
		}
	default:
		return false
	}
	return false
}

func isAuthBearer(c *gin.Context) (string, bool) {
	authHeader := c.GetHeader("Authorization")
	splitToken := strings.Split(authHeader, "Bearer ")
	if len(splitToken) == 2 {
		return strings.TrimSpace(splitToken[1]), true
	}
	return "", false
}

func isBasicAuth(c *gin.Context) bool {
	if _, _, ok := c.Request.BasicAuth(); ok {
		return true
	}
	return false
}
