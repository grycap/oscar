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
func GetAuthMiddleware(cfg *types.Config, kubeClientset *kubernetes.Clientset) gin.HandlerFunc {
	if !cfg.OIDCEnable {
		return gin.BasicAuth(gin.Accounts{
			// Use the config's username and password for basic auth
			cfg.Username: cfg.Password,
		})
	}
	return CustomAuth(cfg, kubeClientset)
}

// CustomAuth returns a custom auth handler (gin middleware)
func CustomAuth(cfg *types.Config, kubeClientset *kubernetes.Clientset) gin.HandlerFunc {
	basicAuthHandler := gin.BasicAuth(gin.Accounts{
		// Use the config's username and password for basic auth
		cfg.Username: cfg.Password,
	})

	minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)
	// Slice to add default user to all users group on MinIO
	var oscarUser = []string{"console"}

	minIOAdminClient.CreateAllUsersGroup()
	minIOAdminClient.UpdateUsersInGroup(oscarUser, "all_users_group", false)

	oidcHandler := getOIDCMiddleware(kubeClientset, minIOAdminClient, cfg.OIDCIssuer, cfg.OIDCSubject, cfg.OIDCGroups)
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			oidcHandler(c)
		} else {
			basicAuthHandler(c)
		}
	}
}

func GetLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Log custom information after the request is processed
		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path
		uid, _ := c.Get("uidOrigni")
		user, uidParsed := uid.(string)
		if !uidParsed {
			user = "nil"
		}

		log.Printf("[Gin logger] %v | %3d | %13v | %s | %-7s %#v\n | %s",
			time.Now().Format(time.RFC3339), status, latency, clientIP, method, path, user)
	}
}

func GetUIDFromContext(c *gin.Context) (string, error) {
	uidOrigin, uidExists := c.Get("uidOrigin")
	if !uidExists {
		return "", fmt.Errorf("Missing EGI user uid")
	}
	uid, uidParsed := uidOrigin.(string)
	if !uidParsed {
		return "", fmt.Errorf("Error parsing uid origin: %v", uidParsed)
	}
	return uid, nil
}

func GetMultitenancyConfigFromContext(c *gin.Context) (*MultitenancyConfig, error) {
	mcUntyped, mcExists := c.Get("multitenancyConfig")
	if !mcExists {
		return nil, fmt.Errorf("Missing multitenancy config")
	}
	mc, mcParsed := mcUntyped.(*MultitenancyConfig)
	if !mcParsed {
		return nil, fmt.Errorf("Error parsing multitenancy config")
	}
	return mc, nil
}
