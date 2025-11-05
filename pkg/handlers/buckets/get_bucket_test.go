package buckets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
)

func TestMakeGetBucketHandlerAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	lastModified := "2024-05-10T12:00:00Z"

	fakeAdmin := &fakeBucketAdminClient{
		metadata:   map[string]string{"owner": "alice"},
		visibility: utils.PUBLIC,
	}
	overrideBucketAdminFactory(t, fakeAdmin)
	overrideBucketObjectFactory(t, func(cfg *types.Config, c *gin.Context, requester string, isAdmin bool) (bucketObjectClient, error) {
		return &fakeBucketObjectClient{
			exists: true,
			objects: []utils.MinIOObject{
				{ObjectName: "file.txt", SizeBytes: 42, Owner: "alice", LastModified: lastModified},
			},
		}, nil
	})

	cfg := &types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  "http://127.0.0.1:9000",
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}

	router := gin.New()
	router.GET("/system/buckets/:bucket", MakeGetHandler(cfg))

	req := httptest.NewRequest(http.MethodGet, "/system/buckets/demo", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}

	var response struct {
		Objects []struct {
			ObjectName   string `json:"object_name"`
			SizeBytes    int64  `json:"size_bytes"`
			LastModified string `json:"last_modified"`
		} `json:"objects"`
		NextPage      string `json:"next_page"`
		ReturnedItems int    `json:"returned_items"`
		IsTruncated   bool   `json:"is_truncated"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(response.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(response.Objects))
	}
	if response.Objects[0].ObjectName != "file.txt" || response.Objects[0].SizeBytes != 42 {
		t.Fatalf("unexpected object payload: %+v", response.Objects[0])
	}
	if response.Objects[0].LastModified != lastModified {
		t.Fatalf("expected last_modified %s, got %s", lastModified, response.Objects[0].LastModified)
	}
	if response.NextPage != "" {
		t.Fatalf("expected empty next_page, got %s", response.NextPage)
	}
	if response.ReturnedItems != 1 {
		t.Fatalf("expected returned_items 1, got %d", response.ReturnedItems)
	}
	if response.IsTruncated {
		t.Fatalf("expected is_truncated false")
	}

	var raw struct {
		Objects []map[string]interface{} `json:"objects"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to unmarshal raw response: %v", err)
	}
	if _, hasOwner := raw.Objects[0]["owner"]; hasOwner {
		t.Fatalf("unexpected owner field: %+v", raw.Objects[0])
	}
	if got := raw.Objects[0]["last_modified"]; got != lastModified {
		t.Fatalf("expected raw last_modified %s, got %v", lastModified, got)
	}
}

func TestMakeGetBucketHandlerForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fakeAdmin := &fakeBucketAdminClient{
		metadata:   map[string]string{"owner": "alice"},
		visibility: utils.PRIVATE,
		policies:   map[string]bool{},
	}
	overrideBucketAdminFactory(t, fakeAdmin)
	overrideBucketObjectFactory(t, func(cfg *types.Config, c *gin.Context, requester string, isAdmin bool) (bucketObjectClient, error) {
		return &fakeBucketObjectClient{exists: true}, nil
	})

	cfg := &types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  "http://127.0.0.1:9000",
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "bob")
	})
	router.GET("/system/buckets/:bucket", MakeGetHandler(cfg))

	req := httptest.NewRequest(http.MethodGet, "/system/buckets/demo", nil)
	req.Header.Set("Authorization", "Bearer token")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, res.Code)
	}
}

func TestMakeGetBucketHandlerRestrictedMember(t *testing.T) {
	gin.SetMode(gin.TestMode)

	lastModified := "2024-01-02T03:04:05Z"

	fakeAdmin := &fakeBucketAdminClient{
		metadata:   map[string]string{"owner": "alice"},
		visibility: utils.RESTRICTED,
		members:    []string{"bob"},
	}
	overrideBucketAdminFactory(t, fakeAdmin)
	overrideBucketObjectFactory(t, func(cfg *types.Config, c *gin.Context, requester string, isAdmin bool) (bucketObjectClient, error) {
		return &fakeBucketObjectClient{
			exists: true,
			objects: []utils.MinIOObject{
				{ObjectName: "nested/data.bin", SizeBytes: 1024, Owner: "alice", LastModified: lastModified},
			},
		}, nil
	})

	cfg := &types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  "http://127.0.0.1:9000",
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "bob")
	})
	router.GET("/system/buckets/:bucket", MakeGetHandler(cfg))

	req := httptest.NewRequest(http.MethodGet, "/system/buckets/demo", nil)
	req.Header.Set("Authorization", "Bearer token")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}

	var response struct {
		Objects []struct {
			ObjectName   string `json:"object_name"`
			LastModified string `json:"last_modified"`
		} `json:"objects"`
		NextPage      string `json:"next_page"`
		ReturnedItems int    `json:"returned_items"`
		IsTruncated   bool   `json:"is_truncated"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(response.Objects) != 1 || response.Objects[0].ObjectName != "nested/data.bin" {
		t.Fatalf("unexpected objects payload: %+v", response.Objects)
	}
	if response.Objects[0].LastModified != lastModified {
		t.Fatalf("expected last_modified %s, got %s", lastModified, response.Objects[0].LastModified)
	}
	if response.NextPage != "" {
		t.Fatalf("expected empty next_page, got %s", response.NextPage)
	}
	if response.ReturnedItems != 1 {
		t.Fatalf("expected returned_items 1, got %d", response.ReturnedItems)
	}
	if response.IsTruncated {
		t.Fatalf("expected is_truncated false")
	}
}

func TestMakeGetBucketHandlerPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fakeAdmin := &fakeBucketAdminClient{
		metadata:   map[string]string{"owner": "alice"},
		visibility: utils.PUBLIC,
	}
	overrideBucketAdminFactory(t, fakeAdmin)
	overrideBucketObjectFactory(t, func(cfg *types.Config, c *gin.Context, requester string, isAdmin bool) (bucketObjectClient, error) {
		return &fakeBucketObjectClient{
			exists:      true,
			objects:     []utils.MinIOObject{{ObjectName: "a.txt", SizeBytes: 10}},
			nextToken:   "cursor",
			isTruncated: true,
		}, nil
	})

	cfg := &types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  "http://127.0.0.1:9000",
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}

	router := gin.New()
	router.GET("/system/buckets/:bucket", MakeGetHandler(cfg))

	req := httptest.NewRequest(http.MethodGet, "/system/buckets/demo?page=token&limit=1", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}

	var response struct {
		NextPage    string `json:"next_page"`
		IsTruncated bool   `json:"is_truncated"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if response.NextPage != "cursor" {
		t.Fatalf("expected next_page cursor, got %s", response.NextPage)
	}
	if !response.IsTruncated {
		t.Fatalf("expected is_truncated true")
	}
}

func overrideBucketAdminFactory(t *testing.T, fakeClient *fakeBucketAdminClient) {
	t.Helper()
	original := newBucketAdminClient
	newBucketAdminClient = func(cfg *types.Config) (bucketAdminClient, error) {
		return fakeClient, nil
	}
	t.Cleanup(func() {
		newBucketAdminClient = original
	})
}

func overrideBucketObjectFactory(t *testing.T, factory func(cfg *types.Config, c *gin.Context, requester string, isAdmin bool) (bucketObjectClient, error)) {
	t.Helper()
	original := newBucketObjectClient
	newBucketObjectClient = factory
	t.Cleanup(func() {
		newBucketObjectClient = original
	})
}

type fakeBucketAdminClient struct {
	metadata                map[string]string
	metadataErr             error
	visibility              string
	members                 []string
	policies                map[string]bool
	removeResourceErr       error
	removeGroupPolicyErr    error
	deleteErr               error
	removeResourceCalled    bool
	removeGroupPolicyCalled bool
	deleteCalled            bool
}

func (f *fakeBucketAdminClient) GetTaggedMetadata(bucket string) (map[string]string, error) {
	if f.metadataErr != nil {
		return nil, f.metadataErr
	}
	return f.metadata, nil
}

func (f *fakeBucketAdminClient) GetCurrentResourceVisibility(bucket utils.MinIOBucket) string {
	return f.visibility
}

func (f *fakeBucketAdminClient) GetBucketMembers(bucket string) ([]string, error) {
	return f.members, nil
}

func (f *fakeBucketAdminClient) ResourceInPolicy(policyName string, resource string) bool {
	if f.policies == nil {
		return false
	}
	return f.policies[policyName+"|"+resource]
}

func (f *fakeBucketAdminClient) RemoveResource(bucketName string, policyName string, isGroup bool) error {
	f.removeResourceCalled = true
	if f.removeResourceErr != nil {
		return f.removeResourceErr
	}
	return nil
}

func (f *fakeBucketAdminClient) RemoveGroupPolicy(bucket string) error {
	f.removeGroupPolicyCalled = true
	if f.removeGroupPolicyErr != nil {
		return f.removeGroupPolicyErr
	}
	return nil
}

func (f *fakeBucketAdminClient) DeleteBucket(s3Client *s3.S3, bucketName string) error {
	f.deleteCalled = true
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return nil
}

type fakeBucketObjectClient struct {
	exists      bool
	existsErr   error
	objects     []utils.MinIOObject
	nextToken   string
	isTruncated bool
	listErr     error
	statInfo    *utils.MinIOObjectInfo
	statErr     error
}

func (f *fakeBucketObjectClient) BucketExists(ctx context.Context, bucket string) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}
	return f.exists, nil
}

func (f *fakeBucketObjectClient) ListObjects(ctx context.Context, bucket string, includeOwner bool, limit int, continuation string) (*utils.MinIOListResult, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return &utils.MinIOListResult{
		Objects:           f.objects,
		NextToken:         f.nextToken,
		IsTruncated:       f.isTruncated,
		ReturnedItemCount: len(f.objects),
	}, nil
}

func (f *fakeBucketObjectClient) StatObject(ctx context.Context, bucket string, object string) (*utils.MinIOObjectInfo, error) {
	if f.statErr != nil {
		return nil, f.statErr
	}
	if f.statInfo == nil {
		return nil, fmt.Errorf("stat not configured")
	}
	return f.statInfo, nil
}
