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

package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/types"
)

func TestMakeServiceAuthHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Decision graph paths for this handler:
	// 1) Request reaches the handler -> return HTTP 200.
	tests := []struct {
		name       string
		targetPath string
		wantStatus int
	}{
		{
			name:       "returns 200 when request reaches auth handler",
			targetPath: "/system/services/example/auth",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/system/services/:serviceName/auth", MakeServiceAuthHandler(&types.Config{ExposedServicesUseSubdomainRoute: false}))

			req, err := http.NewRequest(http.MethodGet, tt.targetPath, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestMakeServiceAuthHandlerRedirectsFormBootstrap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/system/services/:serviceName/auth", MakeServiceAuthHandler(&types.Config{ExposedServicesUseSubdomainRoute: false}))

	form := url.Values{
		"token":     {"header.payload.signature"},
		"return_to": {"/system/services/oscar-viva/exposed/student?from=dashboard#section"},
	}
	request := httptest.NewRequest(http.MethodPost, "/system/services/oscar-viva/auth", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", "Bearer header.payload.signature")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusSeeOther)
	}
	if got := response.Header().Get("Location"); got != form.Get("return_to") {
		t.Fatalf("redirect = %q, want %q", got, form.Get("return_to"))
	}
	if len(response.Result().Cookies()) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(response.Result().Cookies()))
	}
}

func TestMakeServiceAuthHandlerRejectsExternalReturnPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/system/services/:serviceName/auth", MakeServiceAuthHandler(&types.Config{ExposedServicesUseSubdomainRoute: false}))

	form := url.Values{"return_to": {"https://attacker.example/"}}
	request := httptest.NewRequest(http.MethodPost, "/system/services/oscar-viva/auth", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", "Bearer header.payload.signature")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatal("invalid return path must not set a cookie")
	}
}

func TestMakeServiceAuthHandlerSetsOIDCCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/system/services/:serviceName/auth", MakeServiceAuthHandler(&types.Config{ExposedServicesUseSubdomainRoute: false}))

	request := httptest.NewRequest(http.MethodGet, "/system/services/oscar-viva/auth", nil)
	request.Header.Set("Authorization", "Bearer header.payload.signature")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}
	if cookies[0].Name != "oscar_service_oscar_viva_auth" {
		t.Fatalf("cookie name = %q, want oscar_service_oscar_viva_auth", cookies[0].Name)
	}
}
