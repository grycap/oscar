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

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v2/pkg/types"
	"github.com/grycap/oscar/v2/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
)

// MakeUpdateHandler makes a handler for updating services
func MakeUpdateHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		var newService types.Service
		if err := c.ShouldBindJSON(&newService); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The service specification is not valid: %v", err))
			return
		}

		// Check service values and set defaults
		if err := checkValues(&newService, cfg); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The service specification is not valid: %v", err))
			return
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

		// Update the service
		if err := back.UpdateService(newService); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error updating the service: %v", err))
			return
		}

		// Register minio webhook and restart the server
		if err := registerMinIOWebhook(newService.Name, newService.Token, newService.StorageProviders.MinIO[types.DefaultProvider], cfg); err != nil {
			back.UpdateService(*oldService)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		// Update buckets
		if err := updateBuckets(&newService, oldService, cfg); err != nil {
			if err == errNoMinIOInput {
				c.String(http.StatusBadRequest, err.Error())
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			// If updateBuckets fails restore the oldService
			back.UpdateService(*oldService)
			return
		}

		// Add Yunikorn queue if enabled
		if cfg.YunikornEnable {
			if err := utils.AddYunikornQueue(cfg, back.GetKubeClientset(), &newService); err != nil {
				log.Println(err.Error())
			}
		}

		c.Status(http.StatusNoContent)
	}
}

func updateBuckets(newService, oldService *types.Service, cfg *types.Config) error {
	// Disable notifications from oldService.Input
	if err := disableInputNotifications(oldService.GetMinIOWebhookARN(), oldService.Input, oldService.StorageProviders.MinIO[types.DefaultProvider]); err != nil {
		return fmt.Errorf("error disabling MinIO input notifications: %v", err)
	}

	// Create the input and output buckets/folders from newService
	if err := createBuckets(newService, cfg); err != nil {
		return err
	}

	return nil
}
