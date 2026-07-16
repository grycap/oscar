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
	"github.com/grycap/oscar/v4/pkg/utils/auth"
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
			router.GET("/system/services/:serviceName/auth", MakeServiceAuthHandler())

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

func TestMakeServiceAuthHandlerRedirectsPOSTToExposedService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/system/services/:serviceName/auth", MakeServiceAuthBootstrapHandler())

	tests := []struct {
		name       string
		redirect   string
		wantStatus int
		wantTarget string
	}{
		{name: "defaults to exposed root", wantStatus: http.StatusSeeOther, wantTarget: "/system/services/example/exposed/"},
		{name: "accepts service child", redirect: "/system/services/example/exposed/notebook?tab=1", wantStatus: http.StatusSeeOther, wantTarget: "/system/services/example/exposed/notebook?tab=1"},
		{name: "preserves child trailing slash", redirect: "/system/services/example/exposed/notebook/", wantStatus: http.StatusSeeOther, wantTarget: "/system/services/example/exposed/notebook/"},
		{name: "rejects another service", redirect: "/system/services/other/exposed/", wantStatus: http.StatusBadRequest},
		{name: "rejects absolute URL", redirect: "https://attacker.example/", wantStatus: http.StatusBadRequest},
		{name: "rejects plain traversal", redirect: "/system/services/example/exposed/../../config", wantStatus: http.StatusBadRequest},
		{name: "rejects encoded traversal", redirect: "/system/services/example/exposed/%2e%2e/%2e%2e/config", wantStatus: http.StatusBadRequest},
		{name: "rejects backslash traversal", redirect: `/system/services/example/exposed/..\\..\\config`, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := url.Values{}
			if tt.redirect != "" {
				form.Set("redirect", tt.redirect)
			}
			req := httptest.NewRequest(http.MethodPost, "/system/services/example/auth", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.Code, tt.wantStatus)
			}
			if got := resp.Header().Get("Location"); got != tt.wantTarget {
				t.Fatalf("location = %q, want %q", got, tt.wantTarget)
			}
		})
	}
}

func TestMakeServiceAuthHandlerForwardsOIDCIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/system/services/:serviceName/auth",
		func(c *gin.Context) {
			c.Set("uidOrigin", "student-sub")
			c.Set("userName", "Student\nName")
			c.Set(auth.UserGroupsContextKey, []string{"oscar-students", "pilot"})
			c.Next()
		},
		MakeServiceAuthHandler(),
	)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/services/example/auth", nil)
	router.ServeHTTP(resp, req)

	if got := resp.Header().Get(auth.OscarUserSubHeader); got != "student-sub" {
		t.Fatalf("sub header = %q, want student-sub", got)
	}
	if got := resp.Header().Get(auth.OscarUserNameHeader); got != "StudentName" {
		t.Fatalf("name header = %q, want StudentName", got)
	}
	if got := resp.Header().Get(auth.OscarUserGroupsHeader); got != "oscar-students,pilot" {
		t.Fatalf("groups header = %q, want oscar-students,pilot", got)
	}
}
