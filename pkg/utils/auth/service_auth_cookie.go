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
)

const GLOBAL_SERVICE_AUTH_COOKIE_NAME = "oscar_service_auth"

func getServiceAuthCookiePath(serviceName string) string {
	return "/system/services/" + serviceName + "/exposed"
}

func getServiceAuthCookieDomain(_ string, cfg *types.Config) string {
	// Wildcards are not valid in Set-Cookie Domain. Use the parent domain.
	return "." + cfg.IngressHost
}

// The service-scoped authentication cookie can carry either a service token or
// an OIDC access token. The corresponding middleware validates its contents.
func setServiceAuthCookie(c *gin.Context, serviceName, credential string, isServiceScopedCookieName, isPathBased bool, cfg *types.Config) {
	secure := strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") || c.Request.TLS != nil
	path := "/"
	name := GLOBAL_SERVICE_AUTH_COOKIE_NAME
	domain := getServiceAuthCookieDomain(serviceName, cfg)

	if isServiceScopedCookieName || isPathBased {
		name = getServiceAuthCookieName(serviceName)
	}
	if isPathBased {
		path = getServiceAuthCookiePath(serviceName)
		domain = ""
	}

	http.SetCookie(c.Writer, &http.Cookie{ // #nosec G124
		Name:     name,
		Value:    credential,
		Path:     path,
		Domain:   domain,
		MaxAge:   0,
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// The service-scoped authentication cookie can carry either a service token or
// an OIDC access token. The corresponding middleware validates its contents.
// This function sets the cookie with an expired value, so it will be deleted by the browser.
func setExpiredServiceCookie(c *gin.Context, serviceName string, serviceScopedCookieName bool) {
	secure := strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") || c.Request.TLS != nil
	name := GLOBAL_SERVICE_AUTH_COOKIE_NAME
	if serviceScopedCookieName {
		name = getServiceAuthCookieName(serviceName)
	}

	http.SetCookie(c.Writer, &http.Cookie{ // #nosec G124
		Name:  name,
		Value: "",
		//Path:     getServiceAuthCookiePath(serviceName, cfg),
		//Domain:   getServiceAuthCookieDomain(serviceName, cfg),
		MaxAge:   -1,
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func getServiceAuthCookie(c *gin.Context, serviceName string, serviceScopedCookieName bool) string {
	cookieName := GLOBAL_SERVICE_AUTH_COOKIE_NAME
	if serviceScopedCookieName {
		cookieName = getServiceAuthCookieName(serviceName)
	}
	credential, err := c.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(credential)
}

func getServiceAuthCookieName(serviceName string) string {
	return "oscar_service_" + strings.ReplaceAll(serviceName, "-", "_") + "_auth"
}
