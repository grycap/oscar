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
	objectstorage "github.com/grycap/oscar/v4/pkg/backends/object_storage"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
	"github.com/minio/madmin-go"
)

// MakeGetHandler godoc
// @Summary Get bucket details
// @Description Retrieve metadata and objects for a specific bucket.
// @Tags buckets
// @Produce json
// @Param bucket path string true "Bucket name"
// @Param page query string false "Continuation token"
// @Success 200 {object} buckets.BucketListResponse
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/buckets/{bucket} [get]
func MakeGetHandler(cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		bucketName := strings.TrimSpace(c.Param("bucket"))
		if bucketName == "" {
			c.String(http.StatusBadRequest, "Bucket parameter cannot be empty")
			return
		}
		//ctx := c.Request.Context()

		authHeader := c.GetHeader("Authorization")
		isAdmin := len(strings.Split(authHeader, "Bearer")) == 1
		objectStorageIAM, err := objectstorage.MakeObjectStorageIAM(cfg)

		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating MinIO admin client: %v", err))
			return
		}

		requester := cfg.Username
		if !isAdmin {
			requester, err = auth.GetUIDFromContext(c)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error determining requester identity: %v", err))
				return
			}
			if requester == "" {
				c.String(http.StatusInternalServerError, "Couldn't determine requester identity")
				return
			}
		}

		metadata, metaErr := objectStorageIAM.GetClient(c.Request.Context()).GetTaggedMetadata(bucketName)
		if metaErr != nil {
			metadata = map[string]string{}
		}

		ownerCandidate := metadata["owner"]
		if ownerCandidate == "" {
			ownerCandidate = cfg.Username
		}

		visibility := objectStorageIAM.GetClient(c.Request.Context()).GetCurrentResourceVisibility(types.MinIOBucket{BucketName: bucketName, Owner: ownerCandidate})
		if utils.IsRustFSConfig(cfg) {
			visibility = utils.VisibilityFromObjectStorageTags(metadata)
		}

		var allowedUsers []string
		if utils.IsRustFSConfig(cfg) {
			allowedUsers = utils.AllowedUsersFromTags(metadata)
		} else if visibility == types.RESTRICTED {
			allowedUsers, err = objectStorageIAM.GetClient(c.Request.Context()).GetBucketMembers(bucketName)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error retrieving bucket members: %v", err))
				return
			}
		}

		if !isAdmin && utils.IsRustFSConfig(cfg) && !utils.UserAllowedByTags(cfg, requester, metadata) {
			c.String(http.StatusForbidden, fmt.Sprintf("User '%s' is not authorised", requester))
			return
		}
		if !isAdmin && visibility == "" {
			c.String(http.StatusForbidden, fmt.Sprintf("User '%s' is not authorised", requester))
			return
		}

		pageToken := c.DefaultQuery("page", "")
		limit := int64(cfg.JobListingLimit)
		listObjectsInput := &s3.ListObjectsV2Input{
			Bucket:            &bucketName,
			MaxKeys:           &limit,
			ContinuationToken: nil,
		}
		if pageToken != "" {
			listObjectsInput.ContinuationToken = &pageToken
		}

		s3Client := cfg.MinIOProvider.GetS3Client()
		listResult, err := s3Client.ListObjectsV2(listObjectsInput)

		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error listing objects in bucket '%s': %v", bucketName, err))
			return
		}

		allObjects := []types.MinIOObject{}
		returnedItemCount := 0
		for k := range listResult.Contents {
			singleObject := types.MinIOObject{
				ObjectName:   *listResult.Contents[k].Key,
				SizeBytes:    *listResult.Contents[k].Size,
				LastModified: string(listResult.Contents[k].LastModified.String()),
			}
			allObjects = append(allObjects, singleObject)
			returnedItemCount++
		}
		var dataUsage madmin.DataUsageInfo
		if !utils.IsRustFSConfig(cfg) {
			dataUsage, err = objectStorageIAM.GetClient(c.Request.Context()).GetDataUsageInfo()
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error getting MinIO data usage: %v", err))
				return
			}
		}
		bucketInfo := types.MinIOBucket{
			BucketName:   bucketName,
			Visibility:   visibility,
			Owner:        ownerCandidate,
			AllowedUsers: allowedUsers,
			Metadata:     metadata,
			Objects:      allObjects,
		}
		if utils.IsRustFSConfig(cfg) {
			bucketInfo.StorageQuota = utils.RustFSBucketStorageQuota(objectStorageIAM.GetClient(c.Request.Context()), bucketName, metadata)
			bucketInfo.StorageUsage = &types.MinIOUsage{Used: "0"}
			bucketInfo.Attribution = "partial"
		} else {
			if err := objectStorageIAM.GetClient(c.Request.Context()).EnrichBucketQuotaAndUsage(&bucketInfo, dataUsage); err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error getting bucket quota or usage for bucket '%s': %v", bucketName, err))
				return
			}
		}

		response := BucketListResponse{
			MinIOBucket:   bucketInfo,
			IsTruncated:   *listResult.IsTruncated,
			ReturnedItems: returnedItemCount,
		}
		if listResult.NextContinuationToken != nil {
			response.NextPage = *listResult.NextContinuationToken
		}

		c.JSON(http.StatusOK, response)
	}
}

type BucketListResponse struct {
	types.MinIOBucket
	NextPage      string `json:"next_page,omitempty"`
	IsTruncated   bool   `json:"is_truncated"`
	ReturnedItems int    `json:"returned_items"`
}
