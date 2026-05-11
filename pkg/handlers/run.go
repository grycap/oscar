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
	"net/http/httputil"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	tokenLength            = 64
	errServiceNotFound     = "Service Not Found"
	errMultipleServiceAuth = "More than one service authorize, use the owner query to select the service"
)

// MakeRunHandler godoc
// @Summary Invoke service synchronously
// @Description Invoke a service synchronously using the configured Serverless backend.
// @Tags sync
// @Accept json
// @Accept octet-stream
// @Param serviceName path string true "Service name"
// @Param payload body string false "Event payload"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BearerAuth
// @Router /run/{serviceName} [post]
func MakeRunHandler(cfg *types.Config, back types.SyncBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		var service *types.Service
		serviceList, err := back.ListServicesByName(c.Param("serviceName"), "")
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

		// Check if reqToken is the service token
		rawToken := strings.TrimSpace(splitToken[1])
		if len(rawToken) == tokenLength {
			for _, serviceIter := range serviceList {
				if rawToken == serviceIter.Token {
					service = serviceIter
				}
			}
			if service == nil {
				c.Status(http.StatusUnauthorized)
				return
			}
		} else {
			issuer, err := auth.GetIssuerFromToken(rawToken)
			if err != nil {
				c.String(http.StatusBadGateway, err.Error())
			}
			oidcManager := auth.ClusterOidcManagers[issuer]
			if oidcManager == nil {
				c.String(http.StatusBadRequest, fmt.Sprintf("Error getting oidc manager for issuer '%s'", issuer))
				return
			}

			ui, err := oidcManager.GetUserInfo(rawToken)

			if !oidcManager.IsAuthorised(rawToken) {
				c.Status(http.StatusNotFound)
				return
			}

			uid := ui.Subject
			c.Set("uidOrigin", uid)
			c.Next()

			service, err = selectService(c, serviceList)
			if err != nil {
				if err.Error() == errServiceNotFound {
					c.Status(http.StatusNotFound)
				} else {
					c.String(http.StatusBadRequest, err.Error())
				}
				return
			}

			hasVO := oidcManager.UserInOneGroup(ui, cfg)

			if !hasVO {
				c.String(http.StatusUnauthorized, "this user isn't enrrolled on the vo: %v", service.VO)
				return
			}

		}

		if service.Owner != types.DefaultOwner && cfg.KueueEnable && !utils.VerifyWorkload(*service, service.Namespace, cfg) {
			c.String(http.StatusBadRequest, "invalid workload: try to reduce the service resource (cpu, memory, etc.)")
			return
		}

		proxy := &httputil.ReverseProxy{
			Director: back.GetProxyDirector(service.Name, service.Namespace),
		}
		proxy.ServeHTTP(c.Writer, c.Request) // #nosec
	}
}

func selectService(c *gin.Context, serviceList []*types.Service) (*types.Service, error) {
	// If no services found, return not found
	if len(serviceList) == 0 {
		return nil, fmt.Errorf(errServiceNotFound)
	} else if len(serviceList) == 1 { // Found 1 service
		if authorizeRequest(c, serviceList[0]) { // Found 1 service and is authorize
			return serviceList[0], nil
		} else {
			return nil, fmt.Errorf(errServiceNotFound)
		}
	} else { // Found more than one service with same name
		authTime := 0
		var service *types.Service
		for _, serviceIter := range serviceList {
			if authorizeRequest(c, serviceIter) { // Get the services authorized
				service = serviceIter
				authTime++
			}
		}
		if authTime == 1 { // More than 1 service found, but 1 service authorize
			return service, nil
		} else if authTime == 0 { // More than 1 service found, but no service authorize
			return nil, fmt.Errorf(errServiceNotFound)
		} else if authTime > 1 { // More than 1 service found, and more than one service authorize -> user query owner
			owner := strings.TrimSpace(c.Query("owner"))
			if owner == "" {
				return nil, fmt.Errorf(errMultipleServiceAuth)
			}
			for _, serviceIter := range serviceList {
				if authorizeRequest(c, serviceIter) && serviceIter.Owner == owner {
					service = serviceIter
					return service, nil
				}
			}
			return nil, fmt.Errorf(errServiceNotFound)
		}
		return nil, fmt.Errorf(errServiceNotFound)
	}
}
