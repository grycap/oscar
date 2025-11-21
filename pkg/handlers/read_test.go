package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestMakeReadHandler(t *testing.T) {
	back := backends.MakeFakeBackend()

	r := gin.Default()
	r.GET("/system/services/:serviceName", MakeReadHandler(back))

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
			r.GET("/system/services/:serviceName", MakeReadHandler(svc))

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
	r.GET("/system/services/:serviceName", MakeReadHandler(back))

	req := httptest.NewRequest(http.MethodGet, "/system/services/missing", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing service, got %d", resp.Code)
	}
}
