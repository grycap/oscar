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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
)

var updateLogger = log.New(os.Stdout, "[CREATE-HANDLER] ", log.Flags())

// MakeDeleteHandler makes a handler for deleting services
func MakeUpdateHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		bucketName := c.Param("bucketName")
		var uid string
		var err error
		var allowedUsers []string
		// Check owner
		if err := c.ShouldBindJSON(&allowedUsers); err != nil {
			if allowedUsers != nil {
				c.String(http.StatusBadRequest, fmt.Sprintf("The Bucket specification is not valid: %v", err))
				return
			}
		}
		authHeader := c.GetHeader("Authorization")
		if len(strings.Split(authHeader, "Bearer")) == 1 {
			uid = cfg.Name
			updateLogger.Printf("Updating bucket '%s' for user '%s'", bucketName, uid)
		} else {
			uid, err = auth.GetUIDFromContext(c)
			updateLogger.Printf("Updating bucket '%s' for user '%s'", bucketName, uid)

			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))
				return
			}
		}
		// Check if other policies exist
		// Check if users in allowed_users have a MinIO associated user
		minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)

		err = UpdateBucket(minIOAdminClient, cfg.MinIOProvider.GetS3Client(), cfg, bucketName, uid, allowedUsers)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintln(err))

		}

		c.Status(http.StatusNoContent)
	}
}
func UpdateBucket(minIOAdminClient *utils.MinIOAdminClient, s3Client *s3.S3, cfg *types.Config, bucketName string, uid string, allowedUsers []string) error {
	// existe el bucket?
	// si no existe -> error
	//minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)
	//minio := cfg.MinIOProvider
	//s3Client := minio.GetS3Client()
	_, err := s3Client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			// Check if the error is caused because the bucket already exists
			if aerr.Code() == s3.ErrCodeBucketAlreadyExists || aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
				log.Printf("The bucket \"%s\" already exists\n", bucketName)
			}
		}
	} else if err == nil {
		return err
	}
	// si existe
	/*if uid == cfg.Name {

	}*/
	// es privado for user?
	allowToChange := allowToChange(cfg, uid, minIOAdminClient, bucketName)

	if allowToChange && allowedUsers != nil {
		err := minIOAdminClient.UpdateUsersInGroup(allowedUsers, bucketName, false)
		if err != nil {
			return err
		}
		createLogger.Printf("Group of users '%s' have added the policy '%s'", allowedUsers, bucketName)

		err = minIOAdminClient.CreateAddPolicy(bucketName, bucketName, true)
		if err != nil {
			return err
		}
		createLogger.Printf("Policy '%s' has added the bucket '%s' to his policy", bucketName, bucketName)
	} else if allowToChange && allowedUsers == nil {
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
	}
	// es privado for other users
	// si es existe grupo y allowed user != nil -> update
	// si es existe grupo y allowed user == nil -> delete
	// si no existe grupo y allowed user == nil -> nada
	// si no existe grupo y allowed user != nil -> crear
	return nil
}
