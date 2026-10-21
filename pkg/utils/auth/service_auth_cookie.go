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

// serviceAuthCookieScope holds the attributes that give a service auth cookie
// its browser identity: its name, Path and Domain. A Set-Cookie header only
// replaces or deletes the cookie that was issued with the exact same scope.
type serviceAuthCookieScope struct {
	name   string
	path   string
	domain string
}

// getServiceAuthCookieScope computes the cookie scope shared by the issuing
// and the expiring code paths, so an expiring Set-Cookie always targets the
// cookie that was originally issued. In path-based routing the cookie is
// scoped to the service's exposed path with a host-only domain; in subdomain
// routing it is scoped to the parent ingress domain with the root path.
func getServiceAuthCookieScope(serviceName string, isServiceScopedCookieName, isPathBased bool, cfg *types.Config) serviceAuthCookieScope {
	scope := serviceAuthCookieScope{
		name:   GLOBAL_SERVICE_AUTH_COOKIE_NAME,
		path:   "/",
		domain: getServiceAuthCookieDomain(serviceName, cfg),
	}

	if isServiceScopedCookieName || isPathBased {
		scope.name = getServiceAuthCookieName(serviceName)
	}
	if isPathBased {
		scope.path = getServiceAuthCookiePath(serviceName)
		scope.domain = ""
	}

	return scope
}

// The service-scoped authentication cookie can carry either a service token or
// an OIDC access token. The corresponding middleware validates its contents.
func setServiceAuthCookie(c *gin.Context, serviceName, credential string, isServiceScopedCookieName, isPathBased bool, cfg *types.Config) {
	secure := strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") || c.Request.TLS != nil
	scope := getServiceAuthCookieScope(serviceName, isServiceScopedCookieName, isPathBased, cfg)

	http.SetCookie(c.Writer, &http.Cookie{ // #nosec G124
		Name:     scope.name,
		Value:    credential,
		Path:     scope.path,
		Domain:   scope.domain,
		MaxAge:   0,
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// The service-scoped authentication cookie can carry either a service token or
// an OIDC access token. The corresponding middleware validates its contents.
// This function sets the cookie with an expired value, so it will be deleted by the browser.
func setExpiredServiceCookie(c *gin.Context, serviceName string, isServiceScopedCookieName, isPathBased bool, cfg *types.Config) {
	secure := strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") || c.Request.TLS != nil
	scope := getServiceAuthCookieScope(serviceName, isServiceScopedCookieName, isPathBased, cfg)

	http.SetCookie(c.Writer, &http.Cookie{ // #nosec G124
		Name:     scope.name,
		Value:    "",
		Path:     scope.path,
		Domain:   scope.domain,
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
