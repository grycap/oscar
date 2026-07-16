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

// The service-scoped authentication cookie can carry either a service token or
// an OIDC access token. The corresponding middleware determines and validates its type.
func setServiceAuthCookie(c *gin.Context, serviceName, credential string) {
	path := "/system/services/" + serviceName + "/exposed"
	secure := strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") || c.Request.TLS != nil

	http.SetCookie(c.Writer, &http.Cookie{ // #nosec G124
		Name:     getServiceAuthCookieName(serviceName),
		Value:    credential,
		Path:     path,
		MaxAge:   0,
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func serviceAuthCookie(c *gin.Context, serviceName string) string {
	credential, err := c.Cookie(getServiceAuthCookieName(serviceName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(credential)
}

func getServiceAuthCookieName(serviceName string) string {
	return "oscar_service_" + strings.ReplaceAll(serviceName, "-", "_") + "_auth"
}
