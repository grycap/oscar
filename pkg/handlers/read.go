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
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v2/pkg/types"
	"github.com/grycap/oscar/v2/pkg/utils/auth"
	"k8s.io/apimachinery/pkg/api/errors"
)

// MakeReadHandler makes a handler for reading a service
func MakeReadHandler(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {

		authHeader := c.GetHeader("Authorization")

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
		if len(strings.Split(authHeader, "Bearer")) > 1 {
			uid, err := auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
			}

			var isAllowed bool
			for _, id := range service.AllowedUsers {
				if uid == id {
					isAllowed = true
					break
				}
			}

			if !isAllowed {
				c.String(http.StatusForbidden, "User %s doesn't have permision to get this service", uid)
				return
			}
		}

		c.JSON(http.StatusOK, service)
	}
}
