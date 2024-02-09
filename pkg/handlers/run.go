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
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"k8s.io/apimachinery/pkg/api/errors"
)

// MakeRunHandler makes a handler to manage sync invocations sending them to the gateway of the ServerlessBackend
func MakeRunHandler(cfg *types.Config, back types.SyncBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		service, err := back.ReadService(c.Param("serviceName"))
		if err != nil {
			// Check if error is caused because the service is not found
			if errors.IsNotFound(err) || errors.IsGone(err) {
				c.Status(http.StatusNotFound)
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			return
		}

		// Check auth token
		authHeader := c.GetHeader("Authorization")
		splitToken := strings.Split(authHeader, "Bearer ")
		if len(splitToken) != 2 {
			c.Status(http.StatusUnauthorized)
			return
		}
		reqToken := strings.TrimSpace(splitToken[1])
		if reqToken != service.Token {
			c.Status(http.StatusUnauthorized)
			return
		}

		proxy := &httputil.ReverseProxy{
			Director: back.GetProxyDirector(service.Name),
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
