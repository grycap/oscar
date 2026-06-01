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

		// Check if any request token is the service token. Exposed services may use
		// their own "token" query parameter, so keep checking the auth cookie too.
		tokens := getServiceTokenCandidates(c)
		hasServiceTokenCandidate := false
		for _, token := range tokens {
			if len(token) == tokenLength {
				hasServiceTokenCandidate = true
				break
			}
		}
		if hasServiceTokenCandidate {
			serviceList, err := back.ListServicesByName(c.Param("serviceName"), "")
			if err != nil {
				// Check if error is caused because the service is not found
				if errors.IsNotFound(err) || errors.IsGone(err) {
					c.AbortWithStatus(http.StatusNotFound)
				} else {
					c.AbortWithStatus(http.StatusInternalServerError)
				}
				return
			}

			for _, serviceIter := range serviceList {
				for _, token := range tokens {
					if len(token) != tokenLength {
						continue
					}
					if token == serviceIter.Token {
						c.Set(isServiceTokenKey, true)
						setServiceTokenCookie(c, serviceIter.Name, token)
						c.Next()
						return
					}
				}
			}
			if hasServiceTokenCandidate {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
		}

		c.Next()
		return
	}
}

func getServiceTokenCandidates(c *gin.Context) []string {
	tokens := []string{}

	if token, ok := isAuthBearer(c); ok {
		tokens = append(tokens, token)
	}

	if token := strings.TrimSpace(c.Query("token")); token != "" {
		tokens = append(tokens, token)
	}

	if token := serviceTokenFromForwardedURI(c.GetHeader("X-Forwarded-Uri")); token != "" {
		tokens = append(tokens, token)
	}

	if token, err := c.Cookie(getServiceTokenCookieName(c.Param("serviceName"))); err == nil && strings.TrimSpace(token) != "" {
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
