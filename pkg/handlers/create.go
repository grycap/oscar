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
	"os"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/cdmi-client-go"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	defaultMemory   = "256Mi"
	defaultCPU      = "0.2"
	defaultLogLevel = "INFO"
	createPath      = "/system/services"
)

var errInput = errors.New("unrecognized input (valid inputs are MinIO and dCache)")

// Custom logger
var createLogger = log.New(os.Stdout, "[CREATE-HANDLER] ", log.Flags())
var isAdminUser = false

// MakeCreateHandler makes a handler for creating services
func MakeCreateHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		var service types.Service
		authHeader := c.GetHeader("Authorization")
		if len(strings.Split(authHeader, "Bearer")) == 1 {
			isAdminUser = true
			service.Owner = "cluster_admin"
			createLogger.Printf("Creating service '%s' for user '%s'", service.Name, service.Owner)
		}

		if err := c.ShouldBindJSON(&service); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The service specification is not valid: %v", err))
			return
		}

		// Check service values and set defaults
		checkValues(&service, cfg)
		// Check if users in allowed_users have a MinIO associated user
		minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)

		// Service is created by an EGI user
		if !isAdminUser {
			uid, err := auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
				return
			}

			// Set UID from owner
			service.Owner = uid
			createLogger.Printf("Creating service '%s' for user '%s'", service.Name, service.Owner)

			mc, err := auth.GetMultitenancyConfigFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
				return
			}

			full_uid := auth.FormatUID(uid)
			// Check if the service VO is present on the cluster VO's and if the user creating the service is enrrolled in such
			if service.VO != "" {
				for _, vo := range cfg.OIDCGroups {
					if vo == service.VO {
						err := checkIdentity(&service, cfg, authHeader)
						if err != nil {
							c.String(http.StatusBadRequest, fmt.Sprintln(err))
							return
						}
						break
					}
				}
			} else {
				if len(cfg.OIDCGroups) != 0 {
					c.String(http.StatusBadRequest, fmt.Sprintln("service must be part of one of the following VO: ", cfg.OIDCGroups))
				}
			}

			if len(service.AllowedUsers) > 0 {
				// If AllowedUsers is empty don't add uid
				service.Labels["uid"] = full_uid[0:8]

				// If the uid of the owner is not on the allowed_users list append it
				ownerOnList := false
				for _, user := range service.AllowedUsers {
					if user == service.Owner {
						ownerOnList = true
						break
					}
				}
				if !ownerOnList {
					service.AllowedUsers = append(service.AllowedUsers, uid)
				}
				// Check if the uid's from allowed_users have and asociated MinIO user
				// and create it if not
				uids := mc.CheckUsersInCache(service.AllowedUsers)
				if len(uids) > 0 {
					for _, uid := range uids {
						sk, _ := auth.GenerateRandomKey(8)
						minIOAdminClient.CreateMinIOUser(uid, sk)
						mc.CreateSecretForOIDC(uid, sk)
					}
				}
			}
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
		if err := registerMinIOWebhook(service.Name, service.Token, service.StorageProviders.MinIO[types.DefaultProvider], cfg); err != nil {
			back.DeleteService(service)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		// Create buckets/folders based on the Input and Output and enable notifications
		if err := createBuckets(&service, cfg, minIOAdminClient, service.AllowedUsers, false); err != nil {
			if err == errInput {
				c.String(http.StatusBadRequest, err.Error())
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			back.DeleteService(service)
			return
		}

		// Add Yunikorn queue if enabled
		if cfg.YunikornEnable {
			if err := utils.AddYunikornQueue(cfg, back.GetKubeClientset(), &service); err != nil {
				log.Println(err.Error())
			}
		}
		uid := service.Owner
		if service.Owner == "" {
			uid = "nil"
		}
		createLogger.Printf("%s | %v | %s | %s | %s", "POST", 200, createPath, service.Name, uid)
		c.Status(http.StatusCreated)
	}
}

func checkValues(service *types.Service, cfg *types.Config) {
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

	// Add default Labels
	if service.Labels == nil {
		service.Labels = make(map[string]string)
	}
	service.Labels[types.ServiceLabel] = service.Name
	service.Labels[types.YunikornApplicationIDLabel] = service.Name
	service.Labels[types.YunikornQueueLabel] = fmt.Sprintf("%s.%s.%s", types.YunikornRootQueue, types.YunikornOscarQueue, service.Name)

	// Create default annotations map
	if service.Annotations == nil {
		service.Annotations = make(map[string]string)
	}

	// Add the default MinIO provider
	if service.StorageProviders != nil {
		if service.StorageProviders.MinIO != nil {
			service.StorageProviders.MinIO[types.DefaultProvider] = cfg.MinIOProvider
		} else {
			service.StorageProviders.MinIO = map[string]*types.MinIOProvider{
				types.DefaultProvider: cfg.MinIOProvider,
			}
		}
	} else {
		service.StorageProviders = &types.StorageProviders{
			MinIO: map[string]*types.MinIOProvider{
				types.DefaultProvider: cfg.MinIOProvider,
			},
		}
	}

	// Generate a new access token
	service.Token = utils.GenerateToken()
}

func createBuckets(service *types.Service, cfg *types.Config, minIOAdminClient *utils.MinIOAdminClient, allowed_users []string, isUpdate bool) error {
	var s3Client *s3.S3
	var cdmiClient *cdmi.Client
	var provName, provID string

	// Create input buckets
	for _, in := range service.Input {
		provID, provName = getProviderInfo(in.Provider)

		// Only allow input from MinIO and dCache
		if provName != types.MinIOName && provName != types.WebDavName {
			return errInput
		}

		// If the provider is WebDav (dCache) skip bucket creation
		if provName == types.WebDavName {
			continue
		}

		// Check if the provider identifier is defined in StorageProviders
		if !isStorageProviderDefined(provName, provID, service.StorageProviders) {
			return fmt.Errorf("the StorageProvider \"%s.%s\" is not defined", provName, provID)
		}

		// Check if the input provider is the defined in the server config
		if provID != types.DefaultProvider {
			if !reflect.DeepEqual(*cfg.MinIOProvider, *service.StorageProviders.MinIO[provID]) {
				return fmt.Errorf("the provided MinIO server \"%s\" is not the configured in OSCAR", service.StorageProviders.MinIO[provID].Endpoint)
			}
		}

		// Get client for the provider
		s3Client = service.StorageProviders.MinIO[provID].GetS3Client()

		path := strings.Trim(in.Path, " /")
		// Split buckets and folders from path
		splitPath := strings.SplitN(path, "/", 2)
		// Create bucket
		_, err := s3Client.CreateBucket(&s3.CreateBucketInput{
			Bucket: aws.String(splitPath[0]),
		})
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				// Check if the error is caused because the bucket already exists
				if aerr.Code() == s3.ErrCodeBucketAlreadyExists || aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
					log.Printf("The bucket \"%s\" already exists\n", splitPath[0])
				} else {
					return fmt.Errorf("error creating bucket %s: %v", splitPath[0], err)
				}
			} else {
				return fmt.Errorf("error creating bucket %s: %v", splitPath[0], err)
			}
		}

		// Create group for the service and add users
		// Check if users in allowed_users have a MinIO associated user
		// If new allowed users list is empty the service becames public
		if !isUpdate {
			if !isAdminUser {
				if len(allowed_users) == 0 {
					err = minIOAdminClient.AddServiceToAllUsersGroup(splitPath[0])
					if err != nil {
						return fmt.Errorf("error adding service %s to all users group: %v", splitPath[0], err)
					}
				} else {
					err = minIOAdminClient.CreateServiceGroup(splitPath[0])
					if err != nil {
						return fmt.Errorf("error creating service group for bucket %s: %v", splitPath[0], err)
					}

					err = minIOAdminClient.UpdateUsersInGroup(allowed_users, splitPath[0], false)
					if err != nil {
						return err
					}
				}
			}
		}
		// Create folder(s)
		if len(splitPath) == 2 {
			// Add "/" to the end of the key in order to create a folder
			folderKey := fmt.Sprintf("%s/", splitPath[1])
			_, err := s3Client.PutObject(&s3.PutObjectInput{
				Bucket: aws.String(splitPath[0]),
				Key:    aws.String(folderKey),
			})
			if err != nil {
				return fmt.Errorf("error creating folder \"%s\" in bucket \"%s\": %v", folderKey, splitPath[0], err)
			}
		}

		// Enable MinIO notifications based on the Input []StorageIOConfig
		if err := enableInputNotification(s3Client, service.GetMinIOWebhookARN(), in); err != nil {
			return err
		}
	}

	// Create output buckets
	for _, out := range service.Output {
		provID, provName = getProviderInfo(out.Provider)
		// Check if the provider identifier is defined in StorageProviders
		if !isStorageProviderDefined(provName, provID, service.StorageProviders) {
			disableInputNotifications(service.GetMinIOWebhookARN(), service.Input, cfg.MinIOProvider)
			return fmt.Errorf("the StorageProvider \"%s.%s\" is not defined", provName, provID)
		}

		path := strings.Trim(out.Path, " /")
		// Split buckets and folders from path
		splitPath := strings.SplitN(path, "/", 2)

		switch provName {
		case types.MinIOName, types.S3Name:
			// Use the appropriate client
			if provName == types.MinIOName {
				s3Client = service.StorageProviders.MinIO[provID].GetS3Client()
			} else {
				s3Client = service.StorageProviders.S3[provID].GetS3Client()
			}
			// Create bucket
			_, err := s3Client.CreateBucket(&s3.CreateBucketInput{
				Bucket: aws.String(splitPath[0]),
			})
			if err != nil {
				if aerr, ok := err.(awserr.Error); ok {
					// Check if the error is caused because the bucket already exists
					if aerr.Code() == s3.ErrCodeBucketAlreadyExists || aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
						log.Printf("The bucket \"%s\" already exists\n", splitPath[0])
					} else {
						disableInputNotifications(service.GetMinIOWebhookARN(), service.Input, cfg.MinIOProvider)
						return fmt.Errorf("error creating bucket %s: %v", splitPath[0], err)
					}
				} else {
					disableInputNotifications(service.GetMinIOWebhookARN(), service.Input, cfg.MinIOProvider)
					return fmt.Errorf("error creating bucket %s: %v", splitPath[0], err)
				}
			}
			// Create folder(s)
			if len(splitPath) == 2 {
				// Add "/" to the end of the key in order to create a folder
				folderKey := fmt.Sprintf("%s/", splitPath[1])
				_, err := s3Client.PutObject(&s3.PutObjectInput{
					Bucket: aws.String(splitPath[0]),
					Key:    aws.String(folderKey),
				})
				if err != nil {
					disableInputNotifications(service.GetMinIOWebhookARN(), service.Input, cfg.MinIOProvider)
					return fmt.Errorf("error creating folder \"%s\" in bucket \"%s\": %v", folderKey, splitPath[0], err)
				}
			}
		case types.OnedataName:
			cdmiClient = service.StorageProviders.Onedata[provID].GetCDMIClient()
			err := cdmiClient.CreateContainer(fmt.Sprintf("%s/%s", service.StorageProviders.Onedata[provID].Space, path), true)
			if err != nil {
				if err == cdmi.ErrBadRequest {
					log.Printf("Error creating \"%s\" folder in Onedata. Error: %v\n", path, err)
				} else {
					disableInputNotifications(service.GetMinIOWebhookARN(), service.Input, cfg.MinIOProvider)
					return fmt.Errorf("error connecting to Onedata's Oneprovider \"%s\". Error: %v", service.StorageProviders.Onedata[provID].OneproviderHost, err)
				}
			}
		}
	}

	if service.Mount.Provider != "" {
		provID, provName = getProviderInfo(service.Mount.Provider)
		if provName == types.MinIOName {
			// Check if the provider identifier is defined in StorageProviders
			if !isStorageProviderDefined(provName, provID, service.StorageProviders) {
				return fmt.Errorf("the StorageProvider \"%s.%s\" is not defined", provName, provID)
			}

			path := strings.Trim(service.Mount.Path, " /")
			// Split buckets and folders from path
			splitPath := strings.SplitN(path, "/", 2)

			// Currently only MinIO/S3 are supported
			// Use the appropriate client
			if provName == types.MinIOName {
				s3Client = service.StorageProviders.MinIO[provID].GetS3Client()
			} else {
				s3Client = service.StorageProviders.S3[provID].GetS3Client()
			}
			// Create bucket
			_, err := s3Client.CreateBucket(&s3.CreateBucketInput{
				Bucket: aws.String(splitPath[0]),
			})
			if err != nil {
				if aerr, ok := err.(awserr.Error); ok {
					// Check if the error is caused because the bucket already exists
					if aerr.Code() == s3.ErrCodeBucketAlreadyExists || aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
						log.Printf("The bucket \"%s\" already exists\n", splitPath[0])
					} else {
						return fmt.Errorf("error creating bucket %s: %v", splitPath[0], err)
					}
				} else {
					return fmt.Errorf("error creating bucket %s: %v", splitPath[0], err)
				}
			}
			if !isUpdate {
				if !isAdminUser {
					if len(allowed_users) == 0 {
						err = minIOAdminClient.AddServiceToAllUsersGroup(splitPath[0])
						if err != nil {
							return fmt.Errorf("error adding service %s to all users group: %v", splitPath[0], err)
						}
					} else {
						err = minIOAdminClient.CreateServiceGroup(splitPath[0])
						if err != nil {
							return fmt.Errorf("error creating service group for bucket %s: %v", splitPath[0], err)
						}

						err = minIOAdminClient.UpdateUsersInGroup(allowed_users, splitPath[0], false)
						if err != nil {
							return err
						}
					}
				}
			}
			// Create folder(s)
			if len(splitPath) == 2 {
				// Add "/" to the end of the key in order to create a folder
				folderKey := fmt.Sprintf("%s/", splitPath[1])
				_, err := s3Client.PutObject(&s3.PutObjectInput{
					Bucket: aws.String(splitPath[0]),
					Key:    aws.String(folderKey),
				})
				if err != nil {
					return fmt.Errorf("error creating folder \"%s\" in bucket \"%s\": %v", folderKey, splitPath[0], err)
				}
			}
		}

	}

	return nil
}

func isStorageProviderDefined(storageName string, storageID string, providers *types.StorageProviders) bool {
	var ok = false
	switch storageName {
	case types.MinIOName:
		_, ok = providers.MinIO[storageID]
	case types.S3Name:
		_, ok = providers.S3[storageID]
	case types.OnedataName:
		_, ok = providers.Onedata[storageID]
	case types.WebDavName:
		_, ok = providers.WebDav[storageID]
	}
	return ok
}

func getProviderInfo(rawInfo string) (string, string) {
	var provID, provName string
	// Split input provider
	provSlice := strings.SplitN(strings.TrimSpace(rawInfo), types.ProviderSeparator, 2)
	if len(provSlice) == 1 {
		provName = strings.ToLower(provSlice[0])
		// Set "default" provider ID
		provID = types.DefaultProvider
	} else {
		provName = strings.ToLower(provSlice[0])
		provID = provSlice[1]
	}
	return provID, provName
}

func checkIdentity(service *types.Service, cfg *types.Config, authHeader string) error {
	oidcManager, _ := auth.NewOIDCManager(cfg.OIDCIssuer, cfg.OIDCSubject, cfg.OIDCGroups)
	rawToken := strings.TrimPrefix(authHeader, "Bearer ")

	hasVO, err := oidcManager.UserHasVO(rawToken, service.VO)

	if err != nil {
		return err
	}

	if !hasVO {
		return fmt.Errorf("this user isn't enrrolled on the vo: %v", service.VO)
	}

	service.Labels["vo"] = service.VO

	return nil
}

func registerMinIOWebhook(name string, token string, minIO *types.MinIOProvider, cfg *types.Config) error {
	minIOAdminClient, err := utils.MakeMinIOAdminClient(cfg)
	if err != nil {
		return fmt.Errorf("the provided MinIO configuration is not valid: %v", err)
	}

	if err := minIOAdminClient.RegisterWebhook(name, token); err != nil {
		return fmt.Errorf("error registering the service's webhook: %v", err)
	}

	return minIOAdminClient.RestartServer()
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
		return fmt.Errorf("error getting bucket \"%s\" notifications: %v", splitPath[0], err)
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
		return fmt.Errorf("error enabling bucket notification: %v", err)
	}

	return nil
}
