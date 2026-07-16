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
)

const forwardAuthTokenContextKey = "forwardAuthOIDCToken"
const minOIDCAccessTokenLength = 65
const maxOIDCAccessTokenLength = 16 * 1024

const (
	OscarUserSubHeader    = "X-OSCAR-User-Sub"
	OscarUserNameHeader   = "X-OSCAR-User-Name"
	OscarUserGroupsHeader = "X-OSCAR-User-Groups"
)

// GetForwardAuthBootstrapMiddleware obtains an OIDC access token from a browser
// bootstrap request or service-scoped cookie. The regular OIDC and service
// permission middleware still validate and authorize it on every request.
func GetForwardAuthBootstrapMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := forwardAuthOIDCToken(c)
		if token == "" {
			c.Next()
			return
		}

		if _, ok := isAuthBearer(c); !ok {
			c.Request.Header.Set("Authorization", "Bearer "+token)
		}
		c.Set(forwardAuthTokenContextKey, token)
		c.Next()
	}
}

// SetForwardAuthCookie persists a detected OIDC access token. Call it only
// after OIDC authentication and service permissions have succeeded.
func SetForwardAuthCookie(c *gin.Context) {
	value, exists := c.Get(forwardAuthTokenContextKey)
	if !exists {
		return
	}
	token, ok := value.(string)
	if !ok || !looksLikeJWT(token) {
		return
	}
	setServiceAuthCookie(c, c.Param("serviceName"), token)
}

func forwardAuthOIDCToken(c *gin.Context) string {
	if token, ok := isAuthBearer(c); ok && looksLikeJWT(token) {
		return token
	}

	if token := strings.TrimSpace(c.PostForm("token")); looksLikeJWT(token) {
		return token
	}

	if token := strings.TrimSpace(c.Query("token")); looksLikeJWT(token) {
		return token
	}

	if token := tokenFromForwardedURI(c.GetHeader("X-Forwarded-Uri")); looksLikeJWT(token) {
		return token
	}

	if token := serviceAuthCookie(c, c.Param("serviceName")); looksLikeJWT(token) {
		return token
	}

	return ""
}

// looksLikeJWT performs structural classification only. Cryptographic and
// claim validation is deliberately left to the OIDC middleware.
func looksLikeJWT(token string) bool {
	token = strings.TrimSpace(token)
	return len(token) >= minOIDCAccessTokenLength && len(token) <= maxOIDCAccessTokenLength && strings.Count(token, ".") == 2
}
