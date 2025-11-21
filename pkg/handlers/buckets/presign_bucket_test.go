package buckets

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
)

func TestMakePresignHandler_AdminUpload(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	fakeClient := &fakeAdminClient{
		metadata:   map[string]string{"owner": "oscar"},
		visibility: utils.PRIVATE,
		simple: &fakeSimpleClient{
			bucketExists: true,
			presignURL:   "https://example.com/upload",
		},
	}

	overrideFactory(t, fakeClient)

	cfg := &types.Config{Name: "oscar"}
	router := gin.New()
	router.POST("/system/buckets/:bucket/presign", MakePresignHandler(cfg))

	reqBody := `{"object_key":"path/hello.txt","operation":"upload","expires_in":120,"content_type":"text/plain","extra_headers":{"x-amz-meta-test":"value"}}`
	req := httptest.NewRequest(http.MethodPost, "/system/buckets/test-bucket/presign", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var output PresignResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &output); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if output.Method != http.MethodPut {
		t.Fatalf("expected method PUT, got %s", output.Method)
	}
	if output.URL != fakeClient.simple.presignURL {
		t.Fatalf("expected URL %s, got %s", fakeClient.simple.presignURL, output.URL)
	}
	if fakeClient.simple.lastMethod != http.MethodPut {
		t.Fatalf("expected PresignHeader to be invoked with PUT, got %s", fakeClient.simple.lastMethod)
	}
	if fakeClient.simple.lastHeaders.Get("Content-Type") != "text/plain" {
		t.Fatalf("expected Content-Type header to be signed")
	}
	if fakeClient.simple.lastHeaders.Get("X-Amz-Meta-Test") != "value" {
		t.Fatalf("expected x-amz-meta-test header to be signed")
	}
}

func TestMakePresignHandler_UserUnauthorized(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	fakeClient := &fakeAdminClient{
		metadata:   map[string]string{"owner": "admin"},
		visibility: utils.PRIVATE,
		simple: &fakeSimpleClient{
			bucketExists: true,
			presignURL:   "https://example.com/should-not-be-used",
		},
	}

	overrideFactory(t, fakeClient)

	cfg := &types.Config{Name: "oscar"}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "alice")
		c.Next()
	})
	router.POST("/system/buckets/:bucket/presign", MakePresignHandler(cfg))

	reqBody := `{"object_key":"hello.txt","operation":"upload","expires_in":120,"content_type":"text/plain"}`
	req := httptest.NewRequest(http.MethodPost, "/system/buckets/test-bucket/presign", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer fake-token")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.Code)
	}
	if fakeClient.simple.lastMethod != "" {
		t.Fatalf("expected presign not to be called for unauthorized user")
	}
}

func TestMakePresignHandler_RestrictedMemberDownload(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	fakeClient := &fakeAdminClient{
		metadata:   map[string]string{"owner": "bob"},
		visibility: utils.RESTRICTED,
		members:    []string{"alice"},
		simple: &fakeSimpleClient{
			bucketExists: true,
			presignURL:   "https://example.com/download",
		},
	}

	overrideFactory(t, fakeClient)

	cfg := &types.Config{Name: "oscar"}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "alice")
		c.Next()
	})
	router.POST("/system/buckets/:bucket/presign", MakePresignHandler(cfg))

	reqBody := `{"object_key":"hello.txt","operation":"download","expires_in":300,"content_type":"text/plain"}`
	req := httptest.NewRequest(http.MethodPost, "/system/buckets/test-bucket/presign", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer fake-token")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var output PresignResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &output); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if output.Method != http.MethodGet {
		t.Fatalf("expected method GET, got %s", output.Method)
	}
	if output.Operation != operationDownload {
		t.Fatalf("expected operation download, got %s", output.Operation)
	}
	if fakeClient.simple.lastMethod != http.MethodGet {
		t.Fatalf("expected PresignHeader to be invoked with GET, got %s", fakeClient.simple.lastMethod)
	}
	if fakeClient.simple.lastReqParams == nil {
		t.Fatalf("expected response parameters to be set for download")
	}
	if got := fakeClient.simple.lastReqParams.Get("response-content-type"); got != "text/plain" {
		t.Fatalf("expected response-content-type to be text/plain, got %s", got)
	}
}

func overrideFactory(t *testing.T, fakeClient *fakeAdminClient) {
	t.Helper()
	originalFactory := newPresignAdminClient
	newPresignAdminClient = func(cfg *types.Config) (presignAdminClient, error) {
		return fakeClient, nil
	}
	t.Cleanup(func() {
		newPresignAdminClient = originalFactory
	})
}

type fakeAdminClient struct {
	metadata   map[string]string
	visibility string
	members    []string
	simple     *fakeSimpleClient
	policies   map[string]bool
}

func (f *fakeAdminClient) SimpleClient() presignSimpleClient {
	return f.simple
}

func (f *fakeAdminClient) GetTaggedMetadata(bucket string) (map[string]string, error) {
	return f.metadata, nil
}

func (f *fakeAdminClient) GetCurrentResourceVisibility(bucket utils.MinIOBucket) string {
	return f.visibility
}

func (f *fakeAdminClient) ResourceInPolicy(policyName string, resource string) bool {
	if f.policies == nil {
		return false
	}
	return f.policies[policyName+"|"+resource]
}

func (f *fakeAdminClient) GetBucketMembers(bucket string) ([]string, error) {
	return f.members, nil
}

type fakeSimpleClient struct {
	bucketExists bool
	presignURL   string

	lastMethod    string
	lastHeaders   http.Header
	lastBucket    string
	lastObject    string
	lastReqParams url.Values
}

func (f *fakeSimpleClient) BucketExists(ctx context.Context, bucket string) (bool, error) {
	f.lastBucket = bucket
	return f.bucketExists, nil
}

func (f *fakeSimpleClient) PresignHeader(ctx context.Context, method string, bucketName string, objectName string, expires time.Duration, reqParams url.Values, extraHeaders http.Header) (*url.URL, error) {
	f.lastMethod = method
	f.lastHeaders = extraHeaders
	f.lastBucket = bucketName
	f.lastObject = objectName
	f.lastReqParams = reqParams
	return url.Parse(f.presignURL)
}
