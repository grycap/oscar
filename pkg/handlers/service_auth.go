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

	"github.com/gin-gonic/gin"
)

// MakeServiceAuthHandler godoc
// @Summary Authenticate service
// @Description Validate access to a specific service using Basic auth or Bearer token (service token or OIDC token), plus service-level permissions.
// @Tags services
// @Param serviceName path string true "Service name"
// @Success 200 "OK"
// @Failure 401 "Unauthorized"
// @Failure 403 "Forbidden"
// @Failure 404 "Not Found"
// @Failure 500 "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/services/{serviceName}/auth [get]
func MakeServiceAuthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Status(http.StatusOK)
		return
	}
}
