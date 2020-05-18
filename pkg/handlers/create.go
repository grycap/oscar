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
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/cdmi-client-go"
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

// MakeCreateHandler makes a handler for creating services
func MakeCreateHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		var service types.Service
		if err := c.ShouldBindJSON(&service); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The service specification is not valid: %v", err))
			return
		}

		// Check service values and set defaults
		if err := checkValues(&service, cfg); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The service specification is not valid: %v", err))
			return
		}

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
		if err := registerMinIOWebhook(service.Name, service.StorageProviders.MinIO, cfg); err != nil {
			back.DeleteService(service.Name)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		// Create buckets/folders based on the Input and Output and enable notifications
		if err := createBuckets(&service); err != nil {
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

func checkValues(service *types.Service, cfg *types.Config) error {
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

	// Check if MinIO provider is defined and valid
	if service.StorageProviders != nil {
		if service.StorageProviders.MinIO != nil {
			if !reflect.DeepEqual(*cfg.MinIOProvider, *service.StorageProviders.MinIO) {
				return fmt.Errorf("The provided MinIO server \"%s\" is not the configured in OSCAR", service.StorageProviders.MinIO.Endpoint)
			}
		} else {
			service.StorageProviders.MinIO = cfg.MinIOProvider
		}
	} else {
		service.StorageProviders = &types.StorageProviders{MinIO: cfg.MinIOProvider}
	}

	return nil
}

func createBuckets(service *types.Service) error {
	// Create S3 client for MinIO
	minIOClient := service.StorageProviders.MinIO.GetS3Client()

	// Create S3 client for Amazon S3
	var s3Client *s3.S3
	if service.StorageProviders.S3 != nil {
		s3Client = service.StorageProviders.S3.GetS3Client()
	}

	// Create Onedata CDMI client
	var cdmiClient *cdmi.Client
	if service.StorageProviders.Onedata != nil {
		cdmiClient = service.StorageProviders.Onedata.GetCDMIClient()
	}

	// Create input buckets
	for _, in := range service.Input {
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
				return fmt.Errorf("Error creating bucket %s: %v", splitPath[0], err)
			}
		}
		// Create folder(s)
		if len(splitPath) == 2 {
			// Add "/" to the end of the key in order to create a folder
			folderKey := fmt.Sprintf("%s/", splitPath[1])
			_, err := minIOClient.PutObject(&s3.PutObjectInput{
				Bucket: aws.String(splitPath[0]),
				Key:    aws.String(folderKey),
			})
			if err != nil {
				return fmt.Errorf("Error creating folder \"%s\" in bucket \"%s\": %v", folderKey, splitPath[0], err)
			}
		}

		// Enable MinIO notifications based on the Input []StorageIOConfig
		if err := enableInputNotification(minIOClient, service.GetMinIOWebhookARN(), in); err != nil {
			return err
		}

	}

	// Create output buckets
	for _, out := range service.Output {
		path := strings.Trim(out.Path, " /")
		// Split buckets and folders from path
		splitPath := strings.SplitN(path, "/", 2)

		provName := strings.ToLower(out.Provider)
		switch provName {
		case "minio", "s3":
			// Use the appropiate client
			var client *s3.S3
			if provName == "minio" {
				client = minIOClient
			} else {
				if s3Client != nil {
					client = s3Client
				} else {
					return fmt.Errorf("Output \"%s\" cannot be created. S3 provider not defined", path)
				}
			}
			// Create bucket
			_, err := client.CreateBucket(&s3.CreateBucketInput{
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
				return fmt.Errorf("Error creating bucket %s: %v", splitPath[0], err)
			}
			// Create folder(s)
			if len(splitPath) == 2 {
				// Add "/" to the end of the key in order to create a folder
				folderKey := fmt.Sprintf("%s/", splitPath[1])
				_, err := client.PutObject(&s3.PutObjectInput{
					Bucket: aws.String(splitPath[0]),
					Key:    aws.String(folderKey),
				})
				if err != nil {
					return fmt.Errorf("Error creating folder \"%s\" in bucket \"%s\": %v", folderKey, splitPath[0], err)
				}
			}
		case "onedata":
			if cdmiClient != nil {
				err := cdmiClient.CreateContainer(fmt.Sprintf("%s/%s", service.StorageProviders.Onedata.Space, path), true)
				if err != nil {
					if err != cdmi.ErrBadRequest {
						log.Printf("Error creating \"%s\" folder in Onedata. Error: %v", path, err)
					}
				}
			} else {
				return fmt.Errorf("Output \"%s\" cannot be created. Onedata provider not defined", path)
			}

		}
	}

	return nil
}

func registerMinIOWebhook(name string, minIO *types.MinIOProvider, cfg *types.Config) error {
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

func enableInputNotification(minIOClient *s3.S3, arnStr string, input types.StorageIOConfig) error {
	path := strings.Trim(input.Path, " /")
	// Split buckets and folders from path
	splitPath := strings.SplitN(path, "/", 2)

	// Get current BucketNotificationConfiguration
	gbncRequest := &s3.GetBucketNotificationConfigurationRequest{
		Bucket: aws.String(splitPath[0]),
	}
	nCfg, err := minIOClient.GetBucketNotificationConfiguration(gbncRequest)
	if err != nil {
		return fmt.Errorf("Error getting bucket \"%s\" notifications: %v", splitPath[0], err)
	}
	queueConfiguration := s3.QueueConfiguration{
		QueueArn: aws.String(arnStr),
		Events:   []*string{aws.String(s3.EventS3ObjectCreated)},
	}

	// Add folder filter if required
	if len(splitPath) == 2 {
		queueConfiguration.Filter = &s3.NotificationConfigurationFilter{
			Key: &s3.KeyFilter{
				FilterRules: []*s3.FilterRule{
					{
						Name:  aws.String(s3.FilterRuleNamePrefix),
						Value: aws.String(fmt.Sprintf("%s/", splitPath[1])),
					},
				},
			},
		}
	}

	// Append the new queueConfiguration
	nCfg.QueueConfigurations = append(nCfg.QueueConfigurations, &queueConfiguration)
	pbncInput := &s3.PutBucketNotificationConfigurationInput{
		Bucket:                    aws.String(splitPath[0]),
		NotificationConfiguration: nCfg,
	}

	// Enable the notification
	_, err = minIOClient.PutBucketNotificationConfiguration(pbncInput)
	if err != nil {
		return fmt.Errorf("Error enabling bucket notification: %v", err)
	}

	return nil
}
