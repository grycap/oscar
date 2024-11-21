/*
Copyright (C) GRyCAP - I3M - UPV

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetAuthMiddleware(t *testing.T) {
	cfg := &types.Config{
		OIDCEnable: false,
		Username:   "testuser",
		Password:   "testpass",
	}
	kubeClientset := fake.NewSimpleClientset()

	router := gin.New()
	router.Use(GetAuthMiddleware(cfg, kubeClientset))
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, "")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.SetBasicAuth("testuser", "testpass")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %v, got %v", http.StatusOK, w.Code)
	}

	we := httptest.NewRecorder()
	reqe, _ := http.NewRequest("GET", "/", nil)
	reqe.SetBasicAuth("testuser", "otherpass")
	router.ServeHTTP(we, reqe)

	if we.Code != http.StatusUnauthorized {
		t.Errorf("expected status %v, got %v", http.StatusUnauthorized, we.Code)
	}
}

func TestGetLoggerMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(GetLoggerMiddleware())
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, "")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %v, got %v", http.StatusOK, w.Code)
	}
}

func TestGetUIDFromContext(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("uidOrigin", "testuid")

	uid, err := GetUIDFromContext(c)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if uid != "testuid" {
		t.Errorf("expected uid %v, got %v", "testuid", uid)
	}
}

func TestGetMultitenancyConfigFromContext(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	mc := &MultitenancyConfig{}
	c.Set("multitenancyConfig", mc)

	mcFromContext, err := GetMultitenancyConfigFromContext(c)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mcFromContext != mc {
		t.Errorf("expected multitenancyConfig %v, got %v", mc, mcFromContext)
	}
}
