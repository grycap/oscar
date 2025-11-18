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

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/handlers"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
)

var ALL_USERS_GROUP = "all_users_group"
var deleteLogger = log.New(os.Stdout, "[DELETE-HANDLER] ", log.Flags())

// MakeDeleteHandler godoc
// @Summary Delete bucket
// @Description Delete a MinIO bucket owned by the authenticated user.
// @Tags buckets
// @Produce json
// @Param bucket path string true "Bucket name"
// @Success 204 {string} string "No Content"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/buckets/{bucket} [delete]
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
			uid = types.DefaultOwner
			deleteLogger.Printf("Deleting bucket '%s' for user '%s'", bucketName, uid)
		} else {
			var err error
			uid, err = auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln("error getting user from request:", err))
				return
			}
			deleteLogger.Printf("Deleting bucket '%s' for user '%s'", bucketName, uid)
		}
		s3Client := cfg.MinIOProvider.GetS3Client()
		// Check that the bucket exists
		bucketInfo, err := s3Client.ListBuckets(&s3.ListBucketsInput{})
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintln(err))
		}

		var foundInMinIO bool
		for _, b := range bucketInfo.Buckets {
			if *b.Name == bucketName {
				foundInMinIO = true
				break
			}
		}
		if !foundInMinIO {
			c.String(http.StatusNotFound, fmt.Sprintf("The bucket '%s' does not exist", bucketName))
			return
		}
		// If bucket exit
		minIOAdminClient, err := utils.MakeMinIOAdminClient(cfg)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating MinIO admin client: %v", err))
			return
		}
		v := minIOAdminClient.GetCurrentResourceVisibility(utils.MinIOBucket{BucketName: bucketName, Owner: uid})
		if (uid == types.DefaultOwner) || (v == utils.PUBLIC || minIOAdminClient.ResourceInPolicy(uid, bucketName)) {
			err := handlers.DeleteMinIOBuckets(s3Client, minIOAdminClient, utils.MinIOBucket{
				BucketName: bucketName,
				Visibility: v,
				Owner:      uid,
			})
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
				return
			}
		} else {
			c.String(http.StatusForbidden, fmt.Sprintf("User '%s' is not authorised", uid))
			return
		}

		c.Status(http.StatusNoContent)
	}
}
