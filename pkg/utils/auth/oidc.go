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
	"context"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

const EGIGroupsURNPrefix = "urn:mace:egi.eu:group"

type oidcManager struct {
	provider   *oidc.Provider
	subject    string
	groups     []string
	tokenCache map[string]*userInfo
}

// userInfo custom struct to store essential fields from UserInfo
type userInfo struct {
	subject string
	groups  []string
}

func newOIDCManager(issuer string, subject string, groups []string) (*oidcManager, error) {
	provider, err := oidc.NewProvider(context.TODO(), issuer)
	if err != nil {
		return nil, err
	}

	return &oidcManager{
		provider:   provider,
		subject:    subject,
		groups:     groups,
		tokenCache: map[string]*userInfo{},
	}, nil
}

// getIODCMiddleware returns the Gin's handler middleware to validate OIDC-based auth
func getOIDCMiddleware(issuer string, subject string, groups []string) gin.HandlerFunc {
	oidcManager, err := newOIDCManager(issuer, subject, groups)
	if err != nil {
		return func(c *gin.Context) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	}

	return func(c *gin.Context) {
		// Get token from headers
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		rawToken := strings.TrimPrefix(authHeader, "Bearer ")

		// Check the token
		if !oidcManager.isAuthorised(rawToken) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	}
}

// clearExpired delete expired tokens from the cache
func (om *oidcManager) clearExpired() {
	for rawToken := range om.tokenCache {
		token := &oauth2.Token{AccessToken: rawToken}
		if !token.Valid() {
			delete(om.tokenCache, rawToken)
		}
	}
}

// getUserInfo obtains UserInfo from the issuer
func (om *oidcManager) getUserInfo(rawToken string) (*userInfo, error) {
	ot := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: rawToken})

	// Get OIDC UserInfo
	ui, err := om.provider.UserInfo(context.TODO(), ot)
	if err != nil {
		return nil, err
	}

	// Get "eduperson_entitlement" claims
	var claims struct {
		EdupersonEntitlement []string `json:"eduperson_entitlement"`
	}
	ui.Claims(&claims)

	// Create "userInfo" struct and add the groups
	return &userInfo{
		subject: ui.Subject,
		groups:  getGroups(claims.EdupersonEntitlement),
	}, nil
}

// getGroups transforms "eduperson_entitlement" EGI URNs to a slice of group fields
func getGroups(urns []string) []string {
	groups := []string{}

	for _, v := range urns {
		urn := strings.ToLower(strings.TrimSpace(v))
		if strings.HasPrefix(urn, EGIGroupsURNPrefix) {
			urnFields := strings.Split(urn, ":")
			if len(urnFields) >= 5 {
				groups = append(groups, urnFields[4])
			}
		}
	}

	return groups
}

// isAuthorised checks if a token is authorised to access the API
func (om *oidcManager) isAuthorised(rawToken string) bool {
	// Check if the token is valid
	token := &oauth2.Token{AccessToken: rawToken}
	if !token.Valid() {
		return false
	}

	// Check if token is in cache
	var err error
	ui, found := om.tokenCache[rawToken]
	if !found {
		// Get userInfo from the issuer
		ui, err = om.getUserInfo(rawToken)
		if err != nil {
			return false
		}

		// Store userInfo in cache
		om.tokenCache[rawToken] = ui

		// Call clearExpired to delete expired tokens
		om.clearExpired()
	}

	// Check if is authorised
	// Same subject
	if ui.subject == om.subject {
		return true
	}

	// Groups
	for _, tokenGroup := range ui.groups {
		for _, authGroup := range om.groups {
			if tokenGroup == authGroup {
				return true
			}
		}
	}

	return false
}
