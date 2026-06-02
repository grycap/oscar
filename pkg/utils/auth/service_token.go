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
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/types"
	"k8s.io/apimachinery/pkg/api/errors"
)

const tokenLength = 64
const isServiceTokenKey = "isServiceToken"

// GetServiceTokenMiddleware returns a gin middleware that checks if the request is authenticated with a service token
// APPLY ONLY before auth.GetAuthMiddleware, since it relies on the fact that if a service token is provided, the user authentication will not be performed
func GetServiceTokenMiddleware(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isBasicAuth(c) {
			c.Next()
			return
		}

		if tokens := getServiceTokenCandidates(c); len(tokens) > 0 {
			serviceList, err := back.ListServicesByName(c.Param("serviceName"))
			if err != nil {
				// Check if error is caused because the service is not found
				if errors.IsNotFound(err) || errors.IsGone(err) {
					c.AbortWithStatus(http.StatusNotFound)
				} else {
					c.AbortWithStatus(http.StatusInternalServerError)
				}
				return
			}

			// only one service should be returned
			// the restriction for unique service names is enforced in the service creation and update handlers
			service := serviceList[0]
			for _, token := range tokens {
				if token == service.Token {
					c.Set(isServiceTokenKey, true)
					setServiceTokenCookie(c, service.Name, token)
					c.Next()
					return
				}
			}

			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
		return
	}
}

func getServiceTokenCandidates(c *gin.Context) []string {
	tokens := []string{}

	// Prioritise the token in the authorization header over other sources of service tokens
	if token, ok := isAuthBearer(c); ok {
		if len(strings.TrimSpace(token)) == tokenLength {
			tokens = append(tokens, token)
		}
		return tokens
	}

	if token := strings.TrimSpace(c.Query("token")); len(token) == tokenLength {
		tokens = append(tokens, token)
	}

	if token := serviceTokenFromForwardedURI(c.GetHeader("X-Forwarded-Uri")); len(token) == tokenLength {
		tokens = append(tokens, token)
	}

	if token, err := c.Cookie(getServiceTokenCookieName(c.Param("serviceName"))); err == nil && len(strings.TrimSpace(token)) == tokenLength {
		tokens = append(tokens, strings.TrimSpace(token))
	}

	return tokens
}

func serviceTokenFromForwardedURI(rawURI string) string {
	if strings.TrimSpace(rawURI) == "" {
		return ""
	}

	uri, err := url.Parse(rawURI)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(uri.Query().Get("token"))
}

func setServiceTokenCookie(c *gin.Context, serviceName string, token string) {
	path := "/system/services/" + serviceName + "/exposed"
	secure := strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") || c.Request.TLS != nil

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     getServiceTokenCookieName(serviceName),
		Value:    token,
		Path:     path,
		MaxAge:   0,
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func getServiceTokenCookieName(serviceName string) string {
	return "oscar_service_" + strings.ReplaceAll(serviceName, "-", "_") + "_auth"
}
