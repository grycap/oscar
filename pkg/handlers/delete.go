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

// MakeDeleteHandler makes a handler for deleting services
func MakeDeleteHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First get the Service
		service, _ := back.ReadService(c.Param("serviceName"))
		authHeader := c.GetHeader("Authorization")

		var isAllowed bool
		if len(strings.Split(authHeader, "Bearer")) > 1 {
			uid, err := auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
			}

			for _, id := range service.AllowedUsers {
				if uid == id {
					isAllowed = true
					break
				}
			}

			if !isAllowed {
				c.String(http.StatusForbidden, "User %s doesn't have permision to get this service", uid)
				return
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

		if isAllowed {
			// Delete the group and policy
			for _, in := range service.Input {
				path := strings.Trim(in.Path, " /")
				// Split buckets and folders from path
				bucket := strings.SplitN(path, "/", 2)
				minIOAdminClient.DeleteServiceGroup(bucket[0])
			}

		}

		// Disable input notifications
		if err := disableInputNotifications(service.GetMinIOWebhookARN(), service.Input, service.StorageProviders.MinIO[types.DefaultProvider]); err != nil {
			log.Printf("Error disabling MinIO input notifications for service \"%s\": %v\n", service.Name, err)
		}

		// Remove the service's webhook in MinIO config and restart the server
		if err := removeMinIOWebhook(service.Name, minIOAdminClient); err != nil {
			log.Printf("Error removing MinIO webhook for service \"%s\": %v\n", service.Name, err)
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

func disableInputNotifications(arnStr string, input []types.StorageIOConfig, minIO *types.MinIOProvider) error {
	parsedARN, _ := arn.Parse(arnStr)

	// Create S3 client for MinIO
	minIOClient := minIO.GetS3Client()

	for _, in := range input {
		path := strings.Trim(in.Path, " /")
		// Split buckets and folders from path
		splitPath := strings.SplitN(path, "/", 2)

		updatedQueueConfigurations := []*s3.QueueConfiguration{}
		// Get bucket notification
		nCfg, err := minIOClient.GetBucketNotificationConfiguration(&s3.GetBucketNotificationConfigurationRequest{Bucket: aws.String(splitPath[0])})
		if err != nil {
			return fmt.Errorf("error getting bucket \"%s\" notifications: %v", splitPath[0], err)
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
			Bucket:                    aws.String(splitPath[0]),
			NotificationConfiguration: nCfg,
		}
		_, err = minIOClient.PutBucketNotificationConfiguration(pbncInput)
		if err != nil {
			return fmt.Errorf("error disabling bucket notification: %v", err)
		}
	}

	return nil
}
