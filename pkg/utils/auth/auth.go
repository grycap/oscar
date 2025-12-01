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
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"k8s.io/client-go/kubernetes"
)

// GetAuthMiddleware returns the appropriate gin auth middleware
func GetAuthMiddleware(cfg *types.Config, kubeClientset kubernetes.Interface) gin.HandlerFunc {
	if !cfg.OIDCEnable {
		return gin.BasicAuth(gin.Accounts{
			// Use the config's username and password for basic auth
			cfg.Username: cfg.Password,
		})
	}
	return CustomAuth(cfg, kubeClientset)
}

// CustomAuth returns a custom auth handler (gin middleware)
func CustomAuth(cfg *types.Config, kubeClientset kubernetes.Interface) gin.HandlerFunc {
	basicAuthHandler := gin.BasicAuth(gin.Accounts{
		// Use the config's username and password for basic auth
		cfg.Username: cfg.Password,
	})

	minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)
	// Slice to add default user to all users group on MinIO
	var oscarUser = []string{"console"}

	minIOAdminClient.CreateAllUsersGroup()                               // #nosec G104
	minIOAdminClient.CreateAddGroup("all_users_group", oscarUser, false) // #nosec G104

	oidcHandler := getOIDCMiddleware(kubeClientset, minIOAdminClient, cfg, nil)
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			oidcHandler(c)
		} else {
			basicAuthHandler(c)
		}
	}
}

// GetLoggerMiddleware returns a gin handler as middleware to log custom info about sync/async executions
func GetLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		// Disable default printf timestamp to avoid inconsistencies on logs
		log.SetFlags(0)

		startTime := time.Now()

		// Process request
		c.Next()

		endTime := time.Now()

		// Log custom information after the request is processed
		logTime := endTime.Format("2006/01/02 - 15:04:05")
		latency := time.Since(startTime)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path

		// Get EGI UID from context (if OIDC auth is used)
		uid, uidExists := c.Get("uidOrigin")
		var user string
		if uidExists {
			user, _ = uid.(string)
		} else {
			// Set OSCAR as default user when no UID is found
			user = "oscar"
		}

		// Get source IP from context for jobs triggered through MinIO events
		IPAddress, AddressExists := c.Get("IPAddress")
		if AddressExists {
			clientIP, _ = IPAddress.(string)
		}

		log.Printf("[GIN-EXECUTIONS-LOGGER] %s | %3d | %13v | %s | %-7s %s | %s",
			logTime, status, latency, clientIP, method, path, user)
	}
}

func GetUIDFromContext(c *gin.Context) (string, error) {
	uidOrigin, uidExists := c.Get("uidOrigin")
	if !uidExists || uidOrigin == "" {
		return "", fmt.Errorf("missing user identificator")
	}
	uid, uidParsed := uidOrigin.(string)
	if !uidParsed {
		return "", fmt.Errorf("error parsing uid origin: %v", uidParsed)
	}
	return uid, nil
}

func GetUserNameFromContext(c *gin.Context) string {
	userName := c.GetString("userName")
	return userName
}

func GetMultitenancyConfigFromContext(c *gin.Context) (*MultitenancyConfig, error) {
	mcUntyped, mcExists := c.Get("multitenancyConfig")
	if !mcExists {
		return nil, fmt.Errorf("missing multitenancy config")
	}
	mc, mcParsed := mcUntyped.(*MultitenancyConfig)
	if !mcParsed {
		return nil, fmt.Errorf("error parsing multitenancy config")
	}
	return mc, nil
}
