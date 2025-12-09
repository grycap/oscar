package buckets

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/testsupport"
	"github.com/grycap/oscar/v3/pkg/types"
)

type bucketRequestRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *bucketRequestRecorder) add(call string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
}

func (r *bucketRequestRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func TestMakeCreateBucketHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		body        string
		headers     map[string]string
		wantStatus  int
		setup       func(t *testing.T, cfg *types.Config) (func(), *bucketRequestRecorder)
		assertCalls func(t *testing.T, recorder *bucketRequestRecorder)
	}{
		{
			name:       "invalid json payload",
			body:       "not-json",
			headers:    map[string]string{"Content-Type": "application/json"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "admin creates bucket",
			body: `{"bucket_name":"test-bucket","visibility":"private"}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			wantStatus: http.StatusCreated,
			setup: func(t *testing.T, cfg *types.Config) (func(), *bucketRequestRecorder) {
				testsupport.SkipIfCannotListen(t)
				recorder := &bucketRequestRecorder{}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					recorder.add(r.Method + " " + r.Host + r.URL.RequestURI())
					switch {
					case r.Method == http.MethodGet && r.URL.Path == "/minio/admin/v3/info":
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"Mode":"mode","Region":"us-east-1"}`))
					case r.Method == http.MethodGet && strings.Contains(r.URL.RawQuery, "location"):
						w.Header().Set("Content-Type", "application/xml")
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">us-east-1</LocationConstraint>`))
					case r.Method == http.MethodPut && strings.Contains(r.URL.RawQuery, "tagging"):
						w.WriteHeader(http.StatusOK)
					case strings.HasPrefix(r.URL.Path, "/minio/admin/v3/"):
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"status":"success"}`))
					default:
						w.WriteHeader(http.StatusOK)
					}
				}))

				endpoint := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
				cfg.MinIOProvider.Endpoint = endpoint
				cfg.MinIOProvider.Verify = false

				cleanup := func() {
					server.Close()
				}
				return cleanup, recorder
			},
			assertCalls: func(t *testing.T, recorder *bucketRequestRecorder) {
				calls := recorder.snapshot()
				var sawCreate, sawTagging bool
				for _, call := range calls {
					if !strings.HasPrefix(call, "PUT ") {
						continue
					}
					if strings.Contains(call, "?tagging") {
						sawTagging = true
						continue
					}
					if strings.Contains(call, "/test-bucket") {
						sawCreate = true
					}
				}
				if !sawCreate {
					t.Errorf("expected bucket creation request, got %v", calls)
				}
				if !sawTagging {
					t.Errorf("expected bucket tagging request, got %v", calls)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &types.Config{
				Name:        "oscar",
				Namespace:   "oscar",
				ServicePort: 8080,
				MinIOProvider: &types.MinIOProvider{
					Endpoint:  "http://127.0.0.1:9000",
					Region:    "us-east-1",
					AccessKey: "minioadmin",
					SecretKey: "minioadmin",
					Verify:    false,
				},
			}

			var (
				cleanup  func()
				recorder *bucketRequestRecorder
			)
			if tt.setup != nil {
				cleanup, recorder = tt.setup(t, cfg)
			}
			if cleanup != nil {
				defer cleanup()
			}

			router := gin.New()
			router.POST("/system/buckets", MakeCreateHandler(cfg, nil))

			req, err := http.NewRequest(http.MethodPost, "/system/buckets", strings.NewReader(tt.body))
			if err != nil {
				t.Fatalf("failed to build request: %v", err)
			}
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)

			if tt.assertCalls != nil {
				tt := tt
				tt.assertCalls(t, recorder)
			}

			if res.Code != tt.wantStatus {
				t.Logf("response body: %s", res.Body.String())
				t.Fatalf("expected status %d, got %d", tt.wantStatus, res.Code)
			}
		})
	}
}

func TestMakeCreateBucketHandlerMissingUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &types.Config{
		Name:        "oscar",
		Namespace:   "oscar",
		ServicePort: 8080,
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  "http://127.0.0.1:9000",
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}

	router := gin.New()
	router.POST("/system/buckets", func(c *gin.Context) {
		c.Set("uidOrigin", "")
	MakeCreateHandler(cfg, nil)(c)
	})

	body := `{"bucket_path":"user-bucket"}`
	req, err := http.NewRequest(http.MethodPost, "/system/buckets", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}
