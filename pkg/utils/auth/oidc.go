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
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

// userInfo custom struct to store essential fields from UserInfo
type userInfo struct {
	subject string
	groups  []string
}

// tokenCache a cache to store userinfo from tokens
var tokenCache = map[string]*userInfo{}

func getOIDCMiddleware(issuer, subject, groups string) gin.HandlerFunc {

	return func(c *gin.Context) {

	}
}

// clearExpired delete expired tokens from the cache
func clearExpired() {
	for rawToken := range tokenCache {
		token := &oauth2.Token{AccessToken: rawToken}
		if !token.Valid() {
			delete(tokenCache, rawToken)
		}
	}
}

// TODO
func getUserInfo(rawToken string) (*userInfo, error) {

}
