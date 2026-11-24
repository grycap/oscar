package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/backends"
	"github.com/grycap/oscar/v4/pkg/testsupport"
	"github.com/grycap/oscar/v4/pkg/types"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
)

func TestMakeDeleteHandler(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	back := backends.MakeFakeBackend()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		if hreq.URL.Path != "/input" && hreq.URL.Path != "/output" && !strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/") {
			t.Errorf("Unexpected path in request, got: %s", hreq.URL.Path)
		}
		switch hreq.URL.Path {
		case "/minio/admin/v3/info":
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(`{"Mode": "local", "Region": "us-east-1"}`))
		case "/minio/admin/v3/info-canned-policy":
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(`{
				"PolicyName": "input",
				"Policy": {
					"Version": "2012-10-17",
					"Statement": [
						{
							"Effect": "Allow",
							"Action": ["s3:GetObject"],
							"Resource": ["arn:aws:s3:::example-bucket/*"]
						}
					]
				}
				}`))
		default:
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(`{"status": "success"}`))
		}
	}))

	// and set the MinIO endpoint to the fake server
	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}

	svc := &types.Service{
		Token: "11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf",
		Input: []types.StorageIOConfig{
			{Provider: "minio." + types.DefaultProvider, Path: "/input"},
		},
		Output: []types.StorageIOConfig{
			{Provider: "minio." + types.DefaultProvider, Path: "/output"},
		},
		IsolationLevel: types.IsolationLevelUser,
		AllowedUsers:   []string{"somelonguid1@egi.eu"},
		StorageProviders: &types.StorageProviders{
			MinIO: map[string]*types.MinIOProvider{types.DefaultProvider: {
				Region:    "us-east-1",
				Endpoint:  server.URL,
				AccessKey: "ak",
				SecretKey: "sk"}},
		}}
	back.Service = svc

	r := gin.Default()
	r.DELETE("/system/services/:serviceName", MakeDeleteHandler(&cfg, back))

	scenarios := []struct {
		name        string
		returnError bool
		errType     string
	}{
		{"valid", false, ""},
		{"Service Not Found test", true, "404"},
		{"Internal Server Error test", true, "500"},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			if s.returnError {
				switch s.errType {
				case "404":
					back.AddError("DeleteService", k8serr.NewGone("Not Found")) //nolint:staticcheck // handlers rely on IsGone, which matches StatusReasonGone
				case "500":
					err := errors.New("Not found")
					back.AddError("DeleteService", k8serr.NewInternalError(err))
				}
			}
			serviceName := "testName"
			req, _ := http.NewRequest("DELETE", "/system/services/"+serviceName, nil)

			r.ServeHTTP(w, req)

			if s.returnError {
				if s.errType == "404" && w.Code != http.StatusNotFound {
					t.Errorf("expecting code %d, got %d", http.StatusNotFound, w.Code)
				}

				if s.errType == "500" && w.Code != http.StatusInternalServerError {
					t.Errorf("expecting code %d, got %d", http.StatusInternalServerError, w.Code)
				}
			} else {
				if w.Code != http.StatusNoContent {
					t.Errorf("expecting code %d, got %d", http.StatusNoContent, w.Code)
				}
			}
		})
	}

	// Close the fake MinIO server
	defer server.Close()
}

func TestMakeDeleteHandlerPassesVolumeLifecycleService(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		if !strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/") {
			t.Errorf("Unexpected path in request, got: %s", hreq.URL.Path)
		}
		if hreq.URL.Path == "/minio/admin/v3/info" {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(`{"Mode": "local", "Region": "us-east-1"}`))
			return
		}
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte(`{"status": "success"}`))
	}))
	defer server.Close()

	cfg := &types.Config{
		ServicesNamespace: "oscar-svc",
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "ak",
			SecretKey: "sk",
			Verify:    false,
		},
	}

	tests := []struct {
		name      string
		lifecycle string
	}{
		{name: "retain", lifecycle: types.VolumeLifecycleRetain},
		{name: "delete", lifecycle: types.VolumeLifecycleDelete},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			back := backends.MakeFakeBackend()
			back.Service = &types.Service{
				Name:      "svc",
				Namespace: cfg.ServicesNamespace,
				Owner:     "owner@example.com",
				Volume: &types.ServiceVolumeConfig{
					Name:            "svc",
					Size:            "1Gi",
					MountPath:       "/data",
					LifecyclePolicy: tt.lifecycle,
				},
			}

			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("uidOrigin", "owner@example.com")
				c.Next()
			})
			r.DELETE("/system/services/:serviceName", MakeDeleteHandler(cfg, back))

			req := httptest.NewRequest(http.MethodDelete, "/system/services/svc", nil)
			req.Header.Set("Authorization", "Bearer token")
			resp := httptest.NewRecorder()
			r.ServeHTTP(resp, req)

			if resp.Code != http.StatusNoContent {
				t.Fatalf("expected 204, got %d: %s", resp.Code, resp.Body.String())
			}
			if back.DeletedService == nil || back.DeletedService.Volume == nil {
				t.Fatalf("expected delete handler to pass service volume to backend")
			}
			if back.DeletedService.Volume.LifecyclePolicy != tt.lifecycle {
				t.Fatalf("expected lifecycle %q, got %q", tt.lifecycle, back.DeletedService.Volume.LifecyclePolicy)
			}
		})
	}
}

func TestDeleteBucketsRespectsIsolatedBucketLifecycle(t *testing.T) {
	tests := []struct {
		name            string
		direction       string
		lifecyclePolicy string
		wantDeleted     bool
	}{
		{
			name:            "retain input bucket",
			direction:       "input",
			lifecyclePolicy: types.VolumeLifecycleRetain,
			wantDeleted:     false,
		},
		{
			name:            "delete input bucket",
			direction:       "input",
			lifecyclePolicy: types.VolumeLifecycleDelete,
			wantDeleted:     true,
		},
		{
			name:            "retain output bucket",
			direction:       "output",
			lifecyclePolicy: types.VolumeLifecycleRetain,
			wantDeleted:     false,
		},
		{
			name:            "delete output bucket",
			direction:       "output",
			lifecyclePolicy: types.VolumeLifecycleDelete,
			wantDeleted:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolatedBucket := tt.direction + "-user1234567"
			var isolatedBucketDeleted atomic.Bool
			server := newBucketLifecycleTestServer(t, isolatedBucket, &isolatedBucketDeleted)
			defer server.Close()

			storage := types.StorageIOConfig{
				Provider:        "minio.default",
				Path:            tt.direction + "/data",
				LifecyclePolicy: tt.lifecyclePolicy,
			}
			service := &types.Service{
				Name:           "svc",
				Owner:          types.DefaultOwner,
				IsolationLevel: types.IsolationLevelUser,
				BucketList:     []string{isolatedBucket},
				StorageProviders: &types.StorageProviders{
					MinIO: map[string]*types.MinIOProvider{
						types.DefaultProvider: {
							Endpoint:  server.URL,
							Region:    "us-east-1",
							AccessKey: "ak",
							SecretKey: "sk",
						},
					},
				},
			}
			if tt.direction == "input" {
				service.Input = []types.StorageIOConfig{storage}
			} else {
				service.Output = []types.StorageIOConfig{storage}
			}
			cfg := &types.Config{
				MinIOProvider: &types.MinIOProvider{
					Endpoint:  server.URL,
					Region:    "us-east-1",
					AccessKey: "ak",
					SecretKey: "sk",
				},
			}

			if err := deleteBuckets(service, cfg, &types.MinIOAdminClient{}); err != nil {
				t.Fatalf("deleteBuckets() error = %v", err)
			}
			if got := isolatedBucketDeleted.Load(); got != tt.wantDeleted {
				t.Fatalf("isolated bucket deleted = %v, want %v", got, tt.wantDeleted)
			}
		})
	}
}

func TestDeleteBucketsRespectsNonIsolatedBucketLifecycle(t *testing.T) {
	tests := []struct {
		name            string
		direction       string
		lifecyclePolicy string
		wantDeleted     bool
	}{
		{
			name:            "retain input bucket",
			direction:       "input",
			lifecyclePolicy: types.VolumeLifecycleRetain,
			wantDeleted:     false,
		},
		{
			name:            "delete input bucket",
			direction:       "input",
			lifecyclePolicy: types.VolumeLifecycleDelete,
			wantDeleted:     true,
		},
		{
			name:            "retain output bucket",
			direction:       "output",
			lifecyclePolicy: types.VolumeLifecycleRetain,
			wantDeleted:     false,
		},
		{
			name:            "delete output bucket",
			direction:       "output",
			lifecyclePolicy: types.VolumeLifecycleDelete,
			wantDeleted:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const bucket = "data"
			var bucketDeleted atomic.Bool
			server := newBucketLifecycleTestServer(t, bucket, &bucketDeleted)
			defer server.Close()

			storage := types.StorageIOConfig{
				Provider:        "minio.default",
				Path:            "data/files",
				LifecyclePolicy: tt.lifecyclePolicy,
			}
			service := &types.Service{
				Name:  "svc",
				Owner: types.DefaultOwner,
				StorageProviders: &types.StorageProviders{
					MinIO: map[string]*types.MinIOProvider{
						types.DefaultProvider: {
							Endpoint:  server.URL,
							Region:    "us-east-1",
							AccessKey: "ak",
							SecretKey: "sk",
						},
					},
				},
			}
			if tt.direction == "input" {
				service.Input = []types.StorageIOConfig{storage}
			} else {
				service.Output = []types.StorageIOConfig{storage}
			}
			cfg := &types.Config{
				MinIOProvider: &types.MinIOProvider{
					Endpoint:  server.URL,
					Region:    "us-east-1",
					AccessKey: "ak",
					SecretKey: "sk",
				},
			}

			if err := deleteBuckets(service, cfg, &types.MinIOAdminClient{}); err != nil {
				t.Fatalf("deleteBuckets() error = %v", err)
			}
			if got := bucketDeleted.Load(); got != tt.wantDeleted {
				t.Fatalf("bucket deleted = %v, want %v", got, tt.wantDeleted)
			}
		})
	}
}

func newBucketLifecycleTestServer(t *testing.T, isolatedBucket string, isolatedBucketDeleted *atomic.Bool) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/"+isolatedBucket:
			isolatedBucketDeleted.Store(true)
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/minio/admin/v3/info-canned-policy":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"PolicyName":"policy",
				"Policy":{
					"Version":"2012-10-17",
					"Statement":[{
						"Effect":"Allow",
						"Action":["s3:GetObject"],
						"Resource":["arn:aws:s3:::existing/*"]
					}]
				}
			}`))
		case r.Method == http.MethodGet && r.URL.Query().Has("notification"):
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<NotificationConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"/>`))
		case r.Method == http.MethodGet && !strings.HasPrefix(r.URL.Path, "/minio/admin/"):
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><IsTruncated>false</IsTruncated></ListBucketResult>`))
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success"}`))
		}
	}))
}

func TestMakeDeleteHandlerForbidsVisibleServicesForNonOwners(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		visibility   string
		allowedUsers []string
	}{
		{
			name:       "public service",
			visibility: types.PUBLIC,
		},
		{
			name:         "restricted service with allowed user",
			visibility:   types.RESTRICTED,
			allowedUsers: []string{"user-b@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			back := backends.MakeFakeBackend()
			back.Service = &types.Service{
				Name:         "svc",
				Owner:        "user-a@example.com",
				Visibility:   tt.visibility,
				AllowedUsers: tt.allowedUsers,
			}

			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("uidOrigin", "user-b@example.com")
				c.Next()
			})
			r.DELETE("/system/services/:serviceName", MakeDeleteHandler(&types.Config{}, back))

			req := httptest.NewRequest(http.MethodDelete, "/system/services/svc", nil)
			req.Header.Set("Authorization", "Bearer token")
			resp := httptest.NewRecorder()
			r.ServeHTTP(resp, req)

			if resp.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", resp.Code, resp.Body.String())
			}
			if back.DeletedService != nil {
				t.Fatalf("expected service not to be deleted by non-owner")
			}
		})
	}
}
