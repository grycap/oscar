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
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/types"
	"k8s.io/apimachinery/pkg/api/errors"
)

const serviceTokenLength = 64
const isServiceTokenKey = "isServiceToken"

// GetServiceTokenMiddleware checks credentials issued specifically for an OSCAR service.
// Apply it only before GetAuthMiddleware, because a valid service token bypasses user authentication.
func GetServiceTokenMiddleware(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isBasicAuth(c) {
			c.Next()
			return
		}

		if tokens := getServiceTokenCandidates(c); len(tokens) > 0 {
			serviceList, err := back.ListServicesByName(c.Param("serviceName"))
			if err != nil {
				if errors.IsNotFound(err) || errors.IsGone(err) {
					c.AbortWithStatus(http.StatusNotFound)
				} else {
					c.AbortWithStatus(http.StatusInternalServerError)
				}
				return
			}

			// Service names are unique, so the lookup returns the single target service.
			service := serviceList[0]
			for _, token := range tokens {
				if token == service.Token {
					c.Set(isServiceTokenKey, true)
					setServiceAuthCookie(c, service.Name, token)
					c.Next()
					return
				}
			}

			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}

func getServiceTokenCandidates(c *gin.Context) []string {
	tokens := []string{}

	// An Authorization credential takes precedence over tokens from other sources.
	if token, ok := isAuthBearer(c); ok {
		if len(strings.TrimSpace(token)) == serviceTokenLength {
			tokens = append(tokens, token)
		}
		return tokens
	}

	if token := strings.TrimSpace(c.Query("token")); len(token) == serviceTokenLength {
		tokens = append(tokens, token)
	}

	if token := tokenFromForwardedURI(c.GetHeader("X-Forwarded-Uri")); len(token) == serviceTokenLength {
		tokens = append(tokens, token)
	}

	if token := serviceAuthCookie(c, c.Param("serviceName")); len(token) == serviceTokenLength {
		tokens = append(tokens, token)
	}

	return tokens
}
