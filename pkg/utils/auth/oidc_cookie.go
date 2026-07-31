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
	"github.com/grycap/oscar/v4/pkg/types"
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
func GetOIDCServiceAuthCookieMiddleware(cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}
		// If the request already has a Bearer token or Basic auth, we don't override it.
		if _, isBearerAuth := isAuthBearer(c); isBearerAuth || isBasicAuth(c) {
			c.Next()
			return
		}
		serviceScopedCookieName := cfg.IngressHost == "" || !cfg.ExposedServicesUseSubdomainRoute
		token := getServiceAuthCookie(c, c.Param("serviceName"), serviceScopedCookieName)
		if looksLikeJWT(token) {
			// Promote the cookie's JWT to the Authorization header for the rest of the middleware chain.
			c.Request.Header.Set("Authorization", "Bearer "+token)
		}

		// Set the cookie as expired and if validation succeeds,
		// the OIDC middleware will set it again with a renewed expiration time.
		// This prevents users from being blocked when the cookie is used after the OIDC token has expired.
		setExpiredServiceCookie(c, c.Param("serviceName"), serviceScopedCookieName)

		c.Next()
	}
}

// SetForwardOIDCAuthorizationHeader promotes the OIDC token from the request's
// Authorization header to the response's Authorization header. This is used for
// service-to-service calls where the caller needs to forward the OIDC token.
func SetForwardOIDCAuthorizationHeader(c *gin.Context) {
	token, ok := isAuthBearer(c)
	if !ok || !looksLikeJWT(token) {
		return
	}
	c.Writer.Header().Set("Authorization", "Bearer "+token)
}

// SetOIDCServiceAuthCookie stores an OIDC Bearer token after authentication and
// service authorization have succeeded.
func SetOIDCServiceAuthCookie(c *gin.Context, cfg *types.Config) {
	token, ok := isAuthBearer(c)
	if !ok || !looksLikeJWT(token) {
		return
	}
	isPathBased := cfg.IngressHost == "" || !cfg.ExposedServicesUseSubdomainRoute
	setServiceAuthCookie(c, c.Param("serviceName"), token, isPathBased, isPathBased, cfg)
}

// looksLikeJWT only classifies the credential. The regular OIDC middleware is
// responsible for cryptographic and claims validation.
func looksLikeJWT(token string) bool {
	parts := strings.Split(strings.TrimSpace(token), ".")
	return len(parts) == 3 && parts[0] != "" && parts[1] != "" && parts[2] != ""
}
