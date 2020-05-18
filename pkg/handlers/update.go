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
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/pkg/types"
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

		// Update buckets
		if err := updateBuckets(&newService, oldService); err != nil {
			if err == errNoMinIOInput {
				c.String(http.StatusBadRequest, err.Error())
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			// If updateBuckets fails restore the oldService
			back.UpdateService(*oldService)
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func updateBuckets(newService, oldService *types.Service) error {
	// Disable notifications from oldService.Input
	if err := disableInputNotifications(oldService.GetMinIOWebhookARN(), oldService.Input, oldService.StorageProviders.MinIO); err != nil {
		return fmt.Errorf("Error disabling MinIO input notifications: %v", err)
	}

	// Create the input and output buckets/folders from newService
	if err := createBuckets(newService); err != nil {
		return err
	}

	return nil
}
