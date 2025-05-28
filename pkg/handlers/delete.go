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
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	"k8s.io/apimachinery/pkg/api/errors"
)

var ALL_USERS_GROUP = "all_users_group"
var allUserGroupNotExist = "unable to remove bucket from policy \"" + ALL_USERS_GROUP + "\", policy '" + ALL_USERS_GROUP + "' does not exist"
var deleteLogger = log.New(os.Stdout, "[DELETE-HANDLER] ", log.Flags())

// MakeDeleteHandler makes a handler for deleting services
func MakeDeleteHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First get the Service
		service, _ := back.ReadService(c.Param("serviceName"))
		authHeader := c.GetHeader("Authorization")

		if len(strings.Split(authHeader, "Bearer")) > 1 {
			uid, err := auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
			}

			if service.Owner != uid {
				c.String(http.StatusForbidden, "User %s doesn't have permision to delete this service", uid)
				return
			}
		}
		if utils.SecretExists(service.Name, cfg.ServicesNamespace, back.GetKubeClientset()) {
			secretsErr := utils.DeleteSecret(service.Name, cfg.ServicesNamespace, back.GetKubeClientset())
			if secretsErr != nil {
				c.String(http.StatusInternalServerError, "Error deleting asociated secret: %v", secretsErr)
			}
		}
		if err := back.DeleteService(*service); err != nil {
			// Check if error is caused because the service is not found
			if errors.IsNotFound(err) || errors.IsGone(err) {
				c.Status(http.StatusNotFound)
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			return
		}
		minIOAdminClient, err := utils.MakeMinIOAdminClient(cfg)
		if err != nil {
			log.Printf("the provided MinIO configuration is not valid: %v", err)
		}

		if service.Mount.Path != "" {
			path := strings.Trim(service.Mount.Path, " /")
			// Split buckets and folders from path
			bucket := strings.SplitN(path, "/", 2)
			var users []string
			err = minIOAdminClient.CreateAddGroup(bucket[0], users, true)
			if err != nil {
				log.Printf("error updating MinIO users in group: %v", err)
			}
		}

		// Remove the service's webhook in MinIO config and restart the server
		if err := removeMinIOWebhook(service.Name, minIOAdminClient); err != nil {
			log.Printf("Error removing MinIO webhook for service \"%s\": %v\n", service.Name, err)
		}

		// Delete service buckets
		err = deleteBuckets(service, cfg, minIOAdminClient)
		if err != nil && !strings.Contains(err.Error(), allUserGroupNotExist) {
			c.String(http.StatusInternalServerError, "Error deleting service buckets: ", err)
		}

		if len(service.BucketList) > 0 && strings.ToUpper(service.IsolationLevel) == "USER" {
			for i, b := range service.BucketList {
				err = minIOAdminClient.RemoveResource(b, service.AllowedUsers[i], false)
				if err != nil {
					c.String(http.StatusInternalServerError, "error while removing isolated bucket %v", err)
				}
			}
		}

		// Add Yunikorn queue if enabled
		if cfg.YunikornEnable {
			if err := utils.DeleteYunikornQueue(cfg, back.GetKubeClientset(), service); err != nil {
				log.Println(err.Error())
			}
		}

		c.Status(http.StatusNoContent)
	}
}

func removeMinIOWebhook(name string, minIOAdminClient *utils.MinIOAdminClient) error {

	if err := minIOAdminClient.RemoveWebhook(name); err != nil {
		return fmt.Errorf("error removing the service's webhook: %v", err)
	}

	return minIOAdminClient.RestartServer()
}

func deleteBuckets(service *types.Service, cfg *types.Config, minIOAdminClient *utils.MinIOAdminClient) error {
	var s3Client *s3.S3
	var provName, provID string

	// Delete input buckets
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

		// Get admin client for the provider
		s3Client = cfg.MinIOProvider.GetS3Client()

		path := strings.Trim(in.Path, " /")
		// Split buckets and folders from path
		splitPath := strings.SplitN(path, "/", 2)
		err := DeleteMinIOBuckets(s3Client, minIOAdminClient, utils.MinIOBucket{
			BucketPath:   splitPath[0],
			Visibility:   service.Visibility,
			AllowedUsers: service.AllowedUsers,
			Owner:        service.Owner,
		})

		if err != nil {
			return fmt.Errorf("error while removing MinIO bucket %v", err)
		}

		// Disable input notifications for service bucket
		if err := disableInputNotifications(s3Client, service.GetMinIOWebhookARN(), splitPath[0]); err != nil {
			log.Printf("Error disabling MinIO input notifications for service \"%s\": %v\n", service.Name, err)
		}

	}

	// Delete output buckets
	for _, out := range service.Output {
		provID, provName = getProviderInfo(out.Provider)
		// Check if the provider identifier is defined in StorageProviders
		if !isStorageProviderDefined(provName, provID, service.StorageProviders) {
			return fmt.Errorf("the StorageProvider \"%s.%s\" is not defined", provName, provID)
		}

		switch provName {
		case types.MinIOName, types.S3Name:
			// TODO check if output is a different bucket and delete

		case types.OnedataName:
			// TODO
		}
	}

	// TODO check if some components of mount need to be deleted
	return nil
}

func DeleteMinIOBuckets(s3Client *s3.S3, minIOAdminClient *utils.MinIOAdminClient, bucket utils.MinIOBucket) error {
	var policyName string
	var isGroup bool
	if strings.ToLower(bucket.Visibility) == utils.PUBLIC {
		policyName = ALL_USERS_GROUP
		isGroup = true
	} else {
		policyName = bucket.Owner
	}

	err := minIOAdminClient.RemoveResource(bucket.BucketPath, policyName, isGroup)
	if err != nil {
		return fmt.Errorf("error removing resource")
	}

	if strings.ToLower(bucket.Visibility) == utils.RESTRICTED {
		err := minIOAdminClient.RemoveGroupPolicy(bucket.BucketPath)
		if err != nil {
			return fmt.Errorf("error removing policy for group")
		}
	}

	err = minIOAdminClient.DeleteBucket(s3Client, bucket.BucketPath)
	if err != nil {
		return fmt.Errorf("error deleting bucket %s, %v", bucket.BucketPath, err)
	}
	return nil
}

func disableInputNotifications(s3Client *s3.S3, arnStr string, bucket string) error {
	parsedARN, _ := arn.Parse(arnStr)

	// path := strings.Trim(in.Path, " /")
	// // Split buckets and folders from path
	// splitPath := strings.SplitN(path, "/", 2)

	updatedQueueConfigurations := []*s3.QueueConfiguration{}
	// Get bucket notification
	nCfg, err := s3Client.GetBucketNotificationConfiguration(&s3.GetBucketNotificationConfigurationRequest{Bucket: aws.String(bucket)})
	if err != nil {
		return fmt.Errorf("error getting bucket \"%s\" notifications: %v", bucket, err)
	}

	// Filter elements that doesn't match with service's ARN
	for _, q := range nCfg.QueueConfigurations {
		queueARN, _ := arn.Parse(*q.QueueArn)
		if queueARN.Resource == parsedARN.Resource &&
			queueARN.AccountID != parsedARN.AccountID {
			updatedQueueConfigurations = append(updatedQueueConfigurations, q)
		}
	}

	// Put the updated bucket configuration
	nCfg.QueueConfigurations = updatedQueueConfigurations
	pbncInput := &s3.PutBucketNotificationConfigurationInput{
		Bucket:                    aws.String(bucket),
		NotificationConfiguration: nCfg,
	}
	_, err = s3Client.PutBucketNotificationConfiguration(pbncInput)
	if err != nil {
		return fmt.Errorf("error disabling bucket notification: %v", err)
	}

	return nil
}
