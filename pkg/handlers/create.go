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
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/pkg/types"
	"github.com/grycap/oscar/pkg/utils"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	defaultMemory   = "256Mi"
	defaultCPU      = "0.2"
	defaultLogLevel = "INFO"
)

var errNoMinIOInput = errors.New("Only MinIO input allowed")

// MakeCreateHandler makes a handler to create services
func MakeCreateHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		var service types.Service
		if err := c.ShouldBindJSON(&service); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The service specification is not valid: %v", err))
			return
		}
		addDefaultValues(&service, cfg)

		// Create the service
		if err := back.CreateService(service); err != nil {
			// Check if error is caused because the service name provided already exists
			if k8sErrors.IsAlreadyExists(err) {
				c.String(http.StatusConflict, "A service with the provided name already exists")
			} else {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating the service: %v", err))
			}
			return
		}

		// Register minio webhook and restart the server
		if err := configureMinIO(service.Name, service.StorageProviders.MinIO, cfg); err != nil {
			back.DeleteService(service.Name)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		// Create buckets/folders based on the Input and Output
		if err := createBuckets(service.Input, service.Output, service.StorageProviders); err != nil {
			if err == errNoMinIOInput {
				c.String(http.StatusBadRequest, err.Error())
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			back.DeleteService(service.Name)
			return
		}

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
				Region:    cfg.MinIORegion,
			},
		}
	}
	if service.StorageProviders.MinIO == nil {
		service.StorageProviders.MinIO = &types.MinIOProvider{
			Endpoint:  cfg.MinIOEndpoint,
			Verify:    cfg.MinIOTLSVerify,
			AccessKey: cfg.MinIOAccessKey,
			SecretKey: cfg.MinIOSecretKey,
			Region:    cfg.MinIORegion,
		}
	}
}

func createBuckets(input []types.StorageIOConfig, output []types.StorageIOConfig, providers *types.StorageProviders) error {
	// Create S3 client for MinIO
	var minIOClient *s3.S3
	if providers.MinIO != nil {
		s3MinIOConfig := &aws.Config{
			Credentials:      credentials.NewStaticCredentials(providers.MinIO.AccessKey, providers.MinIO.SecretKey, ""),
			Endpoint:         aws.String(providers.MinIO.Endpoint.String()),
			Region:           aws.String(providers.MinIO.Region),
			DisableSSL:       aws.Bool(!providers.MinIO.Verify),
			S3ForcePathStyle: aws.Bool(true),
		}
		minIOSession := session.New(s3MinIOConfig)
		minIOClient = s3.New(minIOSession)
	}

	// Create S3 client for Amazon S3
	var s3Client *s3.S3
	if providers.S3 != nil {
		s3Config := &aws.Config{
			Credentials: credentials.NewStaticCredentials(providers.S3.AccessKey, providers.S3.SecretKey, ""),
			Region:      aws.String(providers.S3.Region),
		}
		s3Session := session.New(s3Config)
		s3Client = s3.New(s3Session)
	}

	// TODO: Onedata support

	// Create input buckets
	// TODO: finish...
	for _, in := range input {
		// Only allow input from MinIO
		if strings.ToLower(in.Provider) != "minio" {
			return errNoMinIOInput
		}
		path := strings.Trim(in.Path, " /")
		// Split buckets and folders from path
		splitPath := strings.SplitN(path, "/", 2)
		// Create bucket
		_, err := minIOClient.CreateBucket(&s3.CreateBucketInput{
			Bucket: aws.String(splitPath[0]),
		})
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				// Check if the error is caused because the bucket already exists
				if aerr.Code() == s3.ErrCodeBucketAlreadyExists || aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
					log.Printf("The bucket \"%s\" already exists\n", splitPath[0])
				}
			} else {
				return err
			}
		}
		// TODO: Create folder(s)
		// TODO: Register MinIO notifications based on the Input []StorageIOConfig
	}

	// Create output buckets
	for _, out := range output {
		path := strings.Trim(out.Path, " /")
		// Split buckets and folders from path
		splitPath := strings.SplitN(path, "/", 2)

		switch strings.ToLower(out.Provider) {
		case "minio":
			// Create bucket
			_, err := minIOClient.CreateBucket(&s3.CreateBucketInput{
				Bucket: aws.String(splitPath[0]),
			})
			if err != nil {
				if aerr, ok := err.(awserr.Error); ok {
					// Check if the error is caused because the bucket already exists
					if aerr.Code() == s3.ErrCodeBucketAlreadyExists || aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
						log.Printf("The bucket \"%s\" already exists\n", splitPath[0])
						continue
					}
				}
				return err
			}
			// TODO: Create folder(s)
			if len(splitPath) == 2 {
				// Add "/" to the end of the key in order to create a folder
				folderKey := fmt.Sprint("%s/", splitPath[1])
				_, err := minIOClient.PutObject(&s3.PutObjectInput{
					Bucket: aws.String(splitPath[0]),
					Key:    aws.String(folderKey),
				})
				// TODO: finish...
			}
		case "s3":
			// TODO: Use the same code as for minIO, only change the client...
			// TODO: Create bucket
			// TODO: Create folder(s)

		}
	}

	return nil
}

func configureMinIO(name string, minIO *types.MinIOProvider, cfg *types.Config) error {
	minIOAdminClient, err := utils.MakeMinIOAdminClient(minIO, cfg)
	if err != nil {
		return fmt.Errorf("The provided MinIO configuration is not valid: %v", err)
	}

	if err := minIOAdminClient.RegisterWebhook(name); err != nil {
		return fmt.Errorf("Error registering the service's webhook: %v", err)
	}

	if err := minIOAdminClient.RestartServer(); err != nil {
		return err
	}

	return nil
}
