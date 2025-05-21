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
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
)

const (
	createPath = "/system/services"
	PRIVATE    = "private"
	PUBLIC     = "public"
	RESTRICTED = "restricted"
)

// Custom logger
var createLogger = log.New(os.Stdout, "[CREATE-BUCKETS-HANDLER] ", log.Flags())
var isAdminUser = false

// MakeCreateHandler makes a handler for creating services
func MakeCreateHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		var uid string
		var bucket utils.MinIOBucket
		if err := c.ShouldBindJSON(&bucket); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("The Bucket specification is not valid: %v", err))
			return

		}
		isAdminUser = false
		uid = cfg.Name

		authHeader := c.GetHeader("Authorization")
		if len(strings.Split(authHeader, "Bearer")) == 1 {
			isAdminUser = true
		}

		if !isAdminUser {
			var err error
			uid, err = auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))

			}
		}
		bucket.Owner = uid
		// Use admin MinIO client for the bucket creation
		s3Client := cfg.MinIOProvider.GetS3Client()
		minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)

		path := strings.Trim(bucket.BucketPath, " /")
		// Split buckets and folders from path
		splitPath := strings.SplitN(path, "/", 2)
		if err := minIOAdminClient.CreateS3Path(s3Client, splitPath, false); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("error creating a bucket with the name '%s' %v", splitPath[0], err))
			return
		}
		// If not specified default visibility is PRIVATE
		visibility := strings.ToLower(bucket.Visibility)
		if bucket.Visibility == "" {
			visibility = utils.PRIVATE
		}
		if visibility == RESTRICTED || visibility == PRIVATE {
			// Both types of visibility require config of the user policy
			if err := minIOAdminClient.CreateAddPolicy(splitPath[0], bucket.Owner, utils.ALL_ACTIONS, false); err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln("error creating policy for user: %v", err))
				return
			}
			if visibility == RESTRICTED {
				if err := minIOAdminClient.CreateAddGroup(splitPath[0], bucket.AllowedUsers, false); err != nil {
					c.String(http.StatusInternalServerError, fmt.Sprintln("error creating group: %v", err))
					return
				}
				if err := minIOAdminClient.CreateAddPolicy(splitPath[0], splitPath[0], utils.RESTRICTED_ACTIONS, true); err != nil {
					c.String(http.StatusInternalServerError, fmt.Sprintln("error creating group: %v", err))
					return
				}
			}
		} else {
			// Config public visibility
			if err := minIOAdminClient.CreateAddPolicy(splitPath[0], ALL_USERS_GROUP, utils.ALL_ACTIONS, true); err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln("error creating policy for user: %v", err))
				return
			}
		}

		createLogger.Printf("%s | %v | %s | %s | %s", "POST", 200, createPath, uid, bucket.BucketPath)
		c.Status(http.StatusCreated)
	}

}
