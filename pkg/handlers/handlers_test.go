package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v2/pkg/backends"
	"github.com/grycap/oscar/v2/pkg/types"
)

//Variables for valid testing
var (
	testServiceValid types.Service = types.Service{
		Name:             "testname",
		Image:            "testimage",
		Alpine:           false,
		Memory:           "1Gi",
		CPU:              "1.0",
		ImagePullSecrets: []string{"testcred1", "testcred2"},
		Script:           "testscript",
		Environment: struct {
			Vars map[string]string `json:"Variables"`
		}{
			Vars: map[string]string{
				"TEST_VAR": "testvalue",
			},
		},
		Annotations: map[string]string{
			"testannotation": "testannotationvalue",
		},
		Labels: map[string]string{
			"testlabel": "testlabelvalue",
		},
		StorageProviders: &types.StorageProviders{
			MinIO: map[string]*types.MinIOProvider{
				types.DefaultProvider: {
					Endpoint:  "https://minio-service.minio:9000",
					Verify:    true,
					AccessKey: "testaccesskey",
					SecretKey: "testsecretkey",
					Region:    "us-east-1",
				},
			},
		},
	}

	testMinIOProviderValid types.MinIOProvider = types.MinIOProvider{
		Endpoint:  "https://minio-service.minio:9000",
		Verify:    true,
		AccessKey: "testaccesskey",
		SecretKey: "testsecretkey",
		Region:    "us-east-1",
	}

	testConfigValid types.Config = types.Config{
		MinIOProvider:        &testMinIOProviderValid,
		WatchdogMaxInflight:  20,
		WatchdogWriteDebug:   true,
		WatchdogExecTimeout:  60,
		WatchdogReadTimeout:  60,
		WatchdogWriteTimeout: 60,
	}
)

//Variables for invalid testing

var (
	testServiceInvalid types.Service = types.Service{
		Image:            "testimage",
		Alpine:           false,
		Memory:           "1Gi",
		CPU:              "1.0",
		ImagePullSecrets: []string{"testcred1", "testcred2"},
		Script:           "testscript",
		Environment: struct {
			Vars map[string]string `json:"Variables"`
		}{
			Vars: map[string]string{
				"TEST_VAR": "testvalue",
			},
		},
		Annotations: map[string]string{
			"testannotation": "testannotationvalue",
		},
		Labels: map[string]string{
			"testlabel": "testlabelvalue",
		},
		StorageProviders: &types.StorageProviders{
			MinIO: map[string]*types.MinIOProvider{
				types.DefaultProvider: {
					Endpoint:  "https://minio-service.minio:9000",
					Verify:    true,
					AccessKey: "testaccesskey",
					SecretKey: "testsecretkey",
					Region:    "us-east-1",
				},
			},
		},
	}
	testMinIOProviderInvalid types.MinIOProvider = types.MinIOProvider{
		Endpoint:  "https://minio-service.minio:9000",
		Verify:    true,
		AccessKey: "test",
		SecretKey: "test",
		Region:    "us-east-1",
	}
	testConfigInvalid types.Config = types.Config{
		MinIOProvider:        &testMinIOProviderInvalid,
		WatchdogMaxInflight:  20,
		WatchdogWriteDebug:   true,
		WatchdogExecTimeout:  60,
		WatchdogReadTimeout:  60,
		WatchdogWriteTimeout: 60,
	}
)

func TestCreateHandler(t *testing.T) {
	back := backends.MakeFakeBackend()
	r := gin.Default()
	r.POST("/system/service", MakeCreateHandler(&testConfigValid, back))

	t.Run("", func(t *testing.T) {
		w := httptest.NewRecorder()

		// Make valid request
		reqBody, _ := json.Marshal(&testServiceValid)
		req, _ := http.NewRequest("POST", "/system/service", strings.NewReader(string(reqBody)))
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("> Failed test http.StatusOK")
			t.Logf("Status code: %v", w.Code)
			t.Logf("Body: %v", w.Body)
		}

		//Make invalid request with invalid service
		badReqBody, _ := json.Marshal(&testServiceInvalid)
		badReq, _ := http.NewRequest("POST", "/system/service", strings.NewReader(string(badReqBody)))
		r.ServeHTTP(w, badReq)
		if w.Code != http.StatusBadRequest {
			t.Errorf("> Failed test http.StatusBadRequest w/ invalid service")
			t.Logf("Status code: %v", w.Code)
			t.Logf("Body: %v", w.Body)
		}
	})

	/* 	r.POST("/system/service", MakeCreateHandler(&testConfigInvalid, back))
	   	t.Run("", func(t *testing.T) {
	   		w := httptest.NewRecorder()
	   		reqBody, _ := json.Marshal(&testServiceValid)
	   		req, _ := http.NewRequest("POST", "/system/service", strings.NewReader(string(reqBody)))
	   		r.ServeHTTP(w, req)
	   		if w.Code != http.StatusBadRequest {
	   			t.Errorf("> Failed test http.StatusBadRequest w/ invalid Config")
	   			t.Logf("Status code: %v", w.Code)
	   			t.Logf("Body: %v", w.Body)
	   		}
	   	}) */

}

func TestDeleteServiceHandler(t *testing.T) {
	back := backends.MakeFakeBackend()
	r := gin.Default()
	r.GET("/system/service/delete", MakeDeleteHandler(&testConfigInvalid, back))
	t.Run("Delete null service test", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/system/service/delete", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("> Failed delete null service test http.StatusBadRequest")
			t.Logf("Status code: %v", w.Code)
			t.Logf("Body: %v", w.Body)
		}
	})
	t.Run("Delete test service test", func(t *testing.T) {
		back.CreateService(testServiceValid)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/system/service/delete", strings.NewReader(""))
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("> Failed delete null service test http.StatusBadRequest")
			t.Logf("Status code: %v", w.Code)
			t.Logf("Body: %v", w.Body)
		}
	})
}

func TestListHandler(t *testing.T) {
	back := backends.MakeFakeBackend()

	r := gin.Default()
	r.GET("/system/services", MakeListHandler(back))

	scenarios := []struct {
		name        string
		returnError bool
	}{
		{"valid", false},
		{"invalid", true},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			if s.returnError {
				back.AddError("ListServices", errors.New("test error"))
			}

			req, _ := http.NewRequest("GET", "/system/services", nil)

			r.ServeHTTP(w, req)

			if s.returnError {
				if w.Code != http.StatusInternalServerError {
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
