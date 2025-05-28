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
	"github.com/grycap/oscar/v3/pkg/handlers"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
)

var ALL_USERS_GROUP = "all_users_group"
var deleteLogger = log.New(os.Stdout, "[DELETE-HANDLER] ", log.Flags())

// MakeDeleteHandler makes a handler for deleting services
func MakeDeleteHandler(cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var uid string
		bucketName := c.Param("bucket")
		if bucketName == "" {
			c.String(http.StatusBadRequest, fmt.Sprintf("Received empty bucket name"))
			return

		}
		// Check owner
		authHeader := c.GetHeader("Authorization")
		if len(strings.Split(authHeader, "Bearer")) == 1 {
			uid = cfg.Name
			deleteLogger.Printf("Deleting bucket '%s' for user '%s'", bucketName, uid)
		} else {
			uid, err := auth.GetUIDFromContext(c)
			deleteLogger.Printf("Deleting bucket '%s' for user '%s'", bucketName, uid)

			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln("error getting user from request:", err))
				return
			}
		}
		s3Client := cfg.MinIOProvider.GetS3Client()
		minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)
		v := minIOAdminClient.GetCurrentResourceVisibility(utils.MinIOBucket{BucketPath: bucketName, Owner: uid})
		if v == utils.PUBLIC || minIOAdminClient.ResourceInPolicy(uid, bucketName) {
			err := handlers.DeleteMinIOBuckets(s3Client, minIOAdminClient, utils.MinIOBucket{
				BucketPath: bucketName,
				Visibility: v,
				Owner:      uid,
			})
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
				return
			}
		} else {
			c.String(http.StatusUnauthorized, fmt.Sprintf("User '%s' is not authorised", uid))
			return
		}

		c.Status(http.StatusNoContent)
	}
}
