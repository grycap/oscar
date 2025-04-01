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
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
)

const (
	defaultMemory   = "256Mi"
	defaultCPU      = "0.2"
	defaultLogLevel = "INFO"
	createPath      = "/system/services"
)

//var errInput = errors.New("unrecognized input (valid inputs are MinIO and dCache)")
//var overlappingError = "An object key name filtering rule defined with overlapping prefixes"

// Custom logger
var createLogger = log.New(os.Stdout, "[CREATE-HANDLER] ", log.Flags())
var isAdminUser = false

// MakeCreateHandler makes a handler for creating services
func MakeCreateHandler(cfg *types.Config, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		bucketName := c.Param("bucketName")
		var allowedUsers []string
		var err error
		if err := c.ShouldBindJSON(&allowedUsers); err != nil {
			if allowedUsers != nil {
				c.String(http.StatusBadRequest, fmt.Sprintf("The Bucket specification is not valid: %v", err))
				return
			}
		}
		isAdminUser = false
		uid := cfg.Name

		authHeader := c.GetHeader("Authorization")
		if len(strings.Split(authHeader, "Bearer")) == 1 {
			isAdminUser = true
		}

		if !isAdminUser {
			uid, err = auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintln(err))

			}
		}
		minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)
		//minio := cfg.MinIOProvider
		err = CreateBucket(minIOAdminClient, cfg.MinIOProvider.GetS3Client(), bucketName, uid, allowedUsers)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintln(err))
		}
		// Check if users in allowed_users have a MinIO associated user

		createLogger.Printf("%s | %v | %s | %s | %s", "POST", 200, createPath, uid, bucketName)
		c.Status(http.StatusCreated)
	}

}

func CreateBucket(minIOAdminClient *utils.MinIOAdminClient, s3Client *s3.S3, bucketName string, uid string, allowedUsers []string) error {

	//minIOAdminClient, _ := utils.MakeMinIOAdminClient(cfg)
	createLogger.Printf("Creating bucket '%s' for user '%s'", bucketName, uid)

	//Minio provider (maybe need to change, i dont know)
	//minio := cfg.MinIOProvider
	//s3Client := minio.GetS3Client()
	// Create Bucket
	splitPath := strings.SplitN(bucketName, "/", 2)
	_, err := s3Client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})

	var folderKey string
	if len(splitPath) == 2 {
		// Add "/" to the end of the key in order to create a folder
		folderKey = fmt.Sprintf("%s/", splitPath[1])
		_, err := s3Client.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(splitPath[0]),
			Key:    aws.String(folderKey),
		})
		if err != nil {
			return fmt.Errorf("error creating folder \"%s\" in bucket \"%s\": %v", folderKey, splitPath[0], err)
		}
	}

	if err != nil {
		return err
	}

	// Make the bucket private to the user
	if !isAdminUser {
		err = minIOAdminClient.CreateAddPolicy(bucketName, uid, false)
		createLogger.Printf("User '%s' have added the bucket '%s' to his policy", uid, bucketName)
		if err != nil {
			_, _ = s3Client.DeleteBucket(&s3.DeleteBucketInput{
				Bucket: aws.String(bucketName),
			})
		}
	}

	// More users?
	if allowedUsers != nil {
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

	}
	return nil
}
