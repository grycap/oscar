package buckets

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
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

	admin := &stubBucketAdmin{
		Metadata: map[string]string{"service": "true"},
		SetPoliciesFn: func(utils.MinIOBucket) error {
			t.Fatal("SetPolicies must not be invoked for service buckets")
			return nil
		},
		UnsetPoliciesFn: func(utils.MinIOBucket) error {
			t.Fatal("UnsetPolicies must not be invoked for service buckets")
			return nil
		},
	}
	overrideBucketAdminClient(t, admin)

	cfg := &types.Config{
		Name:          "oscar",
		MinIOProvider: &types.MinIOProvider{},
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
}

func TestMakeUpdateBucketHandler_VisibilityChange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var unsetCalls, setCalls int
	var lastVisibility string
	admin := &stubBucketAdmin{
		Metadata:       map[string]string{"service": "false"},
		Visibility:     utils.PRIVATE,
		ResourceAccess: true,
		UnsetPoliciesFn: func(utils.MinIOBucket) error {
			unsetCalls++
			return nil
		},
		SetPoliciesFn: func(bucket utils.MinIOBucket) error {
			setCalls++
			lastVisibility = bucket.Visibility
			return nil
		},
	}
	overrideBucketAdminClient(t, admin)

	cfg := &types.Config{
		Name:          "oscar",
		MinIOProvider: &types.MinIOProvider{},
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
	if unsetCalls != 1 {
		t.Fatalf("expected UnsetPolicies to be called once, got %d", unsetCalls)
	}
	if setCalls != 1 {
		t.Fatalf("expected SetPolicies to be called once, got %d", setCalls)
	}
	if lastVisibility != utils.PUBLIC {
		t.Fatalf("expected SetPolicies to receive visibility %s, got %s", utils.PUBLIC, lastVisibility)
	}
}

func TestMakeUpdateBucketHandler_RestrictedUpdateMembers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var updateCalls int
	var lastUsers []string
	admin := &stubBucketAdmin{
		Metadata:       map[string]string{"service": "false"},
		Visibility:     utils.RESTRICTED,
		ResourceAccess: true,
		UpdateServiceGroupFn: func(_ string, users []string) error {
			updateCalls++
			lastUsers = append([]string(nil), users...)
			return nil
		},
	}
	overrideBucketAdminClient(t, admin)

	cfg := &types.Config{
		Name:          "oscar",
		MinIOProvider: &types.MinIOProvider{},
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
	if updateCalls != 1 {
		t.Fatalf("expected UpdateServiceGroup to be called, got %d", updateCalls)
	}
	if len(lastUsers) != 2 || lastUsers[1] != "bob" {
		t.Fatalf("unexpected users passed to UpdateServiceGroup: %v", lastUsers)
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
