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
	createPath = "/system/buckets"
	PRIVATE    = "private"
	PUBLIC     = "public"
	RESTRICTED = "restricted"
)

// Custom logger
var createLogger = log.New(os.Stdout, "[CREATE-BUCKETS-HANDLER] ", log.Flags())
var isAdminUser = false

// MakeCreateHandler makes a handler for creating services
func MakeCreateHandler(cfg *types.Config) gin.HandlerFunc {
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
				return
			}
		}

		if uid == "" {
			c.String(http.StatusInternalServerError, fmt.Sprintln("Couldn't find user identification"))
			return
		}

		bucket.Owner = uid
		// Use admin MinIO client for the bucket creation
		s3Client := cfg.MinIOProvider.GetS3Client()
		minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)

		path := strings.Trim(bucket.BucketPath, " /")
		// Split buckets and folders from path
		splitPath := strings.SplitN(path, "/", 2)
		if err := minIOAdminClient.CreateS3Path(s3Client, splitPath, false); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("Error creating bucket with name '%s': %v", splitPath[0], err))
			return
		}
		// Bucket metadata for filtering
		tags := map[string]string{
			"owner":   uid,
			"service": "false",
		}
		if err := minIOAdminClient.SetTags(splitPath[0], tags); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("Error tagging bucket: %v", err))
		}
		// If not specified default visibility is PRIVATE
		if strings.ToLower(bucket.Visibility) == "" {
			bucket.Visibility = utils.PRIVATE
		}

		err := minIOAdminClient.SetPolicies(bucket)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating policies for bucket: %v", err))
		}

		createLogger.Printf("%s | %v | %s | %s | %s", "POST", 200, createPath, uid, bucket.BucketPath)
		c.Status(http.StatusCreated)
	}

}
