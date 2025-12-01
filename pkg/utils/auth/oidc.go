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
	"github.com/golang-jwt/jwt/v5"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"golang.org/x/oauth2"
	"k8s.io/client-go/kubernetes"
)

const (
	// EGIGroupsURNPrefix prefix to identify EGI group URI
	EGIGroupsURNPrefix = "urn:mace:egi.eu:group"
	EGIIssuer          = "/realms/egi"
	SecretKeyLength    = 10
)

var oidcLogger = log.New(os.Stdout, "[OIDC-AUTH] ", log.Flags())
var ClusterOidcManagers = make(map[string]*oidcManager)

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
	Subject string
	Groups  []string
	Name    string
}

type KeycloakClaims struct {
	GroupMembership []string `json:"group_membership"`
}

type EGIClaims struct {
	EdupersonEntitlement []string `json:"eduperson_entitlement"`
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
func getOIDCMiddleware(kubeClientset kubernetes.Interface, minIOAdminClient *utils.MinIOAdminClient, cfg *types.Config, oidcConfig *oidc.Config) gin.HandlerFunc {

	for _, iss := range cfg.OIDCValidIssuers {
		issuerManager, err := NewOIDCManager(iss, cfg.OIDCSubject, cfg.OIDCGroups)
		if oidcConfig != nil {
			issuerManager.config = oidcConfig
		}
		if err != nil {
			return func(c *gin.Context) {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
		}

		ClusterOidcManagers[iss] = issuerManager

	}

	mc := NewMultitenancyConfig(kubeClientset, cfg.OIDCSubject)

	return func(c *gin.Context) {
		// Get token from headers
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		rawToken := strings.TrimPrefix(authHeader, "Bearer ")
		iss, err := GetIssuerFromToken(rawToken)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("%v", err))
			return
		}
		oidcManager := ClusterOidcManagers[iss]
		if oidcManager == nil {
			c.String(http.StatusUnauthorized, fmt.Sprintf("'%s' is not listed as an authorized issuer", iss))
			return
		}
		// Check the token
		if !oidcManager.IsAuthorised(rawToken) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		ui, err := oidcManager.GetUserInfo(rawToken)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("%v", err))
			return
		}
		uid := ui.Subject

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
		c.Set("userName", ui.Name)
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

// GetUserInfo obtains UserInfo from the issuer
func (om *oidcManager) GetUserInfo(rawToken string) (*userInfo, error) {
	ot := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: rawToken})

	// Get OIDC UserInfo
	ui, err := om.provider.UserInfo(context.TODO(), ot)
	if err != nil {
		return nil, err
	}

	// Get claims from the provider
	providerAuth := om.provider.Endpoint().AuthURL
	var cerr error
	var groups []string
	if strings.Contains(providerAuth, EGIIssuer) {
		var claims EGIClaims
		cerr = ui.Claims(&claims)
		groups = getGroupsEGI(claims.EdupersonEntitlement)
	} else {
		var claims KeycloakClaims
		cerr = ui.Claims(&claims)
		groups = claims.GroupMembership
	}

	if cerr != nil {
		return nil, cerr
	}

	// Extract name claim in a type-safe way
	name := ""
	var allClaims map[string]interface{}
	if err := ui.Claims(&allClaims); err == nil {
		if n, ok := allClaims["name"].(string); ok {
			name = n
		}
	}

	// Create "userInfo" struct and add the groups
	return &userInfo{
		Subject: ui.Subject,
		Groups:  groups,
		Name:    name,
	}, nil
}

// getGroups transforms "eduperson_entitlement" EGI URNs to a slice of group fields

func getGroupsEGI(urns []string) []string {
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

func GetIssuerFromToken(rawToken string) (string, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(rawToken, jwt.MapClaims{})
	if err != nil {
		return "", err
	}
	claims, _ := token.Claims.(jwt.MapClaims)
	iss, _ := claims.GetIssuer()
	return iss, nil
}

// UserHasVO checks if the user contained on the request token is enrolled on a specific VO
func (om *oidcManager) UserHasVO(ui *userInfo, vo string) bool {
	for _, gr := range ui.Groups {
		if vo == gr {
			return true
		}
	}
	return false
}

func (om *oidcManager) GetUID(rawToken string) (string, error) {
	ui, err := om.GetUserInfo(rawToken)
	if err != nil {
		return "", err
	}
	return ui.Subject, nil
}

// IsAuthorised checks if a token is authorised to access the API
func (om *oidcManager) IsAuthorised(rawToken string) bool {
	// Check if the token is valid
	_, err := om.provider.Verifier(om.config).Verify(context.TODO(), rawToken)
	if err != nil {
		return false
	}

	// Check if token is in cache
	ui, found := om.tokenCache[rawToken]
	if !found {
		// Get userInfo from the issuer
		ui, err = om.GetUserInfo(rawToken)
		if err != nil {
			return false
		}
		// Store userInfo in cache
		om.tokenCache[rawToken] = ui

		// Call clearExpired to delete expired tokens
		om.clearExpired()
	}

	// Groups
	for _, tokenGroup := range ui.Groups {
		for _, authGroup := range om.groups {
			if tokenGroup == authGroup {
				return true
			}
		}
	}

	return false
}
