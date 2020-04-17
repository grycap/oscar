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
	"github.com/grycap/oscar/pkg/types"
	"github.com/grycap/oscar/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
)

// MakeDeleteHandler makes a handler to delete a service
func MakeDeleteHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First get the Service
		svc, _ := back.ReadService(c.Param("serviceName"))

		if err := back.DeleteService(c.Param("serviceName")); err != nil {
			// Check if error is caused because the service is not found
			if errors.IsNotFound(err) || errors.IsGone(err) {
				c.Status(http.StatusNotFound)
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			return
		}

		// TODO: remove bucket notifications

		// Remove the service's webhook in MinIO config and restart the server
		if err := removeMinIOWebhook(svc.Name, svc.StorageProviders.MinIO, cfg); err != nil {
			log.Printf("Error removing MinIO webhook for service \"%s\": %v", svc.Name, err)
		}

		c.Status(http.StatusNoContent)
	}
}

func removeMinIOWebhook(name string, minIO *types.MinIOProvider, cfg *types.Config) error {
	minIOAdminClient, err := utils.MakeMinIOAdminClient(minIO, cfg)
	if err != nil {
		return fmt.Errorf("The provided MinIO configuration is not valid: %v", err)
	}

	if err := minIOAdminClient.RemoveWebhook(name); err != nil {
		return fmt.Errorf("Error removing the service's webhook: %v", err)
	}

	if err := minIOAdminClient.RestartServer(); err != nil {
		return err
	}

	return nil
}

// TODO
func deleteNotifications(input []types.StorageIOConfig, cfg *types.Config) error {
	return nil
}
