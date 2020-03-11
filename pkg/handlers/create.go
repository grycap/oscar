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
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultMemory   = "256Mi"
	defaultCPU      = "0.2"
	defaultLogLevel = "INFO"
)

// MakeCreateHandler makes a handler to create services
func MakeCreateHandler(cfg *types.Config, kubeClientset *kubernetes.Clientset, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		var service types.Service
		if err := c.ShouldBindJSON(&service); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The service specification is not valid: %v", err))
			return
		}
		addDefaultValues(&service, cfg)

		// Create the service
		if err := back.CreateService(service); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating the service: %v", err))
			return
		}

		// // Register minio webhook and restart the server
		// minIOAdminClient, err := utils.MakeMinIOAdminClient(service.StorageProviders.MinIO, cfg)
		// if err != nil {
		// 	back.DeleteService(service.Name)
		// 	c.String(http.StatusInternalServerError, fmt.Sprintf("The provided MinIO configuration is not valid: %v", err))
		// 	return
		// }
		// if err := minIOAdminClient.RegisterWebhook(service.Name); err != nil {
		// 	back.DeleteService(service.Name)
		// 	c.String(http.StatusInternalServerError, fmt.Sprintf("Error registering the service's webhook: %v", err))
		// 	return
		// }
		// if err := minIOAdminClient.RestartServer(); err != nil {
		// 	back.DeleteService(service.Name)
		// 	c.String(http.StatusInternalServerError, err.Error())
		// 	return
		// }

		// // Create buckets/folders based on the Input []StorageIOConfig
		// if err := createBuckets(service.Input, service.StorageProviders); err != nil {
		// 	back.DeleteService(service.Name)
		// 	c.String(http.StatusInternalServerError, err.Error())
		// 	return
		// }

		// TODO: Register S3/Minio notifications based on the Input []StorageIOConfig

		c.Status(http.StatusCreated)
	}
}

func addDefaultValues(service *types.Service, cfg *types.Config) {
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

func createBuckets(input []types.StorageIOConfig, providers *types.StorageProviders) error {
	// MinIO
	if providers.MinIO != nil {
		// Create s3 (for MinIO) client
		s3MinIOConfig := &aws.Config{
			Credentials:      credentials.NewStaticCredentials(providers.MinIO.AccessKey, providers.MinIO.SecretKey, ""),
			Endpoint:         aws.String(providers.MinIO.Endpoint.String()),
			Region:           aws.String(providers.MinIO.Region),
			DisableSSL:       aws.Bool(!providers.MinIO.Verify),
			S3ForcePathStyle: aws.Bool(true),
		}
		minIOSession := session.New(s3MinIOConfig)
		minIOClient := s3.New(minIOSession)
		// TODO: finish
	}
	// TODO: S3 Support
	// TODO: Onedata Support (define a CDMI client)
	// Functionality to retrieve the oscar service/loadbalancer external IP
	// and port/nodeport is required to support external storage providers
	return nil
}
