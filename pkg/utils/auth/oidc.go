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
	"fmt"
	"log"
	"os"

	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/utils"
	"golang.org/x/oauth2"
	"k8s.io/client-go/kubernetes"
)

const (
	// EGIGroupsURNPrefix prefix to identify EGI group URNs
	EGIGroupsURNPrefix = "urn:mace:egi.eu:group"
	SecretKeyLength    = 10
)

var oidcLogger = log.New(os.Stdout, "[OIDC-AUTH] ", log.Flags())

// oidcManager struct to represent a OIDC manager, including a cache of tokens
type oidcManager struct {
	provider   *oidc.Provider
	config     *oidc.Config
	subject    string
	groups     []string
	tokenCache map[string]*userInfo
}

// userInfo custom struct to store essential fields from UserInfo
type userInfo struct {
	subject string
	groups  []string
}

// newOIDCManager returns a new oidcManager or error if the oidc.Provider can't be created
func NewOIDCManager(issuer string, subject string, groups []string) (*oidcManager, error) {
	provider, err := oidc.NewProvider(context.TODO(), issuer)
	if err != nil {
		return nil, err
	}

	config := &oidc.Config{
		SkipClientIDCheck: true,
	}

	return &oidcManager{
		provider:   provider,
		config:     config,
		subject:    subject,
		groups:     groups,
		tokenCache: map[string]*userInfo{},
	}, nil
}

// getIODCMiddleware returns the Gin's handler middleware to validate OIDC-based auth
func getOIDCMiddleware(kubeClientset *kubernetes.Clientset, minIOAdminClient *utils.MinIOAdminClient, issuer string, subject string, groups []string) gin.HandlerFunc {
	oidcManager, err := NewOIDCManager(issuer, subject, groups)
	if err != nil {
		return func(c *gin.Context) {
			c.AbortWithStatus(http.StatusUnauthorized)
		}
	}

	mc := NewMultitenancyConfig(kubeClientset, subject)

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

		ui, err := oidcManager.getUserInfo(rawToken)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("%v", err))
			return
		}
		uid := ui.subject

		// Check if exist MinIO user in cached users list
		minioUserExists := mc.UserExists(uid)

		if !minioUserExists {
			sk, err := GenerateRandomKey(SecretKeyLength)
			if err != nil {
				oidcLogger.Println("Error generating random key for MinIO user")
			}
			// Create MinIO user and k8s secret with credentials
			err = mc.CreateSecretForOIDC(uid, sk)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating secret for user %s: %v", uid, err))
			}
			err = minIOAdminClient.CreateMinIOUser(uid, sk)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating MinIO user for uid %s: %v", uid, err))
			}
		}
		c.Set("uidOrigin", uid)
		c.Set("multitenancyConfig", mc)
		c.Next()
	}
}

// clearExpired delete expired tokens from the cache
func (om *oidcManager) clearExpired() {
	for rawToken := range om.tokenCache {
		_, err := om.provider.Verifier(om.config).Verify(context.TODO(), rawToken)
		if err != nil {
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

// UserHasVO checks if the user contained on the request token is enrolled on a specific VO
func (om *oidcManager) UserHasVO(rawToken string, vo string) (bool, error) {
	ui, err := om.getUserInfo(rawToken)
	if err != nil {
		return false, err
	}
	for _, gr := range ui.groups {
		if vo == gr {
			return true, nil
		}
	}
	return false, nil
}

func (om *oidcManager) GetUID(rawToken string) (string, error) {
	ui, err := om.getUserInfo(rawToken)
	oidcLogger.Println("received uid: ", ui.subject)
	if err != nil {
		return ui.subject, nil
	}
	return "", err
}

// isAuthorised checks if a token is authorised to access the API
func (om *oidcManager) isAuthorised(rawToken string) bool {
	// Check if the token is valid
	_, err := om.provider.Verifier(om.config).Verify(context.TODO(), rawToken)
	if err != nil {
		return false
	}

	// Check if token is in cache
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
