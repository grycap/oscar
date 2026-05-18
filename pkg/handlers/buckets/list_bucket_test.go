package buckets

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/testsupport"
	"github.com/grycap/oscar/v3/pkg/types"
)

func TestMakeListBucketHandlerAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const listXML = `<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
    <Owner>
        <ID>owner</ID>
        <DisplayName>owner</DisplayName>
    </Owner>
    <Buckets>
        <Bucket>
            <Name>bucket-one</Name>
            <CreationDate>2024-01-01T00:00:00Z</CreationDate>
        </Bucket>
    </Buckets>
</ListAllMyBucketsResult>`

	testsupport.SkipIfCannotListen(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/minio/admin/v3/datausageinfo" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"bucketsUsageInfo":{"bucket-one":{"size":42,"objectsCount":1}}}`))
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/minio/admin/v3/get-bucket-quota" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"quota":0}`))
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(listXML))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}

	router := gin.New()
	router.GET("/system/buckets", MakeListHandler(cfg))

	req, err := http.NewRequest(http.MethodGet, "/system/buckets", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, res.Code, res.Body.String())
	}

	Buckets := []struct {
		BucketName   string `json:"bucket_name"`
		Visibility   string `json:"visibility"`
		AllowedUsers string `json:"allowed_users"`
		Owner        string `json:"owner"`
		StorageQuota struct {
			Source string `json:"source"`
		} `json:"storage_quota"`
		StorageUsage struct {
			UsedBytes int64 `json:"used_bytes"`
			Objects   int64 `json:"objects"`
		} `json:"storage_usage"`
		Attribution string `json:"attribution"`
	}{}
	if err := json.Unmarshal(res.Body.Bytes(), &Buckets); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(Buckets) != 1 || Buckets[0].BucketName != "bucket-one" {
		t.Fatalf("unexpected response payload: %v", Buckets)
	}
	if Buckets[0].StorageQuota.Source != "unset" {
		t.Fatalf("expected unset storage quota, got %+v", Buckets[0].StorageQuota)
	}
	if Buckets[0].StorageUsage.UsedBytes != 42 || Buckets[0].StorageUsage.Objects != 1 {
		t.Fatalf("unexpected storage usage: %+v", Buckets[0].StorageUsage)
	}
	if Buckets[0].Attribution != "complete" {
		t.Fatalf("expected complete attribution, got %s", Buckets[0].Attribution)
	}
}

func TestMakeListBucketHandlerMissingUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
	router.GET("/system/buckets", MakeListHandler(cfg))

	req, err := http.NewRequest(http.MethodGet, "/system/buckets", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer token")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}
