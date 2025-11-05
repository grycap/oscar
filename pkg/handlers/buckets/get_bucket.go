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
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
)

type bucketAdminClient interface {
	GetTaggedMetadata(bucket string) (map[string]string, error)
	GetCurrentResourceVisibility(bucket utils.MinIOBucket) string
	GetBucketMembers(bucket string) ([]string, error)
	ResourceInPolicy(policyName string, resource string) bool
	RemoveResource(bucketName string, policyName string, isGroup bool) error
	RemoveGroupPolicy(bucket string) error
	DeleteBucket(s3Client *s3.S3, bucketName string) error
}

var newBucketAdminClient = func(cfg *types.Config) (bucketAdminClient, error) {
	client, err := utils.MakeMinIOAdminClient(cfg)
	if err != nil {
		return nil, err
	}
	return &defaultBucketAdminClient{delegate: client}, nil
}

type defaultBucketAdminClient struct {
	delegate *utils.MinIOAdminClient
}

func (d *defaultBucketAdminClient) GetTaggedMetadata(bucket string) (map[string]string, error) {
	return d.delegate.GetTaggedMetadata(bucket)
}

func (d *defaultBucketAdminClient) GetCurrentResourceVisibility(bucket utils.MinIOBucket) string {
	return d.delegate.GetCurrentResourceVisibility(bucket)
}

func (d *defaultBucketAdminClient) GetBucketMembers(bucket string) ([]string, error) {
	return d.delegate.GetBucketMembers(bucket)
}

func (d *defaultBucketAdminClient) ResourceInPolicy(policyName string, resource string) bool {
	return d.delegate.ResourceInPolicy(policyName, resource)
}

func (d *defaultBucketAdminClient) RemoveResource(bucketName string, policyName string, isGroup bool) error {
	return d.delegate.RemoveResource(bucketName, policyName, isGroup)
}

func (d *defaultBucketAdminClient) RemoveGroupPolicy(bucket string) error {
	return d.delegate.RemoveGroupPolicy(bucket)
}

func (d *defaultBucketAdminClient) DeleteBucket(s3Client *s3.S3, bucketName string) error {
	return d.delegate.DeleteBucket(s3Client, bucketName)
}

type bucketObjectClient interface {
	BucketExists(ctx context.Context, bucket string) (bool, error)
	ListObjects(ctx context.Context, bucket string, includeOwner bool, limit int, continuation string) (*utils.MinIOListResult, error)
	StatObject(ctx context.Context, bucket string, object string) (*utils.MinIOObjectInfo, error)
}

var newBucketObjectClient = func(cfg *types.Config, c *gin.Context, requester string, isAdmin bool) (bucketObjectClient, error) {
	provider := &types.MinIOProvider{
		Endpoint: cfg.MinIOProvider.Endpoint,
		Verify:   cfg.MinIOProvider.Verify,
		Region:   cfg.MinIOProvider.Region,
	}

	if isAdmin {
		provider.AccessKey = cfg.MinIOProvider.AccessKey
		provider.SecretKey = cfg.MinIOProvider.SecretKey
	} else {
		mc, err := auth.GetMultitenancyConfigFromContext(c)
		if err != nil {
			return nil, fmt.Errorf("error getting multitenancy config: %w", err)
		}
		ak, sk, err := mc.GetUserCredentials(requester)
		if err != nil {
			return nil, fmt.Errorf("error getting credentials for MinIO user: %s", requester)
		}
		provider.AccessKey = ak
		provider.SecretKey = sk
	}

	return &defaultBucketObjectClient{client: utils.NewMinIODataClient(provider)}, nil
}

type defaultBucketObjectClient struct {
	client *utils.MinIODataClient
}

func (c *defaultBucketObjectClient) BucketExists(ctx context.Context, bucket string) (bool, error) {
	return c.client.BucketExists(ctx, bucket)
}

func (c *defaultBucketObjectClient) ListObjects(ctx context.Context, bucket string, includeOwner bool, limit int, continuation string) (*utils.MinIOListResult, error) {
	return c.client.ListBucketObjects(ctx, bucket, includeOwner, limit, continuation)
}

func (c *defaultBucketObjectClient) StatObject(ctx context.Context, bucket string, object string) (*utils.MinIOObjectInfo, error) {
	return c.client.StatObject(ctx, bucket, object)
}

// MakeGetHandler makes a handler that returns bucket information including stored objects.
func MakeGetHandler(cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		bucketName := strings.TrimSpace(c.Param("bucket"))
		if bucketName == "" {
			c.String(http.StatusBadRequest, "Bucket parameter cannot be empty")
			return
		}

		adminClient, err := newBucketAdminClient(cfg)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating MinIO admin client: %v", err))
			return
		}

		ctx := c.Request.Context()

		authHeader := c.GetHeader("Authorization")
		isAdmin := len(strings.Split(authHeader, "Bearer")) == 1

		requester := types.DefaultOwner
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

		objectClient, err := newBucketObjectClient(cfg, c, requester, isAdmin)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error configuring storage client: %v", err))
			return
		}

		exists, err := objectClient.BucketExists(ctx, bucketName)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error validating bucket '%s': %v", bucketName, err))
			return
		}
		if !exists {
			c.String(http.StatusNotFound, fmt.Sprintf("Bucket '%s' not found", bucketName))
			return
		}

		metadata, metaErr := adminClient.GetTaggedMetadata(bucketName)
		if metaErr != nil {
			metadata = map[string]string{}
		}

		ownerCandidate := metadata["owner"]
		if ownerCandidate == "" {
			ownerCandidate = types.DefaultOwner
		}

		visibility, resolvedOwner := resolveBucketVisibility(adminClient, bucketName, ownerCandidate, requester)
		if resolvedOwner != "" {
			ownerCandidate = resolvedOwner
		}
		if visibility == "" {
			visibility = utils.PRIVATE
		}

		var allowedUsers []string
		if visibility == utils.RESTRICTED {
			allowedUsers, err = adminClient.GetBucketMembers(bucketName)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Sprintf("Error retrieving bucket members: %v", err))
				return
			}
		}

		if !isAdmin {
			if !isRequesterAuthorised(adminClient, requester, ownerCandidate, bucketName, visibility, allowedUsers) {
				c.String(http.StatusForbidden, fmt.Sprintf("User '%s' is not authorised", requester))
				return
			}
		}

		pageToken := strings.TrimSpace(c.DefaultQuery("page", ""))
		limit := cfg.JobListingLimit
		if limitParam := strings.TrimSpace(c.DefaultQuery("limit", "")); limitParam != "" {
			if parsed, perr := strconv.Atoi(limitParam); perr == nil && parsed >= 0 {
				limit = parsed
			}
		}

		listResult, err := objectClient.ListObjects(ctx, bucketName, false, limit, pageToken)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error retrieving objects for bucket '%s': %v", bucketName, err))
			return
		}
		objects := listResult.Objects
		for i := range objects {
			objects[i].Owner = ""
		}

		delete(metadata, "owner")

		response := bucketListResponse{
			MinIOBucket: utils.MinIOBucket{
				BucketName:   bucketName,
				Visibility:   visibility,
				Owner:        ownerCandidate,
				AllowedUsers: allowedUsers,
				Metadata:     metadata,
				Objects:      objects,
			},
			NextPage:      listResult.NextToken,
			IsTruncated:   listResult.IsTruncated,
			ReturnedItems: listResult.ReturnedItemCount,
		}

		c.JSON(http.StatusOK, response)
	}
}

func resolveBucketVisibility(adminClient bucketAdminClient, bucketName string, owner string, requester string) (string, string) {
	candidates := []string{}
	if owner != "" {
		candidates = append(candidates, owner)
	}
	if requester != "" && requester != owner {
		candidates = append(candidates, requester)
	}
	if owner != types.DefaultOwner {
		candidates = append(candidates, types.DefaultOwner)
	}

	for _, candidate := range candidates {
		visibility := adminClient.GetCurrentResourceVisibility(utils.MinIOBucket{BucketName: bucketName, Owner: candidate})
		if visibility != "" {
			if visibility != utils.PUBLIC {
				return visibility, candidate
			}
			return visibility, owner
		}
	}
	return "", owner
}

func isRequesterAuthorised(adminClient bucketAdminClient, requester string, owner string, bucketName string, visibility string, allowedUsers []string) bool {
	if requester == owner {
		return true
	}

	switch visibility {
	case utils.PUBLIC:
		return true
	case utils.PRIVATE:
		return adminClient.ResourceInPolicy(requester, bucketName)
	case utils.RESTRICTED:
		if adminClient.ResourceInPolicy(requester, bucketName) {
			return true
		}
		return slices.Contains(allowedUsers, requester)
	default:
		return adminClient.ResourceInPolicy(requester, bucketName)
	}
}

type bucketListResponse struct {
	utils.MinIOBucket
	NextPage      string `json:"next_page,omitempty"`
	IsTruncated   bool   `json:"is_truncated"`
	ReturnedItems int    `json:"returned_items"`
}
