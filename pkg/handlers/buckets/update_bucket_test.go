package buckets

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestMakeUpdateBucketHandlerValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &types.Config{MinIOProvider: &types.MinIOProvider{}}

	router := gin.New()
	router.PUT("/system/buckets", MakeUpdateHandler(cfg))

	req, err := http.NewRequest(http.MethodPut, "/system/buckets", http.NoBody)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestMakeUpdateBucketHandler_ServiceBucketForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		if hreq.URL.Path == "/alice-bucket/" && hreq.URL.RawQuery == "tagging=" && hreq.Method == http.MethodGet {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
							<Tagging xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
							<TagSet>
								<Tag>
									<Key>service</Key>
									<Value>true</Value>
								</Tag>
							</TagSet>
							</Tagging>`))
			return
		} else if hreq.URL.Path == "/alice-bucket/" && hreq.Method == http.MethodGet {
			rw.WriteHeader(http.StatusForbidden)
			rw.Write([]byte(`{"Code":"AccessDenied","Message":"Access Denied"}`))
			return
		} else {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(`{"status": "success"}`))
			return
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

	router := buildUpdateRouter(cfg)

	body := `{"bucket_name":"alice-bucket","visibility":"private"}`
	req := httptest.NewRequest(http.MethodPut, "/system/buckets", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, res.Code)
	}
	// Close the fake MinIO server
	defer server.Close()
}

func TestMakeUpdateBucketHandler_VisibilityChange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		if hreq.URL.Path == "/alice-bucket/" && hreq.URL.RawQuery == "tagging=" && hreq.Method == http.MethodGet {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
							<Tagging xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
							<TagSet>
								<Tag>
									<Key>service</Key>
									<Value>false</Value>
					</Tag>
					<Tag><Key>owner</Key><Value>alice</Value></Tag>
				</TagSet>
							</Tagging>`))
			return
		} else if hreq.URL.Path == "/alice-bucket/" && hreq.Method == http.MethodGet {
			rw.WriteHeader(http.StatusForbidden)
			rw.Write([]byte(`{"Code":"AccessDenied","Message":"Access Denied"}`))
			return
		} else {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(`{"status": "success"}`))
			return
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

	router := buildUpdateRouter(cfg)

	body := `{"bucket_name":"alice-bucket","visibility":"public"}`
	req := httptest.NewRequest(http.MethodPut, "/system/buckets", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}
}

func TestMakeUpdateBucketHandler_RestrictedUpdateMembers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const listXML = `<?xml version="1.0" encoding="UTF-8"?>
					<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
						<Owner>
							<ID>owner</ID>
							<DisplayName>owner</DisplayName>
						</Owner>
						<Buckets>
							<Bucket>
								<Name>alice-bucket</Name>
								<CreationDate>2024-01-01T00:00:00Z</CreationDate>
							</Bucket>
						</Buckets>
					</ListAllMyBucketsResult>`
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		if hreq.URL.Path == "/alice-bucket/" && hreq.URL.RawQuery == "tagging=" && hreq.Method == http.MethodGet {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
							<Tagging xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
							<TagSet>
								<Tag>
									<Key>service</Key>
									<Value>false</Value>
					</Tag>
					<Tag><Key>owner</Key><Value>alice</Value></Tag>
				</TagSet>
							</Tagging>`))
			return
		} else if hreq.URL.Path == "/alice-bucket/" && hreq.Method == http.MethodGet {
			rw.WriteHeader(http.StatusForbidden)
			rw.Write([]byte(`{"Code":"AccessDenied","Message":"Access Denied"}`))
			return
		} else if hreq.URL.Path == "/" && hreq.Method == http.MethodGet {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(listXML))
			return
		} else if strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/info-canned-policy") && hreq.Method == http.MethodGet {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"PolicyName": "testpolicy", "Policy": {"Version": "version","Statement": [{"Resource": ["arn:aws:s3:::alice-bucket/*"]}]}}`))
			return
		} else if hreq.URL.Path == "/minio/admin/v3/update-group-members" && hreq.Method == http.MethodPut {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"Members": {"a"}, "Status":"a","Policy":"a"}`))
			return
		} else {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(`{"status": "success"}`))
			return
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

	router := buildUpdateRouter(cfg)

	body := `{"bucket_name":"alice-bucket","visibility":"restricted","allowed_users":["alice","bob"]}`
	req := httptest.NewRequest(http.MethodPut, "/system/buckets", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}
}

func TestMakeUpdateBucketHandler_RequiresOwnerForPolicyChanges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, visibility, uid, authHeader string
		wantStatus                        int
	}{
		{"public non-owner", "public", "bob", "Bearer token", http.StatusForbidden},
		{"restricted member", "restricted", "bob", "Bearer token", http.StatusForbidden},
		{"untagged non-owner", "public", "bob", "Bearer token", http.StatusForbidden},
		{"public owner", "public", "alice", "Bearer token", http.StatusNoContent},
		{"restricted owner", "restricted", "alice", "Bearer token", http.StatusNoContent},
		{"basic admin", "public", "oscar", "Basic token", http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var writes atomic.Int32
			var adminPolicyChanged atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					writes.Add(1)
					if strings.Contains(r.URL.Path, "canned-policy") && r.URL.Query().Get("name") == "oscar" {
						adminPolicyChanged.Store(true)
					}
				}
				if strings.Contains(r.URL.RawQuery, "location") {
					_, _ = w.Write([]byte(`<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/"></LocationConstraint>`))
					return
				}
				if strings.Contains(r.URL.RawQuery, "tagging") && r.Method == http.MethodGet {
					ownerTag := "<Tag><Key>owner</Key><Value>alice</Value></Tag>"
					if tc.name == "untagged non-owner" {
						ownerTag = ""
					}
					_, _ = w.Write([]byte(`<Tagging xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><TagSet>` + ownerTag + `<Tag><Key>service</Key><Value>false</Value></Tag></TagSet></Tagging>`))
					return
				}
				if strings.HasPrefix(r.URL.Path, "/minio/admin/v3/info-canned-policy") {
					_, _ = w.Write([]byte(`{"PolicyName":"test","Policy":{"Version":"2012-10-17","Statement":[{"Resource":["arn:aws:s3:::alice-bucket/*"]}]}}`))
					return
				}
				_, _ = w.Write([]byte(`{"status":"success"}`))
			}))
			defer server.Close()
			cfg := &types.Config{Name: "oscar", MinIOProvider: &types.MinIOProvider{
				Endpoint: server.URL, Region: "us-east-1", AccessKey: "ak", SecretKey: "sk",
			}}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("uidOrigin", tc.uid)
				c.Set("multitenancyConfig", auth.NewMultitenancyConfig(k8sfake.NewSimpleClientset(), tc.uid))
				c.Next()
			})
			router.PUT("/system/buckets", MakeUpdateHandler(cfg))
			req := httptest.NewRequest(http.MethodPut, "/system/buckets", strings.NewReader(`{"bucket_name":"alice-bucket","visibility":"`+tc.visibility+`","allowed_users":["bob"]}`))
			req.Header.Set("Authorization", tc.authHeader)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tc.wantStatus, res.Code, res.Body.String())
			}
			if tc.wantStatus == http.StatusForbidden && writes.Load() != 0 {
				t.Fatalf("unauthorized request made %d storage writes", writes.Load())
			}
			if tc.wantStatus == http.StatusNoContent && writes.Load() == 0 {
				t.Fatal("authorized request did not update storage")
			}
			if tc.name == "basic admin" && adminPolicyChanged.Load() {
				t.Fatal("admin update changed the admin's policy instead of the tagged owner's")
			}
		})
	}
}

func buildUpdateRouter(cfg *types.Config) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "alice")
		c.Set("multitenancyConfig", auth.NewMultitenancyConfig(k8sfake.NewSimpleClientset(), "alice"))
		c.Next()
	})
	router.PUT("/system/buckets", MakeUpdateHandler(cfg))
	return router
}
