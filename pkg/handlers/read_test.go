package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestMakeReadHandler(t *testing.T) {
	back := backends.MakeFakeBackend()

	r := gin.Default()
	r.GET("/system/services/:serviceName", MakeReadHandler(back, back.GetKubeClientset(), &types.Config{}))

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
					back.AddError("ReadService", k8serr.NewGone("Not Found"))
				case "500":
					err := errors.New("Not found")
					back.AddError("ReadService", k8serr.NewInternalError(err))
				}
			}
			serviceName := "testName"
			req, _ := http.NewRequest("GET", "/system/services/"+serviceName, nil)

			r.ServeHTTP(w, req)

			if s.returnError {
				if s.errType == "404" && w.Code != http.StatusNotFound {
					t.Errorf("expecting code %d, got %d", http.StatusNotFound, w.Code)
				}

				if s.errType == "500" && w.Code != http.StatusInternalServerError {
					t.Errorf("expecting code %d, got %d", http.StatusInternalServerError, w.Code)
				}
			} else {
				if w.Code != http.StatusOK {
					t.Errorf("expecting code %d, got %d", http.StatusOK, w.Code)
				}
			}
		})
	}
}

func TestMakeReadHandlerVisibility(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name       string
		visibility string
		uid        string
		status     int
	}{
		{"public_with_bearer", utils.PUBLIC, "any", http.StatusOK},
		{"private_owner", utils.PRIVATE, "owner", http.StatusOK},
		{"restricted_allowed", utils.RESTRICTED, "friend", http.StatusOK},
		{"no_token_defaults", utils.RESTRICTED, "", http.StatusOK},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			svc := backends.MakeFakeBackend()
			svc.Service = &types.Service{
				Name:         "svc",
				Owner:        "owner",
				AllowedUsers: []string{"friend"},
				Visibility:   tt.visibility,
			}

			r := gin.New()
			r.Use(func(c *gin.Context) {
				if tt.uid != "" {
					c.Set("uidOrigin", tt.uid)
					c.Request.Header.Set("Authorization", "Bearer token")
				}
				c.Next()
			})
			r.GET("/system/services/:serviceName", MakeReadHandler(svc, svc.GetKubeClientset(), &types.Config{}))

			req := httptest.NewRequest(http.MethodGet, "/system/services/svc", nil)
			resp := httptest.NewRecorder()
			r.ServeHTTP(resp, req)

			if resp.Code != tt.status {
				t.Fatalf("expected status %d, got %d", tt.status, resp.Code)
			}
		})
	}
}

func TestMakeReadHandlerNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	back.AddError("ReadService", k8serr.NewNotFound(schema.GroupResource{Group: "test", Resource: "services"}, "missing"))

	r := gin.New()
	r.GET("/system/services/:serviceName", MakeReadHandler(back, back.GetKubeClientset(), &types.Config{}))

	req := httptest.NewRequest(http.MethodGet, "/system/services/missing", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing service, got %d", resp.Code)
	}
}

func TestMakeReadHandlerVolumeStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "default",
		Volume: &types.ServiceVolumeConfig{
			Size:      "1Gi",
			MountPath: "/data",
		},
	}
	_, _ = back.GetKubeClientset().CoreV1().PersistentVolumeClaims("default").Create(t.Context(), &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc",
			Namespace: "default",
			Labels: map[string]string{
				types.ManagedVolumeLabel:     "true",
				types.ManagedVolumeNameLabel: "svc",
			},
		},
	}, metav1.CreateOptions{})

	r := gin.New()
	r.GET("/system/services/:serviceName", MakeReadHandler(back, back.GetKubeClientset(), &types.Config{}))

	req := httptest.NewRequest(http.MethodGet, "/system/services/svc", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var got types.Service
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if !got.VolumeStatus.Enabled || got.VolumeStatus.Name != "svc" {
		t.Fatalf("expected volume status to be enabled with resolved name")
	}
}

func TestMakeReadHandlerIncludeDeploymentSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:      "svc",
		Namespace: "default",
		Expose: types.Expose{
			APIPort: 8080,
		},
	}

	replicas := int32(2)
	_, _ = back.GetKubeClientset().AppsV1().Deployments("default").Create(t.Context(), &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-dpl",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          2,
			AvailableReplicas: 1,
		},
	}, metav1.CreateOptions{})

	r := gin.New()
	r.GET("/system/services/:serviceName", MakeReadHandler(back, back.GetKubeClientset(), &types.Config{}))

	req := httptest.NewRequest(http.MethodGet, "/system/services/svc?include=deployment", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var got types.Service
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if got.Deployment == nil {
		t.Fatalf("expected deployment summary to be included")
	}
	if got.Deployment.State != types.DeploymentStateDegraded {
		t.Fatalf("expected degraded deployment state, got %s", got.Deployment.State)
	}
}

func TestMakeReadHandlerInvalidServiceName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()

	r := gin.New()
	r.GET("/system/services/:serviceName", MakeReadHandler(back, back.GetKubeClientset(), &types.Config{}))

	req := httptest.NewRequest(http.MethodGet, "/system/services/svc}", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid service name, got %d", resp.Code)
	}
}
