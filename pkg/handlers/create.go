// Copyright (C) GRyCAP - I3M - UPV
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/pkg/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultMemory   = "256Mi"
	defaultCPU      = "0.2"
	defaultLogLevel = "INFO"
)

// MakeCreateHandler makes a handler to create services based on
func MakeCreateHandler(cfg types.Config, kubeClientset *kubernetes.Clientset, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		var service types.Service
		if err := c.ShouldBindJSON(&service); err != nil {
			c.String(http.StatusBadRequest, "The service specification is not valid")
			return
		}
		addDefaultValues(&service, cfg)

		// Create the configMap with FDL and user-script
		cm, err := createConfigMapSpec(service, cfg.Namespace)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error creating the service's configMap spec")
			return
		}
		_, err = kubeClientset.CoreV1().ConfigMaps(cfg.Namespace).Create(cm)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error creating the service's configMap")
			return
		}

		// Create the service
		err = back.CreateService(service)
		if err != nil {
			kubeClientset.CoreV1().ConfigMaps(cfg.Namespace).Delete(service.Name, &metav1.DeleteOptions{})
			c.String(http.StatusInternalServerError, "Error creating the service")
			return
		}

		// TODO: Register minio events if defined and restart the server

		c.Status(http.StatusCreated)
	}
}

func createConfigMapSpec(service types.Service, namespace string) (*v1.ConfigMap, error) {
	fdl, err := service.ToYAML()
	if err != nil {
		return nil, err
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"script.sh":            service.Script,
			"function_config.yaml": fdl,
		},
	}

	return cm, nil
}

func addDefaultValues(service *types.Service, cfg types.Config) {
	// Add default values for Memory and CPU if they are not set
	// Do not validate, Kubernetes client throws an error if they are not correct
	if service.Memory == "" {
		service.Memory = defaultMemory
	}
	if service.CPU == "" {
		service.CPU = defaultCPU
	}

	// Validate logLevel (Python logging levels for faas-supervisor)
	service.LogLevel = strings.ToUpper(service.LogLevel)
	switch service.LogLevel {
	case "NOTSET", "DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL":
	default:
		service.LogLevel = defaultLogLevel
	}

	// Add MinIO storage provider if not set
	if service.StorageProviders == nil {
		service.StorageProviders = &types.StorageProviders{
			MinIO: &types.MinIOProvider{
				Endpoint:  cfg.MinIOEndpoint,
				Verify:    cfg.MinIOTLSVerify,
				AccessKey: cfg.MinIOAccessKey,
				SecretKey: cfg.MinIOSecretKey,
			},
		}
	}
	if service.StorageProviders.MinIO == nil {
		service.StorageProviders.MinIO = &types.MinIOProvider{
			Endpoint:  cfg.MinIOEndpoint,
			Verify:    cfg.MinIOTLSVerify,
			AccessKey: cfg.MinIOAccessKey,
			SecretKey: cfg.MinIOSecretKey,
		}
	}
}
