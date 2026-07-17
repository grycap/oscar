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
	"errors"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const serviceAuthFormMaxBytes = 16 * 1024

// GetOIDCServiceAuthFormMiddleware promotes the OIDC token submitted by a
// Dashboard top-level form into the regular Bearer authentication chain. A
// form navigation avoids cross-origin cookie bootstrap and CORS requirements.
func GetOIDCServiceAuthFormMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || mediaType != "application/x-www-form-urlencoded" {
			c.AbortWithStatus(http.StatusUnsupportedMediaType)
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, serviceAuthFormMaxBytes)
		if err := c.Request.ParseForm(); err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			} else {
				c.AbortWithStatus(http.StatusBadRequest)
			}
			return
		}

		token := strings.TrimSpace(c.Request.PostForm.Get("token"))
		if !looksLikeJWT(token) {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		c.Request.Header.Set("Authorization", "Bearer "+token)
		c.Next()
	}
}

// GetOIDCServiceAuthCookieMiddleware restores a browser's service-scoped OIDC
// credential as a Bearer header so the regular OIDC middleware validates it.
func GetOIDCServiceAuthCookieMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := isAuthBearer(c); !ok {
			token := serviceAuthCookie(c, c.Param("serviceName"))
			if looksLikeJWT(token) {
				c.Request.Header.Set("Authorization", "Bearer "+token)
			}
		}
		c.Next()
	}
}

// SetOIDCServiceAuthCookie stores an OIDC Bearer token after authentication and
// service authorization have succeeded.
func SetOIDCServiceAuthCookie(c *gin.Context) {
	token, ok := isAuthBearer(c)
	if !ok || !looksLikeJWT(token) {
		return
	}
	setServiceAuthCookie(c, c.Param("serviceName"), token)
}

// looksLikeJWT only classifies the credential. The regular OIDC middleware is
// responsible for cryptographic and claims validation.
func looksLikeJWT(token string) bool {
	parts := strings.Split(strings.TrimSpace(token), ".")
	return len(parts) == 3 && parts[0] != "" && parts[1] != "" && parts[2] != ""
}
