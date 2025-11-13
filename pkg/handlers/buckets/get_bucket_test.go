package buckets

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/testsupport"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestMakeGetBucketHandlerAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	lastModified := "2024-05-10T12:00:00Z"

	testsupport.SkipIfCannotListen(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/demo" &&
			r.URL.RawQuery == "list-type=2&max-keys=0" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
				<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
					<Name>demo</Name>
					<Prefix></Prefix>
					<KeyCount>1</KeyCount>
					<MaxKeys>1</MaxKeys>
					<Contents>
						<Key>file.txt</Key>
						<LastModified>` + lastModified + `</LastModified>
						<ETag>"d41d8cd98f00b204e9800998ecf8427e"</ETag>
						<Size>42</Size>
						<StorageClass>STANDARD</StorageClass>
					</Contents>
					<IsTruncated>false</IsTruncated>
				</ListBucketResult>`))
			return
		} else if r.Method == http.MethodGet && r.URL.Path == "/demo" &&
			r.URL.RawQuery == "location=" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"success"}`))
			return
		} else if r.Method == http.MethodGet && r.URL.Path == "/minio/admin/v3/info-canned-policy" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"PolicyName": "demo", "Policy": {"Version": "version","Statement": [{"Resource": ["arn:aws:s3:::demo/*"]}]}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
		return
	}))

	cfg := &types.Config{
		Name:        "oscar",
		Namespace:   "oscar",
		ServicePort: 8080,
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}
	kubeClientset := testclient.NewSimpleClientset()

	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "somelonguid@egi.eu")
		c.Set("multitenancyConfig", auth.NewMultitenancyConfig(kubeClientset, "somelonguid@egi.eu"))
		c.Next()
	})
	router.GET("/system/buckets/:bucket", MakeGetHandler(cfg))

	req := httptest.NewRequest(http.MethodGet, "/system/buckets/demo", nil)
	req.Header.Add("Authorization", "Bearer token")
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

	// Close the fake MinIO server
	defer server.Close()
}

func TestMakeGetBucketHandlerForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testsupport.SkipIfCannotListen(t)
	const listXML = `<?xml version="1.0" encoding="UTF-8"?>
			<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
				<Owner>
					<ID>owner</ID>
					<DisplayName>owner</DisplayName>
				</Owner>
				<Buckets>
					<Bucket>
						<Name>demo</Name>
						<CreationDate>2024-01-01T00:00:00Z</CreationDate>
					</Bucket>
				</Buckets>
			</ListAllMyBucketsResult>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	// Close the fake MinIO server
	defer server.Close()
}

func TestMakeGetBucketHandlerRestrictedMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lastModified := "2024-01-02T03:04:05Z"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/demo" &&
			r.URL.RawQuery == "list-type=2&max-keys=0" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
				<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
					<Name>demo</Name>
					<Prefix></Prefix>
					<KeyCount>1</KeyCount>
					<MaxKeys>1</MaxKeys>
					<Contents>
						<Key>file.txt</Key>
						<LastModified>` + lastModified + `</LastModified>
						<ETag>"d41d8cd98f00b204e9800998ecf8427e"</ETag>
						<Size>42</Size>
						<StorageClass>STANDARD</StorageClass>
					</Contents>
					<IsTruncated>false</IsTruncated>
				</ListBucketResult>`))
			return
		} else if r.Method == http.MethodGet && r.URL.Path == "/minio/admin/v3/group" &&
			r.URL.RawQuery == "name=demo" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Group":{"name":"demo","members":["bob"],"policy":"readonly"}}`))
			return
		} else if r.Method == http.MethodGet && r.URL.Path == "/demo" &&
			r.URL.RawQuery == "location=" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"success"}`))
			return
		} else if r.Method == http.MethodGet && r.URL.Path == "/minio/admin/v3/info-canned-policy" &&
			r.URL.RawQuery == "name=bob&v=2" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"PolicyName": "bob", "Policy": {"Version": "version","Statement": [{"Resource": ["arn:aws:s3:::demo/*"]}]}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
		return
	}))

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
		AllowedUsers  []string `json:"allowed_users"`
		NextPage      string   `json:"next_page"`
		ReturnedItems int      `json:"returned_items"`
		IsTruncated   bool     `json:"is_truncated"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(response.Objects) != 1 || response.Objects[0].ObjectName != "file.txt" {
		t.Fatalf("unexpected objects payload: %+v", response.Objects)
	}

	/*if response.AllowedUsers[0] != "bob" {
		t.Fatalf("expected allowed_users [bob], got %v", response.AllowedUsers)
	}*/
	if response.NextPage != "" {
		t.Fatalf("expected empty next_page, got %s", response.NextPage)
	}
	if response.ReturnedItems != 1 {
		t.Fatalf("expected returned_items 1, got %d", response.ReturnedItems)
	}
	if response.IsTruncated {
		t.Fatalf("expected is_truncated false")
	}
	// Close the fake MinIO server
	defer server.Close()
}

func TestMakeGetBucketHandlerPublicBucket(t *testing.T) {
	gin.SetMode(gin.TestMode)

	lastModified := "2024-06-15T10:00:00Z"
	testsupport.SkipIfCannotListen(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/demo" &&
			r.URL.RawQuery == "list-type=2&max-keys=0":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
				<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
					<Name>demo</Name>
					<Prefix></Prefix>
					<KeyCount>1</KeyCount>
					<MaxKeys>1</MaxKeys>
					<Contents>
						<Key>public.txt</Key>
						<LastModified>` + lastModified + `</LastModified>
						<ETag>"abc"</ETag>
						<Size>10</Size>
						<StorageClass>STANDARD</StorageClass>
					</Contents>
					<IsTruncated>false</IsTruncated>
				</ListBucketResult>`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/demo" &&
			r.URL.RawQuery == "location=":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"success"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/minio/admin/v3/info-canned-policy":
			policyName := r.URL.Query().Get("name")
			payload := fmt.Sprintf(`{"PolicyName": "%s", "Policy": {"Version": "version","Statement": [{"Resource": ["arn:aws:s3:::other/*"]}]}}`, policyName)
			if policyName == "all_users_group" {
				payload = fmt.Sprintf(`{"PolicyName": "%s", "Policy": {"Version": "version","Statement": [{"Resource": ["arn:aws:s3:::demo/*"]}]}}`, policyName)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(payload))
			return
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"success"}`))
			return
		}
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
	router.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "charlie")
	})
	router.GET("/system/buckets/:bucket", MakeGetHandler(cfg))

	req := httptest.NewRequest(http.MethodGet, "/system/buckets/demo", nil)
	req.Header.Set("Authorization", "Bearer token")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
}

func TestMakeGetBucketHandlerPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lastModified := "2024-01-02T03:04:05Z"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/demo" &&
			r.URL.RawQuery == "continuation-token=token&list-type=2&max-keys=0" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
				<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
					<Name>demo</Name>
					<Prefix></Prefix>
					<KeyCount>1</KeyCount>
					<MaxKeys>1</MaxKeys>
					<Contents>
						<Key>file.txt</Key>
						<LastModified>` + lastModified + `</LastModified>
						<ETag>"d41d8cd98f00b204e9800998ecf8427e"</ETag>
						<Size>42</Size>
						<StorageClass>STANDARD</StorageClass>
					</Contents>
					<IsTruncated>true</IsTruncated>
					<NextContinuationToken>cursor</NextContinuationToken>
				</ListBucketResult>`))
			return
		} else if r.Method == http.MethodGet && r.URL.Path == "/minio/admin/v3/info-canned-policy" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"PolicyName": "demo", "Policy": {"Version": "version","Statement": [{"Resource": ["arn:aws:s3:::demo/*"]}]}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
		return
	}))

	cfg := &types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}

	//kubeClientset := testclient.NewSimpleClientset()
	router := gin.Default()
	router.GET("/system/buckets/:bucket", MakeGetHandler(cfg))

	req := httptest.NewRequest(http.MethodGet, "/system/buckets/demo?page=token", nil)
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
	// Close the fake MinIO server
	defer server.Close()
}
