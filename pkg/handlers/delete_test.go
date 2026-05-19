package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
		if hreq.URL.Path == "/minio/admin/v3/info" {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"Mode": "local", "Region": "us-east-1"}`))
		} else if hreq.URL.Path == "/minio/admin/v3/info-canned-policy" {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{
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
		} else {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"status": "success"}`))
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
					back.AddError("DeleteService", k8serr.NewGone("Not Found"))
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
			rw.Write([]byte(`{"Mode": "local", "Region": "us-east-1"}`))
			return
		}
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte(`{"status": "success"}`))
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
