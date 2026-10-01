package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/backends"
	"github.com/grycap/oscar/v4/pkg/types"
)

func TestIsBearerRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		authHeader string
		expected   bool
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

func TestIsServiceOwnedByUser(t *testing.T) {
	publicSvc := &types.Service{
		Name:         "public",
		Visibility:   types.PUBLIC,
		Owner:        "owner",
		AllowedUsers: []string{},
	}
	restrictedSvc := &types.Service{
		Name:         "restricted",
		Visibility:   types.RESTRICTED,
		Owner:        "owner",
		AllowedUsers: []string{"user1", "user2"},
	}
	privateSvc := &types.Service{
		Name:         "private",
		Visibility:   types.PRIVATE,
		Owner:        "owner",
		AllowedUsers: []string{},
	}

	tests := []struct {
		name     string
		service  *types.Service
		uid      string
		expected bool
	}{
		{"Nil service", nil, "user", false},
		{"Public service owner", publicSvc, "owner", true},
		{"Public service other user", publicSvc, "anyone", false},
		{"Restricted service owner", restrictedSvc, "owner", true},
		{"Restricted service allowed user", restrictedSvc, "user1", false},
		{"Restricted service not allowed", restrictedSvc, "user3", false},
		{"Private service owner", privateSvc, "owner", true},
		{"Private service other", privateSvc, "other", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isServiceOwnedByUser(tt.service, tt.uid)
			if result != tt.expected {
				t.Errorf("isServiceOwnedByUser(%v, %q) = %v, want %v", tt.service, tt.uid, result, tt.expected)
			}
		})
	}
}

func TestListAuthorizedServicesForMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("No error bearer request", func(t *testing.T) {
		back := backends.MakeFakeBackend()
		back.Services = []*types.Service{
			{Name: "svc1", Visibility: types.PUBLIC, Owner: "owner1"},
			{Name: "svc2", Visibility: types.PRIVATE, Owner: "owner2"},
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

	t.Run("Bearer request only returns owned services", func(t *testing.T) {
		back := backends.MakeFakeBackend()
		back.Services = []*types.Service{
			{Name: "owned", Visibility: types.PRIVATE, Owner: "user1"},
			{Name: "public-other", Visibility: types.PUBLIC, Owner: "owner1"},
			{Name: "restricted-other", Visibility: types.RESTRICTED, Owner: "owner2", AllowedUsers: []string{"user1"}},
		}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/", nil)
		c.Request.Header.Set("Authorization", "Bearer token")
		c.Set("uidOrigin", "user1")

		services, ok := listAuthorizedServicesForMetrics(c, back)
		if !ok {
			t.Fatal("expected ok = true")
		}
		if len(services) != 1 || services[0].Name != "owned" {
			t.Fatalf("expected only the owned service, got %v", services)
		}
	})
}

var errTest = &errTestType{}

type errTestType struct{}

func (e *errTestType) Error() string {
	return "test error"
}
