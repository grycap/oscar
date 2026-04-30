package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
)

func TestIsBearerRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		authHeader string
		expected  bool
	}{
		{"Empty header", "", false},
		{"No Bearer prefix", "Basic token", false},
		{"Bearer prefix", "Bearer token", true},
		{"Bearer with space", "Bearer ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("GET", "/", nil)
			if tt.authHeader != "" {
				c.Request.Header.Set("Authorization", tt.authHeader)
			}

			result := isBearerRequest(c)
			if result != tt.expected {
				t.Errorf("isBearerRequest() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsServiceAccessibleByUser(t *testing.T) {
	publicSvc := &types.Service{
		Name:       "public",
		Visibility:  utils.PUBLIC,
		Owner:      "owner",
		AllowedUsers: []string{},
	}
	restrictedSvc := &types.Service{
		Name:       "restricted",
		Visibility:  utils.RESTRICTED,
		Owner:      "owner",
		AllowedUsers: []string{"user1", "user2"},
	}
	privateSvc := &types.Service{
		Name:       "private",
		Visibility:  utils.PRIVATE,
		Owner:      "owner",
		AllowedUsers: []string{},
	}

	tests := []struct {
		name      string
		service   *types.Service
		uid       string
		expected  bool
	}{
		{"Nil service", nil, "user", false},
		{"Public service any user", publicSvc, "anyone", true},
		{"Public service anonymous", publicSvc, "", true},
		{"Restricted service owner", restrictedSvc, "owner", true},
		{"Restricted service allowed user", restrictedSvc, "user1", true},
		{"Restricted service not allowed", restrictedSvc, "user3", false},
		{"Private service owner", privateSvc, "owner", true},
		{"Private service other", privateSvc, "other", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isServiceAccessibleByUser(tt.service, tt.uid)
			if result != tt.expected {
				t.Errorf("isServiceAccessibleByUser(%v, %q) = %v, want %v", tt.service, tt.uid, result, tt.expected)
			}
		})
	}
}

func TestListAuthorizedServicesForMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("No error bearer request", func(t *testing.T) {
		back := backends.MakeFakeBackend()
		back.Services = []*types.Service{
			{Name: "svc1", Visibility: utils.PUBLIC, Owner: "owner1"},
			{Name: "svc2", Visibility: utils.PRIVATE, Owner: "owner2"},
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/", nil)

		services, ok := listAuthorizedServicesForMetrics(c, back)
		if !ok {
			t.Error("expected ok = true")
		}
		if len(services) != 2 {
			t.Errorf("expected 2 services, got %d", len(services))
		}
	})

	t.Run("Backend error", func(t *testing.T) {
		back := backends.MakeFakeBackend()
		back.AddError("ListServices", errTest)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/", nil)

		services, ok := listAuthorizedServicesForMetrics(c, back)
		if ok {
			t.Error("expected ok = false")
		}
		if services != nil {
			t.Error("expected nil services")
		}
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})
}

var errTest = &errTestType{}

type errTestType struct{}

func (e *errTestType) Error() string {
	return "test error"
}