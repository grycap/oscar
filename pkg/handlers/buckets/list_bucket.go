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
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
)

// MakeListHandler makes a handler for listing services
func MakeListHandler(cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {

		authHeader := c.GetHeader("Authorization")
		isAdminUser = false
		var uid string
		var err error
		if len(strings.Split(authHeader, "Bearer")) == 1 {
			isAdminUser = true
			output, err := listUserBuckets(cfg.MinIOProvider.GetS3Client())
			if err != nil {
				c.JSON(http.StatusInternalServerError, err)
			}
			c.JSON(http.StatusOK, output)

		} else {
			uid, err = auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))

			}
			mc, err := auth.GetMultitenancyConfigFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
			}

			ak, sk, err := mc.GetUserCredentials(uid)
			if err != nil {
				c.String(http.StatusInternalServerError, "Error getting credentials for MinIO user: ", uid)
			}

			userMinIOProvider := &types.MinIOProvider{
				Endpoint:  cfg.MinIOProvider.Endpoint,
				Verify:    cfg.MinIOProvider.Verify,
				AccessKey: ak,
				SecretKey: sk,
				Region:    cfg.MinIOProvider.Region,
			}

			bucketsList, err := listUserBuckets(userMinIOProvider.GetS3Client())
			if err != nil {
				c.String(http.StatusInternalServerError, "Error reading buckets from user: ", uid)
			}
			var bucketsInfo []utils.MinIOBucket
			minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)

			for _, b := range bucketsList.Buckets {
				var allowedUsers []string
				var bowner string
				path := *b.Name
				bucketVisibility := minIOAdminClient.GetCurrentResourceVisibility(utils.MinIOBucket{BucketPath: *b.Name, Owner: uid})

				if bucketVisibility == utils.PRIVATE {
					bowner = uid
				} else {
					bowner, err = minIOAdminClient.GetTaggedOwner(path)
					if err != nil {
						bowner = ""
						fmt.Printf("Couldn't get bucket owner info: %v", err)
					}
				}
				if bucketVisibility == utils.RESTRICTED {
					members, err := minIOAdminClient.GetBucketMembers(path)
					if err != nil {
						fmt.Printf("Couldn't get bucket owner info: %v", err)
					} else {
						allowedUsers = append(allowedUsers, members...)
					}
				}

				bucketsInfo = append(bucketsInfo, utils.MinIOBucket{
					BucketPath:   path,
					Visibility:   bucketVisibility,
					Owner:        bowner,
					AllowedUsers: allowedUsers,
				})
			}
			c.JSON(http.StatusOK, bucketsInfo)

		}
	}
}

func listUserBuckets(s3Client *s3.S3) (*s3.ListBucketsOutput, error) {
	output, err := s3Client.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}
	return output, nil
}
