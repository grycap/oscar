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
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	"k8s.io/client-go/kubernetes"
)

// MakeListHandler godoc
// @Summary List services
// @Description List all created services.
// @Tags services
// @Produce json
// @Param include query string false "Optional expansions (for example: deployment)"
// @Success 200 {array} types.Service
// @Failure 401 {string} string "Unauthorized"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/services [get]
func MakeListHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		includeDeployment := includeQueryContains(c.Query("include"), "deployment")

		services, err := back.ListServices()
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		if len(strings.Split(authHeader, "Bearer")) > 1 {
			uid, err := auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
				return
			}

			isAllowedServiceForUser := false
			allowedServicesForUser := []*types.Service{}
			for _, service := range services {
				switch service.Visibility {
				case utils.PUBLIC:
					isAllowedServiceForUser = true
				case utils.PRIVATE:
					if service.Owner == uid {
						isAllowedServiceForUser = true
					}
				case utils.RESTRICTED:
					if service.Owner == uid || slices.Contains(service.AllowedUsers, uid) {
						isAllowedServiceForUser = true
					}
				}
				// If the service is allowed for the user,
				// set the volume status and deployment summary (if requested),
				// and add it to the list of allowed services for the user.
				if isAllowedServiceForUser {
					isAllowedServiceForUser = false
					setVolumeStatus(back, service)
					if includeDeployment {
						if err := setDeploymentSummary(back, kubeClientset, cfg, service); err != nil {
							c.String(http.StatusInternalServerError, err.Error())
							return
						}
					}
					allowedServicesForUser = append(allowedServicesForUser, service)
				}
			}

			c.JSON(http.StatusOK, allowedServicesForUser)
		} else {
			for _, service := range services {
				setVolumeStatus(back, service)
				if includeDeployment {
					if err := setDeploymentSummary(back, kubeClientset, cfg, service); err != nil {
						c.String(http.StatusInternalServerError, err.Error())
						return
					}
				}
			}
			c.JSON(http.StatusOK, services)
		}

	}
}

func includeQueryContains(raw string, target string) bool {
	for _, value := range strings.Split(raw, ",") {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func setDeploymentSummary(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config, service *types.Service) error {
	runtimeCtx, err := inspectDeploymentRuntime(back, kubeClientset, service, cfg)
	if err != nil {
		return err
	}
	service.Deployment = deploymentSummaryFromStatus(runtimeCtx.status)
	return nil
}
