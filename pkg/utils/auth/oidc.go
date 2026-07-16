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
	"sync"

	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
	"golang.org/x/oauth2"
	"k8s.io/client-go/kubernetes"
)

const (
	// EGIGroupsURNPrefix prefix to identify EGI group URI
	EGIGroupsURNPrefix = "urn:mace:egi.eu:group"
	EGIIssuer          = "/realms/egi"
	AI4EOSCIssuer      = "/realms/ai4eosc"

	SecretKeyLength      = 10
	UserGroupsContextKey = "userGroups"
)

var oidcLogger = log.New(os.Stdout, "[OIDC-AUTH] ", log.Flags())
var ClusterOidcManagers = make(map[string]*oidcManager)

// oidcManager struct to represent a OIDC manager, including a cache of tokens
type oidcManager struct {
	provider        *oidc.Provider
	config          *oidc.Config
	subject         string
	groups          []string
	tokenCache      map[string]*userInfo
	tokenCacheMutex sync.RWMutex
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
	Entitlements []string `json:"entitlements"`
}

type GroupAI4EOSC struct {
	Sub            string `json:"sub"`
	ResourceAccess struct {
		Account struct {
			Roles []string `json:"roles"`
		} `json:"account"`
	} `json:"resource_access"`
	EmailVerified bool `json:"email_verified"`
	RealmAccess   struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	Email             string `json:"email"`
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

type oidcIdentityHandler func(*gin.Context) (*userInfo, bool)

func newOIDCIdentityHandler(cfg *types.Config, oidcConfig *oidc.Config) oidcIdentityHandler {
	managers := make(map[string]*oidcManager, len(cfg.OIDCValidIssuers))
	for _, iss := range cfg.OIDCValidIssuers {
		manager, err := NewOIDCManager(iss, cfg.OIDCSubject, cfg.OIDCGroups)
		if err != nil {
			return func(c *gin.Context) (*userInfo, bool) {
				c.AbortWithStatus(http.StatusUnauthorized)
				return nil, false
			}
		}
		if oidcConfig != nil {
			manager.config = oidcConfig
		}

		managers[iss] = manager
		ClusterOidcManagers[iss] = manager
	}

	return func(c *gin.Context) (*userInfo, bool) {
		rawToken, ok := isAuthBearer(c)
		if !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
			return nil, false
		}
		iss, err := GetIssuerFromToken(rawToken)
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("%v", err))
			return nil, false
		}
		manager := managers[iss]
		if manager == nil {
			c.String(http.StatusUnauthorized, fmt.Sprintf("'%s' is not listed as an authorized issuer", iss))
			return nil, false
		}

		ui, ok := manager.authorizedUserInfo(rawToken)
		if !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
			return nil, false
		}
		return ui, true
	}
}

// getOIDCIdentityMiddleware validates OIDC identity and groups without
// provisioning any per-user infrastructure. It is used by ForwardAuth.
func getOIDCIdentityMiddleware(cfg *types.Config, oidcConfig *oidc.Config) gin.HandlerFunc {
	authenticate := newOIDCIdentityHandler(cfg, oidcConfig)
	return func(c *gin.Context) {
		ui, ok := authenticate(c)
		if !ok {
			return
		}
		setOIDCIdentityContext(c, ui)
		c.Next()
	}
}

// getOIDCMiddleware validates OIDC identity and provisions the infrastructure
// needed by regular OSCAR API operations.
func getOIDCMiddleware(kubeClientset kubernetes.Interface, minIOAdminClient *utils.MinIOAdminClient, cfg *types.Config, oidcConfig *oidc.Config) gin.HandlerFunc {
	authenticate := newOIDCIdentityHandler(cfg, oidcConfig)
	mc := NewMultitenancyConfig(kubeClientset, cfg.OIDCSubject)

	return func(c *gin.Context) {
		ui, ok := authenticate(c)
		if !ok {
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
				return
			}
			err = minIOAdminClient.CreateMinIOUser(uid, sk)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating MinIO user for uid %s: %v", uid, err))
				return
			}
		}

		// Create Kueue ClusterQueue and LocalQueue for the user if they don't exist
		if err := utils.CreateKueueUserQueuesIfDontExist(cfg, uid); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating Kueue ClusterQueue for user %s: %v", uid, err))
			return
		}
		namespace, err := utils.EnsureUserNamespace(c.Request.Context(), kubeClientset, cfg, uid)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("error ensuring namespace for user %s: %v", uid, err))
			return
		}

		// Ensure Volume Quotas for the user
		if _, err := utils.CreateMinIOQuotaConfigMapIfDontExist(c.Request.Context(), cfg, kubeClientset, namespace); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating Kueue ClusterQueue for user %s: %v", uid, err))
			return
		}

		utils.EnsureVolumeLimits(FormatUID(uid), namespace, kubeClientset, cfg)

		setOIDCIdentityContext(c, ui)
		c.Set("multitenancyConfig", mc)
		c.Next()
	}
}

func setOIDCIdentityContext(c *gin.Context, ui *userInfo) {
	c.Set("uidOrigin", ui.Subject)
	c.Set("userName", ui.Name)
	c.Set(UserGroupsContextKey, ui.Groups)
}

// clearExpired delete expired tokens from the cache
func (om *oidcManager) clearExpired() {
	om.tokenCacheMutex.RLock()
	tokens := make([]string, 0, len(om.tokenCache))
	for rawToken := range om.tokenCache {
		tokens = append(tokens, rawToken)
	}
	om.tokenCacheMutex.RUnlock()

	for _, rawToken := range tokens {
		_, err := om.provider.Verifier(om.config).Verify(context.TODO(), rawToken)
		if err != nil {
			om.tokenCacheMutex.Lock()
			delete(om.tokenCache, rawToken)
			om.tokenCacheMutex.Unlock()
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
		groups = getGroupsEGI(claims.Entitlements)
	} else if strings.Contains(providerAuth, AI4EOSCIssuer) {
		var claims KeycloakClaims
		cerr = ui.Claims(&claims)
		groups = getGroupsKeycloak(ui)
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

// getGroups transforms "entitlements" EGI URNs to a slice of group fields

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

func getGroupsKeycloak(ui *oidc.UserInfo) []string {
	var claims GroupAI4EOSC
	cerr := ui.Claims(&claims)
	if cerr != nil {
		return []string{}
	}
	memberships := claims.RealmAccess.Roles
	return memberships

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
	_, ok := om.authorizedUserInfo(rawToken)
	return ok
}

func (om *oidcManager) authorizedUserInfo(rawToken string) (*userInfo, bool) {
	// Check if the token is valid
	_, err := om.provider.Verifier(om.config).Verify(context.TODO(), rawToken)
	if err != nil {
		return nil, false
	}

	// Check if token is in cache
	om.tokenCacheMutex.RLock()
	ui, found := om.tokenCache[rawToken]
	om.tokenCacheMutex.RUnlock()
	if !found {
		// Get userInfo from the issuer
		ui, err = om.GetUserInfo(rawToken)
		if err != nil {
			return nil, false
		}
		// Store userInfo in cache
		om.tokenCacheMutex.Lock()
		om.tokenCache[rawToken] = ui
		om.tokenCacheMutex.Unlock()

		// Call clearExpired to delete expired tokens
		om.clearExpired()
	}

	// Groups
	for _, tokenGroup := range ui.Groups {
		for _, authGroup := range om.groups {
			if tokenGroup == authGroup {
				return ui, true
			}
		}
	}

	return nil, false
}

func (om *oidcManager) UserInOneGroup(ui *userInfo, cfg *types.Config) bool {
	for _, vo := range cfg.OIDCGroups {
		if om.UserHasVO(ui, vo) {
			return true
		}
	}
	return false
}
