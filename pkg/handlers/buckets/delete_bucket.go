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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
)

var ALL_USERS_GROUP = "all_users_group"
var allUserGroupNotExist = "unable to remove bucket from policy \"" + ALL_USERS_GROUP + "\", policy '" + ALL_USERS_GROUP + "' does not exist"
var deleteLogger = log.New(os.Stdout, "[DELETE-HANDLER] ", log.Flags())

// MakeDeleteHandler makes a handler for deleting services
func MakeDeleteHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		bucketName := c.Param("bucketName")
		var uid string
		var err error
		// Check owner
		authHeader := c.GetHeader("Authorization")
		if len(strings.Split(authHeader, "Bearer")) == 1 {
			uid = cfg.Name
			deleteLogger.Printf("Deleting bucket '%s' for user '%s'", bucketName, uid)
		} else {
			uid, err = auth.GetUIDFromContext(c)
			deleteLogger.Printf("Deleting bucket '%s' for user '%s'", bucketName, uid)

			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
				return
			}
		}
		// Check if other policies exist
		// Check if users in allowed_users have a MinIO associated user
		minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)
		err = DeleteBucket(minIOAdminClient, cfg.MinIOProvider.GetS3Client(), cfg, bucketName, uid)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintln(err))

		}

		c.Status(http.StatusNoContent)
	}
}

func allowToChange(cfg *types.Config, uid string, minIOAdminClient *utils.MinIOAdminClient, bucketName string) bool {
	if uid != cfg.Name {
		// Check if is the owner of bucket
		fmt.Println(uid)

		jsonUnmarshal, err := getPolicy(minIOAdminClient, uid)
		if err != nil {
			return false
		}
		for i := range jsonUnmarshal.Statement {
			for _, r := range jsonUnmarshal.Statement[i].Resource {
				if r == "arn:aws:s3:::"+bucketName+"/*" {
					deleteLogger.Printf("User '%s' allow to delete bucket '%s'", uid, bucketName)
					return true
				}
			}
		}
		return false
		// minIOAdminClient.adminClient.InfoCannedPolicyV2(context.TODO(), policyName)
	} else {
		isAdminUser = true
		//allowToDelete = true
		return true
	}
}

func DeleteBucket(minIOAdminClient *utils.MinIOAdminClient, s3Client *s3.S3, cfg *types.Config, bucketName string, uid string) error {
	//Minio provider (maybe need to change, i dont know)
	//minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)
	allowToDelete := allowToChange(cfg, uid, minIOAdminClient, bucketName)
	if allowToDelete {
		//minio := cfg.MinIOProvider
		//s3Client := minio.GetS3Client()

		// More users?
		group, errGroup := minIOAdminClient.GetGroup(bucketName)
		_, errPolicy := getPolicy(minIOAdminClient, bucketName)
		if errGroup == nil && errPolicy == nil {
			// Delete users in group
			err := minIOAdminClient.UpdateUsersInGroup(group.Members, bucketName, true)
			if err != nil {
				return err
			}
			// Delete group
			err = minIOAdminClient.UpdateUsersInGroup([]string{}, bucketName, true)
			if err != nil {
				//c.String(http.StatusInternalServerError, fmt.Sprintln(err))

				return err
			}

			// Remove policy
			err = minIOAdminClient.RemoveFromPolicy(bucketName, bucketName, true)
			if err != nil {
				//c.String(http.StatusInternalServerError, fmt.Sprintln(err))

				return err
			}
		}

		// Delete the bucket private to the user

		if !isAdminUser {

			// Remove policy
			err := minIOAdminClient.RemoveFromPolicy(bucketName, uid, false)
			if err != nil {
				return err
			}
		}

		// Delete Bucket
		_, err := s3Client.DeleteBucket(&s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
		if err != nil {
			return err
		}
	} else {
		return errors.New("user is not allow to delete this bucket")
		//return c.Status(http.StatusConflict)
	}
	return nil
}

func getPolicy(minIOAdminClient *utils.MinIOAdminClient, policyName string) (*utils.Policy, error) {
	policy, err := minIOAdminClient.GetPolicy(policyName)
	if err != nil {
		return nil, err
	}
	jsonUnmarshal := &utils.Policy{}
	jsonErr := json.Unmarshal(policy.Policy, jsonUnmarshal)
	if jsonErr != nil {
		return nil, jsonErr
	} else {
		return jsonUnmarshal, nil
	}
}
