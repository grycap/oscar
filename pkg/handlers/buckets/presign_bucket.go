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
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	"github.com/minio/minio-go/v7"
)

const (
	defaultPresignExpirySeconds = 900
	maxPresignExpirySeconds     = 3600
	minPresignExpirySeconds     = 1
	operationUpload             = "upload"
	operationDownload           = "download"
)

type adminClientFactory func(cfg *types.Config) (presignAdminClient, error)

var newPresignAdminClient adminClientFactory = func(cfg *types.Config) (presignAdminClient, error) {
	client, err := utils.MakeMinIOAdminClient(cfg)
	if err != nil {
		return nil, err
	}
	return &defaultPresignAdminClient{delegate: client}, nil
}

type presignAdminClient interface {
	SimpleClient() presignSimpleClient
	GetTaggedMetadata(bucket string) (map[string]string, error)
	GetCurrentResourceVisibility(bucket utils.MinIOBucket) string
	ResourceInPolicy(policyName string, resource string) bool
	GetBucketMembers(bucket string) ([]string, error)
}

type presignSimpleClient interface {
	BucketExists(ctx context.Context, bucket string) (bool, error)
	PresignHeader(ctx context.Context, method string, bucketName string, objectName string, expires time.Duration, reqParams url.Values, extraHeaders http.Header) (*url.URL, error)
}

type PresignRequest struct {
	ObjectKey    string            `json:"object_key" binding:"required"`
	Operation    string            `json:"operation" binding:"required"`
	ExpiresIn    int64             `json:"expires_in"`
	ContentType  string            `json:"content_type"`
	ExtraHeaders map[string]string `json:"extra_headers"`
}

type PresignResponse struct {
	ObjectKey string            `json:"object_key"`
	Operation string            `json:"operation"`
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	ExpiresAt string            `json:"expires_at"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// MakePresignHandler godoc
// @Summary Generate presigned URL
// @Description Create a short-lived MinIO presigned URL to upload or download an object.
// @Tags buckets
// @Accept json
// @Produce json
// @Param bucket path string true "Bucket name"
// @Param request body buckets.PresignRequest true "Presign parameters"
// @Success 200 {object} buckets.PresignResponse
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/buckets/{bucket}/presign [post]
func MakePresignHandler(cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req PresignRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("Invalid presign request: %v", err))
			return
		}

		bucketName := strings.TrimSpace(c.Param("bucket"))
		if bucketName == "" {
			c.String(http.StatusBadRequest, "Bucket parameter cannot be empty")
			return
		}

		objectKey := strings.Trim(strings.TrimSpace(req.ObjectKey), "/")
		if objectKey == "" {
			c.String(http.StatusBadRequest, "Object key cannot be empty")
			return
		}

		operation := strings.ToLower(strings.TrimSpace(req.Operation))
		if operation != operationUpload && operation != operationDownload {
			c.String(http.StatusBadRequest, fmt.Sprintf("Unsupported operation '%s'. Allowed values are '%s' or '%s'", req.Operation, operationUpload, operationDownload))
			return
		}

		expires := req.ExpiresIn
		if expires == 0 {
			expires = defaultPresignExpirySeconds
		}
		if expires < minPresignExpirySeconds || expires > maxPresignExpirySeconds {
			c.String(http.StatusBadRequest, fmt.Sprintf("Invalid expiration requested: %d. Allowed range is %d-%d seconds", expires, minPresignExpirySeconds, maxPresignExpirySeconds))
			return
		}

		adminClient, err := newPresignAdminClient(cfg)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error creating MinIO admin client: %v", err))
			return
		}

		ctx := c.Request.Context()
		minioClient := adminClient.SimpleClient()
		exists, err := minioClient.BucketExists(ctx, bucketName)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error validating bucket '%s': %v", bucketName, err))
			return
		}
		if !exists {
			c.String(http.StatusNotFound, fmt.Sprintf("Bucket '%s' not found", bucketName))
			return
		}

		authHeader := c.GetHeader("Authorization")
		isAdmin := len(strings.Split(authHeader, "Bearer")) == 1

		requester := cfg.Name
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

		metadata, metaErr := adminClient.GetTaggedMetadata(bucketName)
		if metaErr != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error retrieving metadata for bucket '%s': %v", bucketName, metaErr))
			return
		}

		owner := metadata["owner"]
		if owner == "" {
			owner = types.DefaultOwner
		}

		visibility := adminClient.GetCurrentResourceVisibility(utils.MinIOBucket{BucketName: bucketName, Owner: owner})
		if visibility == "" && !isAdmin {
			visibility = adminClient.GetCurrentResourceVisibility(utils.MinIOBucket{BucketName: bucketName, Owner: requester})
		}

		if !isAdmin {
			allowed := requester == owner
			if !allowed {
				switch visibility {
				case utils.PUBLIC:
					allowed = true
				case utils.PRIVATE:
					allowed = adminClient.ResourceInPolicy(requester, bucketName)
				case utils.RESTRICTED:
					if adminClient.ResourceInPolicy(requester, bucketName) {
						allowed = true
					} else {
						members, memberErr := adminClient.GetBucketMembers(bucketName)
						if memberErr != nil {
							c.String(http.StatusInternalServerError, fmt.Sprintf("Error retrieving bucket members: %v", memberErr))
							return
						}
						for _, member := range members {
							if member == requester {
								allowed = true
								break
							}
						}
					}
				default:
					allowed = adminClient.ResourceInPolicy(requester, bucketName)
				}
			}

			if !allowed {
				c.String(http.StatusForbidden, fmt.Sprintf("User '%s' is not authorised to generate presigned URLs for bucket '%s'", requester, bucketName))
				return
			}
		}

		var reqParams url.Values
		if operation == operationDownload && req.ContentType != "" {
			reqParams = url.Values{}
			reqParams.Set("response-content-type", req.ContentType)
		}

		headers := buildSignedHeaders(operation, req.ContentType, req.ExtraHeaders)

		var presignedURL *url.URL
		method := http.MethodGet
		if operation == operationUpload {
			method = http.MethodPut
		}

		presignedURL, err = minioClient.PresignHeader(ctx, method, bucketName, objectKey, time.Duration(expires)*time.Second, reqParams, headers)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error generating presigned URL: %v", err))
			return
		}

		respHeaders := flattenHeaders(headers)
		expiresAt := time.Now().UTC().Add(time.Duration(expires) * time.Second).Format(time.RFC3339)

		c.JSON(http.StatusOK, PresignResponse{
			ObjectKey: objectKey,
			Operation: operation,
			Method:    method,
			URL:       presignedURL.String(),
			ExpiresAt: expiresAt,
			Headers:   respHeaders,
		})
	}
}

func buildSignedHeaders(operation string, contentType string, extra map[string]string) http.Header {
	if len(extra) == 0 && (contentType == "" || operation == operationDownload) {
		return nil
	}

	headers := http.Header{}
	if operation == operationUpload && contentType != "" {
		headers.Set("Content-Type", contentType)
	}

	for k, v := range extra {
		key := http.CanonicalHeaderKey(k)
		headers.Set(key, v)
	}

	if len(headers) == 0 {
		return nil
	}

	return headers
}

func flattenHeaders(headers http.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	result := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) == 0 {
			continue
		}
		result[key] = values[0]
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

type defaultPresignAdminClient struct {
	delegate *utils.MinIOAdminClient
}

func (d *defaultPresignAdminClient) SimpleClient() presignSimpleClient {
	return &defaultPresignSimpleClient{client: d.delegate.GetSimpleClient()}
}

func (d *defaultPresignAdminClient) GetTaggedMetadata(bucket string) (map[string]string, error) {
	return d.delegate.GetTaggedMetadata(bucket)
}

func (d *defaultPresignAdminClient) GetCurrentResourceVisibility(bucket utils.MinIOBucket) string {
	return d.delegate.GetCurrentResourceVisibility(bucket)
}

func (d *defaultPresignAdminClient) ResourceInPolicy(policyName string, resource string) bool {
	return d.delegate.ResourceInPolicy(policyName, resource)
}

func (d *defaultPresignAdminClient) GetBucketMembers(bucket string) ([]string, error) {
	return d.delegate.GetBucketMembers(bucket)
}

type defaultPresignSimpleClient struct {
	client *minio.Client
}

func (d *defaultPresignSimpleClient) BucketExists(ctx context.Context, bucket string) (bool, error) {
	return d.client.BucketExists(ctx, bucket)
}

func (d *defaultPresignSimpleClient) PresignHeader(ctx context.Context, method string, bucketName string, objectName string, expires time.Duration, reqParams url.Values, extraHeaders http.Header) (*url.URL, error) {
	return d.client.PresignHeader(ctx, method, bucketName, objectName, expires, reqParams, extraHeaders)
}
