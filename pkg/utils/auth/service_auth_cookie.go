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
)

func getServiceAuthCookiePath(serviceName string) string {
	return "/system/services/" + serviceName + "/exposed"
}

// The service-scoped authentication cookie can carry either a service token or
// an OIDC access token. The corresponding middleware validates its contents.
func setServiceAuthCookie(c *gin.Context, serviceName, credential string) {
	secure := strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") || c.Request.TLS != nil

	http.SetCookie(c.Writer, &http.Cookie{ // #nosec G124
		Name:     getServiceAuthCookieName(serviceName),
		Value:    credential,
		Path:     getServiceAuthCookiePath(serviceName),
		MaxAge:   0,
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// The service-scoped authentication cookie can carry either a service token or
// an OIDC access token. The corresponding middleware validates its contents.
// This function sets the cookie with an expired value, so it will be deleted by the browser.
func setExpiredServiceCookie(c *gin.Context, serviceName string) {
	secure := strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") || c.Request.TLS != nil

	http.SetCookie(c.Writer, &http.Cookie{ // #nosec G124
		Name:     getServiceAuthCookieName(serviceName),
		Value:    "",
		Path:     getServiceAuthCookiePath(serviceName),
		MaxAge:   -1,
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func getServiceAuthCookie(c *gin.Context, serviceName string) string {
	credential, err := c.Cookie(getServiceAuthCookieName(serviceName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(credential)
}

func getServiceAuthCookieName(serviceName string) string {
	return "oscar_service_" + strings.ReplaceAll(serviceName, "-", "_") + "_auth"
}
