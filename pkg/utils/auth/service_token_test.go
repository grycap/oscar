package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

type serviceTokenMockBackend struct {
	listServicesByNameResult []*types.Service
	listServicesByNameErr    error
	listServicesByNameCalled bool
}

func (m *serviceTokenMockBackend) GetInfo() *types.ServerlessBackendInfo {
	return &types.ServerlessBackendInfo{}
}

func (m *serviceTokenMockBackend) ListServices(namespaces ...string) ([]*types.Service, error) {
	return nil, nil
}

func (m *serviceTokenMockBackend) ListServicesByName(name string, namespaces ...string) ([]*types.Service, error) {
	m.listServicesByNameCalled = true
	return m.listServicesByNameResult, m.listServicesByNameErr
}

func (m *serviceTokenMockBackend) CreateService(service types.Service) error {
	return nil
}

func (m *serviceTokenMockBackend) ReadService(namespace, name string) (*types.Service, error) {
	return nil, nil
}

func (m *serviceTokenMockBackend) UpdateService(service types.Service) error {
	return nil
}

func (m *serviceTokenMockBackend) DeleteService(service types.Service) error {
	return nil
}

func (m *serviceTokenMockBackend) GetKubeClientset() kubernetes.Interface {
	return nil
}

func TestGetServiceTokenMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validToken := strings.Repeat("a", tokenLength)

	// Decision graph paths for GetServiceTokenMiddleware:
	// 1) basic auth -> pass through
	// 2) no bearer token -> pass through
	// 3) bearer token with invalid length -> pass through
	// 4) valid bearer + backend not found -> 404
	// 5) valid bearer + backend error -> 500
	// 6) valid bearer + token match -> set context + pass through
	// 7) valid bearer + no match -> 401
	tests := []struct {
		name                string
		basicAuth           bool
		authHeader          string
		backendServices     []*types.Service
		backendErr          error
		wantLookup          bool
		wantStatus          int
		wantNextHandler     bool
		wantServiceTokenCtx bool
	}{
		{
			name:                "allows request with basic auth",
			basicAuth:           true,
			wantLookup:          false,
			wantStatus:          http.StatusOK,
			wantNextHandler:     true,
			wantServiceTokenCtx: false,
		},
		{
			name:                "allows request without bearer token",
			authHeader:          "",
			wantLookup:          false,
			wantStatus:          http.StatusOK,
			wantNextHandler:     true,
			wantServiceTokenCtx: false,
		},
		{
			name:                "allows request with bearer token of invalid length",
			authHeader:          "Bearer short-token",
			wantLookup:          false,
			wantStatus:          http.StatusOK,
			wantNextHandler:     true,
			wantServiceTokenCtx: false,
		},
		{
			name:                "returns not found when backend returns not found",
			authHeader:          "Bearer " + validToken,
			backendErr:          apierrors.NewNotFound(schema.GroupResource{Resource: "services"}, "svc"),
			wantLookup:          true,
			wantStatus:          http.StatusNotFound,
			wantNextHandler:     false,
			wantServiceTokenCtx: false,
		},
		{
			name:                "returns internal server error on backend failure",
			authHeader:          "Bearer " + validToken,
			backendErr:          errors.New("backend exploded"),
			wantLookup:          true,
			wantStatus:          http.StatusInternalServerError,
			wantNextHandler:     false,
			wantServiceTokenCtx: false,
		},
		{
			name:       "sets service token context and allows when token matches",
			authHeader: "Bearer " + validToken,
			backendServices: []*types.Service{
				{Name: "svc", Token: validToken},
			},
			wantLookup:          true,
			wantStatus:          http.StatusOK,
			wantNextHandler:     true,
			wantServiceTokenCtx: true,
		},
		{
			name:       "returns unauthorized when token does not match",
			authHeader: "Bearer " + validToken,
			backendServices: []*types.Service{
				{Name: "svc", Token: strings.Repeat("b", tokenLength)},
			},
			wantLookup:          true,
			wantStatus:          http.StatusUnauthorized,
			wantNextHandler:     false,
			wantServiceTokenCtx: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			back := &serviceTokenMockBackend{
				listServicesByNameResult: tt.backendServices,
				listServicesByNameErr:    tt.backendErr,
			}

			router := gin.New()
			nextHandlerCalled := false
			serviceTokenInContext := false

			router.GET("/system/services/:serviceName/auth",
				GetServiceTokenMiddleware(back),
				func(c *gin.Context) {
					nextHandlerCalled = true
					if isServiceToken, exists := c.Get(isServiceTokenKey); exists {
						if validServiceToken, ok := isServiceToken.(bool); ok {
							serviceTokenInContext = validServiceToken
						}
					}
					c.Status(http.StatusOK)
				},
			)

			req, err := http.NewRequest(http.MethodGet, "/system/services/svc/auth", nil)
			if err != nil {
				t.Fatalf("unexpected error creating request: %v", err)
			}
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			if tt.basicAuth {
				req.SetBasicAuth("user", "password")
			}

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, resp.Code)
			}
			if nextHandlerCalled != tt.wantNextHandler {
				t.Errorf("expected next handler called = %v, got %v", tt.wantNextHandler, nextHandlerCalled)
			}
			if serviceTokenInContext != tt.wantServiceTokenCtx {
				t.Errorf("expected isServiceToken in context = %v, got %v", tt.wantServiceTokenCtx, serviceTokenInContext)
			}
			if back.listServicesByNameCalled != tt.wantLookup {
				t.Errorf("expected ListServicesByName called = %v, got %v", tt.wantLookup, back.listServicesByNameCalled)
			}
		})
	}
}
