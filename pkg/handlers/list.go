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
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
)

// MakeListHandler makes a handler for listing services
func MakeListHandler(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {

		authHeader := c.GetHeader("Authorization")

		services, err := back.ListServices()
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		if len(strings.Split(authHeader, "Bearer")) > 1 {
			uid, err := auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
			}

			var allowedServicesForUser []*types.Service
			for _, service := range services {
				if len(service.AllowedUsers) == 0 {
					allowedServicesForUser = append(allowedServicesForUser, service)
					continue
				}
				for _, id := range service.AllowedUsers {
					if uid == id {
						allowedServicesForUser = append(allowedServicesForUser, service)
						break
					}
				}
			}

			c.JSON(http.StatusOK, allowedServicesForUser)
		} else {
			c.JSON(http.StatusOK, services)
		}

	}
}
