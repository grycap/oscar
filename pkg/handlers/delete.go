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
var bucketNotExist = "NoSuchBucket: The specified bucket does not exist"
var deleteLogger = log.New(os.Stdout, "[DELETE-HANDLER] ", log.Flags())

// MakeDeleteHandler godoc
// @Summary Delete service
// @Description Delete an existing service by name.
// @Tags services
// @Produce json
// @Param serviceName path string true "Service name"
// @Success 204 {string} string "No Content"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/services/{serviceName} [delete]
func MakeDeleteHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First get the Service
		var service *types.Service
		var uid string
		var err error
		serviceName := c.Param("serviceName")
		namespaceArg := ""
		authHeader := c.GetHeader("Authorization")

		isOIDC := len(strings.Split(authHeader, "Bearer")) > 1
		if isOIDC {
			uid, err = auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
				return
			}
			namespaceArg = utils.BuildUserNamespace(cfg, uid)
		}

		service, err = back.ReadService(namespaceArg, serviceName)
		if err != nil {
			if errors.IsNotFound(err) || errors.IsGone(err) {
				c.Status(http.StatusNotFound)
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			return
		}

		if isOIDC && service.Owner != uid {
			c.String(http.StatusForbidden, "User %s doesn't have permision to delete this service", uid)
			return
		}
		if service.Namespace == "" {
			service.Namespace = cfg.ServicesNamespace
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
		if err != nil && !strings.Contains(err.Error(), allUserGroupNotExist) && !strings.Contains(err.Error(), bucketNotExist) {
			c.String(http.StatusInternalServerError, "Error deleting service buckets: ", err)
		}

		if len(service.BucketList) > 0 && strings.ToUpper(service.IsolationLevel) == types.IsolationLevelUser {
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
		if cfg.KueueEnable {
			if err := utils.DeleteKueueLocalQueue(c.Request.Context(), cfg, service.Namespace, service.Name); err != nil {
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
		if provName != types.MinIOName && provName != types.WebDavName && provName != types.RucioName {
			return errInput
		}

		// If the provider is WebDav (dCache) skip bucket creation
		if provName == types.WebDavName || provName == types.RucioName {
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

		// Disable input notifications for service bucket
		if err := disableInputNotifications(s3Client, service.GetMinIOWebhookARN(), splitPath[0]); err != nil {
			log.Printf("Error disabling MinIO input notifications for service \"%s\": %v\n", service.Name, err)
		}
		// Check if the bucket is in the mount path
		if !sameStorage(in, service.Mount) {
			err := DeleteMinIOBuckets(s3Client, minIOAdminClient, utils.MinIOBucket{
				BucketName:   splitPath[0],
				Visibility:   service.Visibility,
				AllowedUsers: service.AllowedUsers,
				Owner:        service.Owner,
			})

			if err != nil {
				return fmt.Errorf("error while removing MinIO bucket %v", err)
			}
		} else {
			// Bucket metadata for filtering
			tags := map[string]string{
				"owner":   service.Owner,
				"service": "false",
			}
			if err := minIOAdminClient.SetTags(splitPath[0], tags); err != nil {
				return fmt.Errorf("Error tagging bucket: %v", err)
			}
		}

	}

	// Delete output buckets
	for _, out := range service.Output {
		provID, provName = getProviderInfo(out.Provider)

		// If the provider is WebDav (dCache) skip bucket creation
		if provName == types.WebDavName || provName == types.RucioName {
			continue
		}

		// Check if the provider identifier is defined in StorageProviders
		if !isStorageProviderDefined(provName, provID, service.StorageProviders) {
			return fmt.Errorf("the StorageProvider \"%s.%s\" is not defined", provName, provID)
		}

		switch provName {
		case types.MinIOName, types.S3Name:
			//Check if this storage provider is defined in input
			previousExist := false
			outPath := strings.Trim(out.Path, " /")
			outBucket := strings.SplitN(outPath, "/", 2)[0]
			//Compare this output storage provider with all the input storage provider
			for _, in := range service.Input {
				//Don't compare in.Provider with out.Provider directly
				inProvID, inProvName := getProviderInfo(in.Provider)
				inPath := strings.Trim(in.Path, " /")
				inBucket := strings.SplitN(inPath, "/", 2)[0]

				if inProvID == provID && inProvName == provName && inBucket == outBucket {
					previousExist = true
				}
			}
			//Its is not defined in input -> delete.
			if !previousExist {
				s3Client = cfg.MinIOProvider.GetS3Client()

				// Disable input notifications for service bucket
				if err := disableInputNotifications(s3Client, service.GetMinIOWebhookARN(), outBucket); err != nil {
					log.Printf("Error disabling MinIO input notifications for service \"%s\": %v\n", service.Name, err)
				}
				if !sameStorage(out, service.Mount) {
					err := DeleteMinIOBuckets(s3Client, minIOAdminClient, utils.MinIOBucket{
						BucketName:   outBucket,
						Visibility:   service.Visibility,
						AllowedUsers: service.AllowedUsers,
						Owner:        service.Owner,
					})
					if err != nil {
						return fmt.Errorf("error while removing MinIO bucket %v", err)
					}
				} else {
					// Bucket metadata for filtering
					tags := map[string]string{
						"owner":   service.Owner,
						"service": "false",
					}
					if err := minIOAdminClient.SetTags(outBucket, tags); err != nil {
						return fmt.Errorf("Error tagging bucket: %v", err)
					}
				}

			}

		case types.OnedataName:
			// TODO
		}
	}
	// Delete isolated buckets
	if strings.ToUpper(service.IsolationLevel) == types.IsolationLevelUser && len(service.BucketList) != 0 {
		for _, bucket := range service.BucketList {

			// Disable input notifications for service bucket
			if err := disableInputNotifications(s3Client, service.GetMinIOWebhookARN(), bucket); err != nil {
				log.Printf("Error disabling MinIO input notifications for service \"%s\": %v\n", service.Name, err)
			}

			err := DeleteMinIOBuckets(s3Client, minIOAdminClient, utils.MinIOBucket{
				BucketName:   bucket,
				Visibility:   utils.PRIVATE,
				AllowedUsers: []string{},
				Owner:        service.Owner,
			})
			if err != nil {
				log.Printf("error while removing MinIO bucket %v", err)
			}
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
	if bucket.Owner != types.DefaultOwner {
		err := minIOAdminClient.RemoveResource(bucket.BucketName, policyName, isGroup)
		if err != nil {
			return fmt.Errorf("error removing resource")
		}

		if strings.ToLower(bucket.Visibility) == utils.RESTRICTED {
			err := minIOAdminClient.RemoveGroupPolicy(bucket.BucketName)
			if err != nil {
				return fmt.Errorf("error removing policy for group")
			}
		}
	}

	err := minIOAdminClient.DeleteBucket(s3Client, bucket.BucketName)
	if err != nil {
		return fmt.Errorf("error deleting bucket %s, %v", bucket.BucketName, err)
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

func sameStorage(firstStorage types.StorageIOConfig, secondStorage types.StorageIOConfig) bool {
	// Check if the bucket is in the mount path
	firstProvID, firstProvName := getProviderInfo(firstStorage.Provider)
	secondProvID, secondProvName := getProviderInfo(secondStorage.Provider)
	firstPath := strings.Trim(firstStorage.Path, " /")
	secondPath := strings.Trim(secondStorage.Path, " /")
	// Split buckets and folders from path
	splitPathBucket := strings.SplitN(firstPath, "/", 2)
	splitPathMount := strings.SplitN(secondPath, "/", 2)

	return firstProvID == secondProvID && firstProvName == secondProvName && splitPathBucket[0] == splitPathMount[0]
}
