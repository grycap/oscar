package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/types"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
)

var (
	testMinIOProviderRun types.MinIOProvider = types.MinIOProvider{
		Endpoint:  "http://minio.minio:30300",
		Verify:    true,
		AccessKey: "minio",
		SecretKey: "ZjhhMWZk",
		Region:    "us-east-1",
	}

	testConfigValidRun types.Config = types.Config{
		MinIOProvider:        &testMinIOProviderRun,
		WatchdogMaxInflight:  20,
		WatchdogWriteDebug:   true,
		WatchdogExecTimeout:  60,
		WatchdogReadTimeout:  60,
		WatchdogWriteTimeout: 60,
	}
)

type GinResponseRecorder struct {
	http.ResponseWriter
}

func (GinResponseRecorder) CloseNotify() <-chan bool {
	return nil
}

func (GinResponseRecorder) Flush() {
}

func TestMakeRunHandler(t *testing.T) {

	scenarios := []struct {
		name        string
		returnError bool
		errType     string
	}{
		{"Valid service test", false, ""},
		{"Service Not Found test", true, "404"},
		{"Internal Server Error test", true, "500"},
		{"Bad token: split token", true, "splitErr"},
		{"Bad token: diff service token", true, "diffErr"},
	}
	for _, s := range scenarios {
		back := backends.MakeFakeSyncBackend()
		http.DefaultClient.Timeout = 400 * time.Second
		r := gin.Default()
		r.POST("/run/:serviceName", MakeRunHandler(&testConfigValidRun, back))

		t.Run(s.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			serviceName := "testName"

			req, _ := http.NewRequest("POST", "/run/"+serviceName, nil)
			req.Header.Set("Authorization", "Bearer 11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf")

			if s.returnError {
				switch s.errType {
				case "404":
					back.AddError("ReadService", k8serr.NewGone("Not Found"))
				case "500":
					err := errors.New("Not found")
					back.AddError("ReadService", k8serr.NewInternalError(err))
				case "splitErr":
					req.Header.Set("Authorization", "11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf")
				case "diffErr":
					req.Header.Set("Authorization", "Bearer 11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513dfg")
				}
			}

			r.ServeHTTP(GinResponseRecorder{w}, req)
			// Sleep time to avoid HTTP error 503 on first request
			time.Sleep(1 * time.Second)
			if s.returnError {

				if s.errType == "splitErr" || s.errType == "diffErr" {
					if w.Code != http.StatusUnauthorized {
						t.Errorf("expecting code %d, got %d", http.StatusUnauthorized, w.Code)
					}
				}

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
