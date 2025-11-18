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

package buckets

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
)

var updateLogger = log.New(os.Stdout, "[CREATE-HANDLER] ", log.Flags())

// MakeUpdateHandler godoc
// @Summary Update bucket
// @Description Change bucket visibility or allowed users.
// @Tags buckets
// @Accept json
// @Param bucket body utils.MinIOBucket true "Bucket definition"
// @Success 204 {string} string "No Content"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/buckets [put]
func MakeUpdateHandler(cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var uid string
		var err error
		var bucket utils.MinIOBucket
		if err := c.ShouldBindJSON(&bucket); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The Bucket specification is not valid: %v", err))
			return
		}

		authHeader := c.GetHeader("Authorization")
		if len(strings.Split(authHeader, "Bearer")) == 1 {
			uid = cfg.Name
		} else {
			uid, err = auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln("error getting user from request:", err))
				return
			}
			if uid == "" {
				c.String(http.StatusInternalServerError, fmt.Sprintln("Couldn't find user identification"))
				return
			}
		}

		minIOAdminClient, err := utils.MakeMinIOAdminClient(cfg)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintln("error creating MinIO admin client:", err))
			return
		}

		metadata, err := minIOAdminClient.GetTaggedMetadata(bucket.BucketName)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintln("Missing bucket metadata: "))
			return
		}
		isService, _ := strconv.ParseBool(metadata["service"])
		if isService {
			c.String(http.StatusForbidden, fmt.Sprintln("Forbidden action: A bucket from a service can't be modified"))
			return
		}

		bucket.Owner = uid
		var oldVis string
		if oldVis = minIOAdminClient.GetCurrentResourceVisibility(bucket); oldVis != "" {
			if oldVis == utils.PUBLIC || minIOAdminClient.ResourceInPolicy(uid, bucket.BucketName) {
				if oldVis != bucket.Visibility {
					// Remove old policies
					err := minIOAdminClient.UnsetPolicies(utils.MinIOBucket{
						BucketName: bucket.BucketName,
						Visibility: oldVis,
						Owner:      uid,
					})
					if err != nil {
						c.String(http.StatusInternalServerError, fmt.Sprintln("error updating bucket:", err))
						return
					}

					// Set new policies
					err = minIOAdminClient.SetPolicies(bucket)
					if err != nil {
						c.String(http.StatusInternalServerError, fmt.Sprintln("error updating bucket:", err))
						return
					}

				} else {
					if oldVis == RESTRICTED {
						err = minIOAdminClient.UpdateServiceGroup(bucket.BucketName, bucket.AllowedUsers)
						if err != nil {
							c.String(http.StatusInternalServerError, fmt.Sprintln("error updating bucket:", err))
							return
						}
					}
				}
			}
		}

		c.Status(http.StatusNoContent)
	}
}
