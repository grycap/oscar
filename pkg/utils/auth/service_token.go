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

		// Check if reqToken is the service token
		if token, ok := isAuthBearer(c); ok && len(token) == tokenLength {
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
				if token == serviceIter.Token {
					c.Set(isServiceTokenKey, true)
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
