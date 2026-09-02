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
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
)

var errInvalidServiceAuthReturnPath = errors.New("invalid service authentication return path")

// MakeServiceAuthHandler godoc
// @Summary Authenticate service
// @Description Validate access to a specific service using Basic auth or Bearer token (service token or OIDC token), plus service-level permissions. A successful OIDC authentication sets a service-scoped browser cookie. Dashboard form POST requests redirect to the validated exposed-service path.
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
// @Router /system/services/{serviceName}/auth [post]
func MakeServiceAuthHandler(cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodPost {
			returnTo, err := validatedServiceAuthReturnPath(c, c.Param("serviceName"), c.Request.PostFormValue("return_to"), cfg)
			if err != nil {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			auth.SetOIDCServiceAuthCookie(c, cfg)
			c.Redirect(http.StatusSeeOther, returnTo)
			return
		}
		auth.SetOIDCServiceAuthCookie(c, cfg)
		auth.SetForwardOIDCAuthorizationHeader(c)
		c.Status(http.StatusOK)
	}
}

func validatedServiceAuthReturnPath(c *gin.Context, serviceName, rawReturnTo string, cfg *types.Config) (string, error) {
	returnToPath := "/system/services/" + serviceName + "/exposed"
	if cfg.IngressHost != "" && cfg.ExposedServicesUseSubdomainRoute {
		protocol := "http"
		if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") || c.Request.TLS != nil {
			protocol = "https"
		}
		returnToPath = protocol + "://" + serviceName + "." + cfg.IngressHost
	}

	if strings.TrimSpace(rawReturnTo) == "" || strings.TrimSpace(rawReturnTo) == "/" {
		return returnToPath + "/", nil
	}

	returnTo, err := url.Parse(strings.TrimSpace(rawReturnTo))
	if err != nil || returnTo.IsAbs() || returnTo.Host != "" || returnTo.User != nil || !strings.HasPrefix(returnTo.Path, "/") {
		return "", errInvalidServiceAuthReturnPath
	}
	cleanedPath := path.Clean(returnTo.Path)
	if cleanedPath != returnToPath && !strings.HasPrefix(cleanedPath, returnToPath+"/") {
		return "", errInvalidServiceAuthReturnPath
	}
	return returnTo.String(), nil
}
