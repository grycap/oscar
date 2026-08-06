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
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-yaml"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// MakeListSecretsHandler godoc
// @Summary List service secrets
// @Description List the environment secrets of the accessible services. Secret values are only returned for services owned by the caller.
// @Tags secrets
// @Produce json
// @Success 200 {array} types.ServiceSecrets
// @Failure 401 {string} string "Unauthorized"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/secrets [get]
func MakeListSecretsHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := ""
		var err error
		isBearer := isBearerRequest(c)
		if isBearer {
			uid, err = auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusUnauthorized, err.Error())
				return
			}
		}
		namespace := utils.BuildUserNamespace(cfg, uid)
		services, err := back.ListServices(namespace)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		response := []types.ServiceSecrets{}
		for _, service := range services {
			if isBearer && !isServiceAccessibleByUser(service, uid) {
				continue
			}

			if namespace == "" || !utils.SecretExists(service.Name, namespace, kubeClientset) {
				continue
			}

			secrets, err := readServiceSecretData(service.Name, namespace, kubeClientset)
			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}
			if isBearer && service.Owner != uid {
				for key := range secrets {
					secrets[key] = ""
				}
			}
			response = append(response, types.ServiceSecrets{
				Service: service.Name,
				Secrets: secrets,
			})
		}

		c.JSON(http.StatusOK, response)
	}
}

// MakeGetServiceSecretHandler godoc
// @Summary Get service secrets
// @Description Get the environment secrets of a specific service.
// @Tags secrets
// @Produce json
// @Param serviceName path string true "Service name"
// @Success 200 {object} types.ServiceSecrets
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/secrets/{serviceName} [get]
func MakeGetServiceSecretHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		serviceName, ok := validateServiceName(c, c.Param("serviceName"))

		if !ok {
			c.String(http.StatusBadRequest, serviceName)
			return
		}

		uid := ""
		var err error
		isBearer := isBearerRequest(c)
		if isBearer {
			uid, err = auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusUnauthorized, err.Error())
				return
			}
		}
		namespace := utils.BuildUserNamespace(cfg, uid)

		secrets, err := readServiceSecretData(serviceName, namespace, kubeClientset)
		if err != nil {
			if apierrors.IsNotFound(err) {
				c.Status(http.StatusNotFound)
				return
			}
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, types.ServiceSecrets{Service: serviceName, Secrets: secrets})
	}
}

// MakeUpdateServiceSecretHandler godoc
// @Summary Update service secrets
// @Description Merge the given key-value pairs into the service secret. Keys that do not exist yet are created and keys not present in the request are preserved.
// @Tags secrets
// @Accept json
// @Produce json
// @Param serviceName path string true "Service name"
// @Param secrets body types.SecretUpdateRequest true "Secrets to merge"
// @Success 200 {object} types.ServiceSecrets
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/secrets/{serviceName} [put]
func MakeUpdateServiceSecretHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		serviceName, ok := validateServiceName(c, c.Param("serviceName"))
		if !ok {
			c.String(http.StatusBadRequest, serviceName)
			return
		}

		var req types.SecretUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.String(http.StatusBadRequest, "The secret specification is not valid: %v", err)
			return
		}
		if len(req.Secrets) == 0 {
			c.String(http.StatusBadRequest, "The secret specification must include at least one key")
			return
		}
		for key := range req.Secrets {
			if key == types.RefreshTokenSecretKey {
				c.String(http.StatusBadRequest, "secret key %q is reserved and cannot be modified", key)
				return
			}
		}

		service, ok := getAuthorizedServiceOwner(c, back, serviceName)
		if !ok {
			return
		}

		namespace := resolveServiceNamespace(service, cfg)
		if err := utils.MergeSecretData(service.Name, namespace, req.Secrets, kubeClientset); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		if err := updateServiceConfigMapSecrets(service.Name, namespace, req.Secrets, kubeClientset); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		/*secrets, err := readServiceSecretData(service.Name, namespace, kubeClientset)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}*/

		c.JSON(http.StatusOK, types.ServiceSecrets{Service: service.Name, Secrets: req.Secrets})
	}
}

// updateServiceConfigMapSecrets adds the given secret keys to the service FDL
// stored in the service configMap, keeping the existing keys and using empty
// values (the secret values only live in the Kubernetes Secret).
func updateServiceConfigMapSecrets(serviceName string, namespace string, secretKeys map[string]string, kubeClientset kubernetes.Interface) error {
	cm, err := kubeClientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	service := &types.Service{}
	if err := yaml.Unmarshal([]byte(cm.Data[types.FDLFileName]), service); err != nil {
		return err
	}
	if service.Environment.Secrets == nil {
		service.Environment.Secrets = map[string]string{}
	}
	for key := range secretKeys {
		service.Environment.Secrets[key] = ""
	}

	fdl, err := service.ToYAML()
	if err != nil {
		return err
	}
	cm.Data[types.FDLFileName] = fdl

	_, err = kubeClientset.CoreV1().ConfigMaps(namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
	return err
}

func readServiceSecretData(secretName string, namespace string, kubeClientset kubernetes.Interface) (map[string]string, error) {
	secret, err := utils.GetSecret(secretName, namespace, kubeClientset)
	if err != nil {
		return nil, err
	}
	data := map[string]string{}
	for key, value := range secret.Data {
		data[key] = string(value)
	}
	return data, nil
}
