package buckets

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/testsupport"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
)

func TestMakeDeleteBucketHandlerValidations(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &types.Config{MinIOProvider: &types.MinIOProvider{}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodDelete, "/system/buckets/", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	c.Request = req

	MakeDeleteHandler(cfg)(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestMakeDeleteBucketHandlerBucketNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testsupport.SkipIfCannotListen(t)

	server := startS3Server(t, []string{"other-bucket"})
	defer server.Close()

	cfg := &types.Config{
		Name: "oscar",
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			AccessKey: "ak",
			SecretKey: "sk",
			Region:    "us-east-1",
		},
	}

	overrideBucketAdminClient(t, &stubBucketAdmin{})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "alice")
		c.Next()
	})
	router.DELETE("/system/buckets/:bucket", MakeDeleteHandler(cfg))

	req := httptest.NewRequest(http.MethodDelete, "/system/buckets/alice-bucket", nil)
	req.Header.Set("Authorization", "Bearer token")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, res.Code, res.Body.String())
	}
}

func TestMakeDeleteBucketHandlerUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testsupport.SkipIfCannotListen(t)

	server := startS3Server(t, []string{"alice-bucket"})
	defer server.Close()

	cfg := &types.Config{
		Name: "oscar",
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			AccessKey: "ak",
			SecretKey: "sk",
			Region:    "us-east-1",
		},
	}

	admin := &stubBucketAdmin{
		Visibility: utils.PRIVATE,
		ResourceInPolicyFn: func(string, string) bool {
			return false
		},
	}
	overrideBucketAdminClient(t, admin)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "alice")
		c.Next()
	})
	router.DELETE("/system/buckets/:bucket", MakeDeleteHandler(cfg))

	req := httptest.NewRequest(http.MethodDelete, "/system/buckets/alice-bucket", nil)
	req.Header.Set("Authorization", "Bearer token")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, res.Code)
	}
}

func TestMakeDeleteBucketHandlerDeletesWhenAuthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testsupport.SkipIfCannotListen(t)

	server := startS3Server(t, []string{"alice-bucket"})
	defer server.Close()

	cfg := &types.Config{
		Name: "oscar",
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			AccessKey: "ak",
			SecretKey: "sk",
			Region:    "us-east-1",
		},
	}

	var deleteCalled bool
	admin := &stubBucketAdmin{
		Visibility:     utils.PRIVATE,
		ResourceAccess: true,
		DeleteBucketsFn: func(*s3.S3, utils.MinIOBucket) error {
			deleteCalled = true
			return nil
		},
	}
	overrideBucketAdminClient(t, admin)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "alice")
		c.Next()
	})
	router.DELETE("/system/buckets/:bucket", MakeDeleteHandler(cfg))

	req := httptest.NewRequest(http.MethodDelete, "/system/buckets/alice-bucket", nil)
	req.Header.Set("Authorization", "Bearer token")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}
	if !deleteCalled {
		t.Fatalf("expected DeleteBuckets to be called")
	}
}

func startS3Server(t *testing.T, buckets []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Owner><DisplayName>owner</DisplayName><ID>owner</ID></Owner><Buckets>%s</Buckets></ListAllMyBucketsResult>`, renderBuckets(buckets))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
}

func renderBuckets(names []string) string {
	var sb strings.Builder
	for _, name := range names {
		sb.WriteString("<Bucket><Name>")
		sb.WriteString(name)
		sb.WriteString("</Name></Bucket>")
	}
	return sb.String()
}
