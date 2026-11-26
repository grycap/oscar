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

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/backends"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	getUIDFromContextFn                = auth.GetUIDFromContext
	getMultitenancyConfigFromContextFn = auth.GetMultitenancyConfigFromContext
)

// MakeConfigHandler godoc
// @Summary Get configuration
// @Description Retrieve cluster configuration and MinIO credentials for the authenticated user.
// @Tags config
// @Produce json
// @Success 200 {object} types.ConfigForUser
// @Failure 401 {string} string "Unauthorized"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/config [get]
func MakeConfigHandler(cfg *types.Config, back kubernetes.Interface) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Return configForUser
		var conf types.ConfigForUser
		minIOProvider := cfg.MinIOProvider
		var air []string
		var afe []string
		cm, err := backends.GetOSCARCMConfiguration(back, cfg.AdditionalConfigPath, cfg.Namespace)
		if err != nil && !apierrors.IsNotFound(err) {
			c.String(http.StatusInternalServerError, fmt.Sprintln(err))

		}
		if apierrors.IsNotFound(err) {
			air = []string{}
			afe = []string{}
		} else {
			if data, ok := cm.Data[types.AIR]; ok {
				err = json.Unmarshal([]byte(data), &air)
				if err != nil {
					c.String(http.StatusInternalServerError, fmt.Sprintln(err))

				}
			}
			// The key may be absent in ConfigMaps created before the
			// allowed_federation_endpoints option existed.
			if data, ok := cm.Data[types.AFE]; ok {
				err = json.Unmarshal([]byte(data), &afe)
				if err != nil {
					c.String(http.StatusInternalServerError, fmt.Sprintln(err))

				}
			}
		}
		authHeader := c.GetHeader("Authorization")
		if len(strings.Split(authHeader, "Bearer")) == 1 {
			conf = types.ConfigForUser{
				Cfg:                        cfg,
				MinIOProvider:              minIOProvider,
				AllowedImageRepositories:   air,
				AllowedFederationEndpoints: afe,
			}
		} else {

			// Get MinIO credentials from k8s secret for user

			uid, err := getUIDFromContextFn(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
			}

			mc, err := getMultitenancyConfigFromContextFn(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
			}

			ak, sk, err := mc.GetUserCredentials(uid)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
			}

			userMinIOProvider := &types.MinIOProvider{
				Endpoint:  minIOProvider.Endpoint,
				Verify:    minIOProvider.Verify,
				AccessKey: ak,
				SecretKey: sk,
				Region:    minIOProvider.Region,
			}

			conf = types.ConfigForUser{
				Cfg:                        cfg,
				MinIOProvider:              userMinIOProvider,
				AllowedImageRepositories:   air,
				AllowedFederationEndpoints: afe,
			}
		}

		c.JSON(http.StatusOK, conf)
	}
}

// MakeConfigHandler godoc
// @Summary Put configuration
// @Description Change the cluster configuration
// @Tags config
// @Produce json
// @Success 200 {object} types.ConfigForUser
// @Failure 401 {string} string "Unauthorized"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Router /system/config [put]
func MakeConfigUpdateHandler(cfg *types.Config, back kubernetes.Interface) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if len(strings.Split(authHeader, "Bearer")) > 1 {
			c.JSON(http.StatusUnauthorized, "")
			return
		}
		configInput := types.ConfigForUser{}
		if err := c.ShouldBindJSON(&configInput); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The configuration specification is not valid: %v", err))
			return
		}
		if invalid := backends.ValidateAllowedImageRepositories(configInput.AllowedImageRepositories); len(invalid) > 0 {
			c.String(http.StatusBadRequest, strings.Join(invalid, "; "))
			return
		}
		if invalid := utils.ValidateFederationAllowlistEntries(configInput.AllowedFederationEndpoints); len(invalid) > 0 {
			c.String(http.StatusBadRequest, strings.Join(invalid, "; "))
			return
		}
		_, err := backends.GetOSCARCMConfiguration(back, cfg.AdditionalConfigPath, cfg.Namespace)

		if err != nil && !apierrors.IsNotFound(err) {
			c.JSON(http.StatusInternalServerError, err)
			return

		}
		if apierrors.IsNotFound(err) {
			if len(configInput.AllowedImageRepositories) == 0 {
				cm := getOSCARCMConfigurationDefaultDefinition(cfg.AdditionalConfigPath)
				err = backends.CreateOSCARCMConfiguration(back, cm, cfg.Namespace)
				if err != nil {
					c.JSON(http.StatusInternalServerError, err)
					return

				}
				c.JSON(http.StatusOK, configInput.AllowedImageRepositories)
				return

			} else {
				cm, err := getOSCARCMConfigurationCustomDefinition(cfg.AdditionalConfigPath, configInput.AllowedImageRepositories, configInput.AllowedFederationEndpoints)
				if err != nil {
					c.JSON(http.StatusInternalServerError, err)
					return
				}
				err = backends.CreateOSCARCMConfiguration(back, cm, cfg.Namespace)
				if err != nil {
					c.JSON(http.StatusInternalServerError, err)
					return

				}
				c.JSON(http.StatusOK, configInput.AllowedImageRepositories)
				return
			}
		}
		newConfigMap, err := getOSCARCMConfigurationCustomDefinition(cfg.AdditionalConfigPath, configInput.AllowedImageRepositories, configInput.AllowedFederationEndpoints)
		if err != nil {
			c.JSON(http.StatusInternalServerError, err)
			return

		}
		err = backends.UpdateOSCARCMConfiguration(back, newConfigMap, cfg.Namespace)
		if err != nil {
			c.JSON(http.StatusInternalServerError, err)
			return

		}
		c.JSON(http.StatusOK, configInput.AllowedImageRepositories)
	}
}

func getOSCARCMConfigurationDefaultDefinition(name string) *v1.ConfigMap {
	// Both keys are always written so that a later update (PUT /system/config)
	// does not drop any of them from the ConfigMap.
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: map[string]string{
			types.AIR: "[]",
			types.AFE: "[]",
		},
	}
}

// getOSCARCMConfigurationCustomDefinition builds the additional configuration
// ConfigMap with both the allowed_image_repositories and the
// allowed_federation_endpoints keys, so updating one never wipes the other.
func getOSCARCMConfigurationCustomDefinition(name string, air, afe []string) (*v1.ConfigMap, error) {
	airData, err := json.Marshal(air)
	if err != nil {
		return nil, err
	}
	afeData, err := json.Marshal(afe)
	if err != nil {
		return nil, err
	}
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: map[string]string{
			types.AIR: string(airData),
			types.AFE: string(afeData),
		},
	}, nil
}
