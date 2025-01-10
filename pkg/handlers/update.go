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
	"strings"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	"k8s.io/apimachinery/pkg/api/errors"
)

// Custom logger
var updateLogger = log.New(os.Stdout, "[CREATE-HANDLER] ", log.Flags())

// MakeUpdateHandler makes a handler for updating services
func MakeUpdateHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		var provName, provID string
		var newService types.Service
		if err := c.ShouldBindJSON(&newService); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The service specification is not valid: %v", err))
			return
		}

		// Check service values and set defaults
		checkValues(&newService, cfg)
		authHeader := c.GetHeader("Authorization")
		if len(strings.Split(authHeader, "Bearer")) == 1 {
			isAdminUser = true
			createLogger.Printf("[*] Updating service as admin user")
		}
		// Read the current service
		oldService, err := back.ReadService(newService.Name)

		if err != nil {
			// Check if error is caused because the service is not found
			if errors.IsNotFound(err) || errors.IsGone(err) {
				c.Status(http.StatusNotFound)
			} else {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error updating the service: %v", err))
			}
			return
		}

		if !isAdminUser {
			uid, err := auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln("Couldn't get UID from context"))
			}

			if oldService.Owner != uid {
				c.String(http.StatusForbidden, "User %s doesn't have permision to modify this service", uid)
				return
			}

			// Set the owner on the new service definition
			newService.Owner = oldService.Owner

			// If the service has changed VO check permisions again
			if newService.VO != "" && newService.VO != oldService.VO {
				for _, vo := range cfg.OIDCGroups {
					if vo == newService.VO {
						authHeader := c.GetHeader("Authorization")
						err := checkIdentity(&newService, cfg, authHeader)
						if err != nil {
							c.String(http.StatusBadRequest, fmt.Sprintln(err))
						}
						break
					}
				}
			}
		}
		minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)

		for _, in := range oldService.Input {

			provID, provName = getProviderInfo(in.Provider)

			if provName == types.MinIOName {
				s3Client := oldService.StorageProviders.MinIO[provID].GetS3Client()

				// Get bucket name
				path := strings.Trim(in.Path, " /")
				// Split buckets and folders from path
				splitPath := strings.SplitN(path, "/", 2)
				// If isolation level was USER delete all private buckets
				if oldService.IsolationLevel == "USER" {
					err = deletePrivateBuckets(oldService, minIOAdminClient, s3Client)
					if err != nil {
						return
					}
				}
				if newService.IsolationLevel == "USER" {
					var newBucketList []string
					var userBucket string
					for _, user := range newService.AllowedUsers {
						userBucket = splitPath[0] + "-" + user[:10]
						newBucketList = append(newBucketList, userBucket)
					}

					newService.BucketList = newBucketList
				}

				// Update the group with allowe users, it empthy and add them again
				err = updateGroup(splitPath[0], oldService, &newService, minIOAdminClient, s3Client)
				if err != nil {
					return
				}

				err = disableInputNotifications(s3Client, oldService.GetMinIOWebhookARN(), splitPath[0])
				if err != nil {
					return
				}
				// Register minio webhook and restart the server
				if err := registerMinIOWebhook(newService.Name, newService.Token, newService.StorageProviders.MinIO[types.DefaultProvider], cfg); err != nil {
					uerr := back.UpdateService(*oldService)
					if uerr != nil {
						log.Println(uerr.Error())
					}
					c.String(http.StatusInternalServerError, err.Error())
					return
				}

				// Update the service
				if err := back.UpdateService(newService); err != nil {
					c.String(http.StatusInternalServerError, fmt.Sprintf("Error updating the service: %v", err))
					return
				}

				// Update buckets
				if err := updateBuckets(&newService, &newService, minIOAdminClient, cfg); err != nil {
					if err == errInput {
						c.String(http.StatusBadRequest, err.Error())
					} else {
						c.String(http.StatusInternalServerError, err.Error())
					}
					// If updateBuckets fails restore the oldService
					uerr := back.UpdateService(*oldService)
					if uerr != nil {
						log.Println(uerr.Error())
					}
					return
				}

			}

			// Add Yunikorn queue if enabled
			if cfg.YunikornEnable {
				if err := utils.AddYunikornQueue(cfg, back.GetKubeClientset(), &newService); err != nil {
					log.Println(err.Error())
				}
			}

			c.Status(http.StatusNoContent)
		}
		if len(oldService.Input) == 0 {
			if err := back.UpdateService(newService); err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error updating the service: %v", err))
				return
			}
		}

	}
}

func updateGroup(group string, oldService *types.Service, newService *types.Service, minIOAdminClient *utils.MinIOAdminClient, s3Client *s3.S3) error {
	//delete users in group
	err := minIOAdminClient.UpdateUsersInGroup(oldService.AllowedUsers, group, true)
	if err != nil {
		return err
	}
	//add the new ones
	err = minIOAdminClient.UpdateUsersInGroup(newService.AllowedUsers, group, false)
	if err != nil {
		return err
	}
	return nil
}

func updateBuckets(newService, oldService *types.Service, minIOAdminClient *utils.MinIOAdminClient, cfg *types.Config) error {
	// Disable notifications from oldService.Input

	// TODO diable all old service notifications if needed
	//if err := disableInputNotifications(oldService.GetMinIOWebhookARN(), oldService.Input); err != nil {
	//	return fmt.Errorf("error disabling MinIO input notifications: %v", err)
	//}

	// Create the input and output buckets/folders from newService
	return createBuckets(newService, cfg, minIOAdminClient, true)
}
