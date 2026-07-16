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
	"net/url"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
)

// MakeServiceAuthHandler godoc
// @Summary Authorize access to an exposed service
// @Description Validate a service token or OIDC bearer credential, apply service-level permissions, and return ForwardAuth identity headers.
// @Tags services
// @Param serviceName path string true "Service name"
// @Success 200 "Authorized"
// @Failure 401 "Unauthorized"
// @Failure 403 "Forbidden"
// @Failure 404 "Not Found"
// @Failure 500 "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/services/{serviceName}/auth [get]
func MakeServiceAuthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		setForwardAuthResponse(c)
		c.Status(http.StatusOK)
	}
}

// MakeServiceAuthBootstrapHandler godoc
// @Summary Start an exposed-service browser session
// @Description Validate and authorize an OIDC access token submitted by a browser, set a service-scoped authentication cookie, and redirect to the exposed service.
// @Tags services
// @Accept application/x-www-form-urlencoded
// @Param serviceName path string true "Service name"
// @Param token formData string true "OIDC access token"
// @Param redirect formData string false "Path below the selected service's exposed route"
// @Success 303 "Authenticated; redirecting to the exposed service"
// @Failure 400 "Invalid redirect path"
// @Failure 401 "Unauthorized"
// @Failure 403 "Forbidden"
// @Failure 404 "Not Found"
// @Failure 500 "Internal Server Error"
// @Security BearerAuth
// @Router /system/services/{serviceName}/auth [post]
func MakeServiceAuthBootstrapHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		setForwardAuthResponse(c)
		target, ok := serviceAuthRedirect(c.Param("serviceName"), c.PostForm("redirect"))
		if !ok {
			c.Status(http.StatusBadRequest)
			return
		}
		c.Redirect(http.StatusSeeOther, target)
	}
}

func setForwardAuthResponse(c *gin.Context) {
	auth.SetForwardAuthCookie(c)
	// Always return these headers so Traefik replaces any client-supplied
	// values, including for service-token authentication where no end-user
	// identity is available.
	c.Header(auth.OscarUserSubHeader, safeForwardAuthHeader(c.GetString("uidOrigin")))
	c.Header(auth.OscarUserNameHeader, safeForwardAuthHeader(c.GetString("userName")))
	groups, _ := c.Get(auth.UserGroupsContextKey)
	if values, ok := groups.([]string); ok {
		c.Header(auth.OscarUserGroupsHeader, safeForwardAuthHeader(strings.Join(values, ",")))
	} else {
		c.Header(auth.OscarUserGroupsHeader, "")
	}
}

func serviceAuthRedirect(serviceName, requested string) (string, bool) {
	basePath := "/system/services/" + serviceName + "/exposed"
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return basePath + "/", true
	}

	target, err := url.ParseRequestURI(requested)
	if err != nil || target.IsAbs() || target.Host != "" || target.Path == "" || strings.Contains(target.Path, "\\") {
		return "", false
	}

	hadTrailingSlash := strings.HasSuffix(target.Path, "/")
	cleanPath := path.Clean(target.Path)
	if cleanPath != basePath && !strings.HasPrefix(cleanPath, basePath+"/") {
		return "", false
	}
	// Reject ambiguous traversal instead of silently redirecting to a normalized target.
	if cleanPath != strings.TrimSuffix(target.Path, "/") {
		return "", false
	}

	target.Path = cleanPath
	target.RawPath = ""
	if hadTrailingSlash {
		target.Path += "/"
	}
	return target.String(), true
}

func safeForwardAuthHeader(value string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(value)
}
