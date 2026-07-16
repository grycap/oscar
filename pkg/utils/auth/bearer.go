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
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// isAuthBearer extracts a credential transported with the HTTP Bearer scheme.
// It does not determine whether the credential is a service token or an OIDC token.
func isAuthBearer(c *gin.Context) (string, bool) {
	authHeader := c.GetHeader("Authorization")
	splitToken := strings.Split(authHeader, "Bearer ")
	if len(splitToken) == 2 {
		return strings.TrimSpace(splitToken[1]), true
	}
	return "", false
}

func tokenFromForwardedURI(rawURI string) string {
	if strings.TrimSpace(rawURI) == "" {
		return ""
	}

	uri, err := url.Parse(rawURI)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(uri.Query().Get("token"))
}
