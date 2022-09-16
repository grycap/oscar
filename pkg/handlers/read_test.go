package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v2/pkg/backends"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
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
