package buckets

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/testsupport"
	"github.com/grycap/oscar/v3/pkg/types"
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
	//testsupport.SkipIfCannotListen(t)
	bucketNameTest := "alice"
	//server := startS3Server(t, []string{bucketNameTest})
	//defer server.Close()
	const listXML = `<?xml version="1.0" encoding="UTF-8"?>
					<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
						<Owner>
							<ID>owner</ID>
							<DisplayName>owner</DisplayName>
						</Owner>
						<Buckets>
							<Bucket>
								<Name>alice</Name>
								<CreationDate>2024-01-01T00:00:00Z</CreationDate>
							</Bucket>
						</Buckets>
					</ListAllMyBucketsResult>`

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		if hreq.URL.Path == "/" && hreq.Method == http.MethodGet {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(listXML))
			return
		} else if strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/info-canned-policy") && hreq.Method == http.MethodGet {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"PolicyName": "testpolicy", "Policy": {"Version": "version","Statement": [{"Resource": ["arn:aws:s3:::alice/*"]}]}}`))
		} else {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(`{"status": "success"}`))
		}
	}))

	cfg := &types.Config{
		Name: "oscar",
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "alice")
		c.Next()
	})

	router.DELETE("/system/buckets/:bucket", MakeDeleteHandler(cfg))
	req := httptest.NewRequest(http.MethodDelete, "/system/buckets/"+bucketNameTest, nil)
	req.Header.Set("Authorization", "Bearer token")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}

	// Close the fake MinIO server
	defer server.Close()
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
