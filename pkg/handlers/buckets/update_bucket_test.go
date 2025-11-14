package buckets

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
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
