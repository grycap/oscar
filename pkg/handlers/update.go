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
		var provName string
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
						err := checkIdentity(&newService, authHeader)
						if err != nil {
							c.String(http.StatusBadRequest, fmt.Sprintln(err))
						}
						break
					}
				}
			}
		}
		minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)

		// If isolation level was USER delete all private buckets
		if strings.ToUpper(oldService.IsolationLevel) == "USER" && strings.ToUpper(newService.IsolationLevel) == "USER" {
			// TODO add/remove users buckets
		}

		// Use create buckets function to create new inputs/outputs if needed
		var newServiceBuckets []utils.MinIOBucket
		if newServiceBuckets, err = createBuckets(&newService, cfg, minIOAdminClient, true); err != nil {
			if err == errInput {
				c.String(http.StatusBadRequest, err.Error())
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			// If createBuckets fails restore the oldService
			uerr := back.UpdateService(*oldService)
			if uerr != nil {
				log.Println(uerr.Error())
			}
			return
		}

		// Get old service buckets and compare to the new ones
		var oldServiceBuckets = make(map[string]bool)
		// Set true all MinIO buckets of the previous definition
		for _, in := range oldService.Input {

			_, provName = getProviderInfo(in.Provider)

			if provName == types.MinIOName {
				path := strings.Trim(in.Path, " /")
				// Split buckets and folders from path
				splitPath := strings.SplitN(path, "/", 2)
				oldServiceBuckets[splitPath[0]] = true
			}
		}
		for _, in := range oldService.Output {

			_, provName = getProviderInfo(in.Provider)

			if provName == types.MinIOName {
				path := strings.Trim(in.Path, " /")
				// Split buckets and folders from path
				splitPath := strings.SplitN(path, "/", 2)
				oldServiceBuckets[splitPath[0]] = true
			}
		}
		if len(newServiceBuckets) > 0 {
			for _, b := range newServiceBuckets {
				if oldServiceBuckets[b.BucketPath] {
					// If the visibility of the bucket has changed remove old policies and config new ones
					if oldService.Visibility != newService.Visibility {
						minIOAdminClient.UnsetPolicies(b)
						// If not specified default visibility is PRIVATE
						if strings.ToLower(newService.Visibility) == "" {
							b.Visibility = utils.PRIVATE
						}
						err := minIOAdminClient.SetPolicies(b)
						if err != nil {
							c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating the service: %v", err))
						}
					} else {
						if newService.Visibility == utils.RESTRICTED {
							err := minIOAdminClient.UpdateServiceGroup(b.BucketPath, newService.AllowedUsers)
							if err != nil {
								c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating the service: %v", err))
							}
						}
					}
					// Set false to know which buckets need to be private
					oldServiceBuckets[b.BucketPath] = false
				} else {
					// If the bucket didn't exist on the old service assume its created an set policies & webhooks
					err := minIOAdminClient.SetPolicies(b)
					if err != nil {
						c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating the service: %v", err))
					}
					// Register minio webhook and restart the server
					if err = registerMinIOWebhook(newService.Name, newService.Token, newService.StorageProviders.MinIO[types.DefaultProvider], cfg); err != nil {
						uerr := back.UpdateService(*oldService)
						if uerr != nil {
							log.Println(uerr.Error())
						}
						c.String(http.StatusInternalServerError, err.Error())
						return
					}
				}
			}
		}

		for key, value := range oldServiceBuckets {
			// If the bucket was not used in the new service definition set it to private
			if value {
				err := minIOAdminClient.SetPolicies(utils.MinIOBucket{BucketPath: key, Visibility: utils.PRIVATE})
				if err != nil {
					c.String(http.StatusInternalServerError, "error setting new policies: %v", err)
				}
			}
		}

		// Update service secret data or create it
		if len(newService.Environment.Secrets) > 0 {
			if utils.SecretExists(newService.Name, cfg.ServicesNamespace, back.GetKubeClientset()) {
				secretsErr := utils.UpdateSecretData(newService.Name, cfg.ServicesNamespace, newService.Environment.Secrets, back.GetKubeClientset())
				if secretsErr != nil {
					c.String(http.StatusInternalServerError, "error updating asociated secret: %v", secretsErr)
				}
			} else {
				secretsErr := utils.CreateSecret(newService.Name, cfg.ServicesNamespace, newService.Environment.Secrets, back.GetKubeClientset())
				if secretsErr != nil {
					c.String(http.StatusInternalServerError, "error adding asociated secret: %v", secretsErr)
				}
			}
			// Empty the secrets content from the Configmap
			for secretKey := range newService.Environment.Secrets {
				newService.Environment.Secrets[secretKey] = ""
			}
		}

		// Update the service
		if err := back.UpdateService(newService); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error updating the service: %v", err))
			return
		}

		c.Status(http.StatusNoContent)
	}
}
