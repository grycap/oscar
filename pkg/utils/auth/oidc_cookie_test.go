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
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetOIDCServiceAuthFormMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotHeader string
	router := gin.New()
	router.POST("/system/services/:serviceName/auth",
		GetOIDCServiceAuthFormMiddleware(),
		GetOIDCServiceAuthCookieMiddleware(),
		func(c *gin.Context) {
			gotHeader = c.GetHeader("Authorization")
			c.Status(http.StatusOK)
		},
	)

	form := url.Values{"token": {"form.payload.signature"}}
	request := httptest.NewRequest(http.MethodPost, "/system/services/svc/auth", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: getServiceAuthCookieName("svc"), Value: "stale.payload.signature"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if gotHeader != "Bearer form.payload.signature" {
		t.Fatalf("Authorization header = %q", gotHeader)
	}
}

func TestGetOIDCServiceAuthFormMiddlewareRejectsInvalidRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
	}{
		{name: "wrong content type", contentType: "application/json", body: `{}`, wantStatus: http.StatusUnsupportedMediaType},
		{name: "malformed token", contentType: "application/x-www-form-urlencoded", body: "token=invalid", wantStatus: http.StatusBadRequest},
		{name: "oversized form", contentType: "application/x-www-form-urlencoded", body: "token=" + strings.Repeat("a", serviceAuthFormMaxBytes), wantStatus: http.StatusRequestEntityTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/system/services/:serviceName/auth", GetOIDCServiceAuthFormMiddleware(), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})
			request := httptest.NewRequest(http.MethodPost, "/system/services/svc/auth", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", tt.contentType)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
		})
	}
}

func TestGetOIDCServiceAuthCookieMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		header     string
		cookie     string
		wantHeader string
	}{
		{
			name:       "restores JWT from service auth cookie",
			cookie:     "header.payload.signature",
			wantHeader: "Bearer header.payload.signature",
		},
		{
			name:       "preserves explicit Bearer credential",
			header:     "Bearer explicit.payload.signature",
			cookie:     "cookie.payload.signature",
			wantHeader: "Bearer explicit.payload.signature",
		},
		{
			name:   "ignores service token cookie",
			cookie: strings.Repeat("a", tokenLength),
		},
		{
			name:   "ignores malformed JWT cookie",
			cookie: "not-a-jwt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotHeader string
			router := gin.New()
			router.GET("/system/services/:serviceName/auth",
				GetOIDCServiceAuthCookieMiddleware(),
				func(c *gin.Context) {
					gotHeader = c.GetHeader("Authorization")
					c.Status(http.StatusOK)
				},
			)

			request := httptest.NewRequest(http.MethodGet, "/system/services/svc/auth", nil)
			if tt.header != "" {
				request.Header.Set("Authorization", tt.header)
			}
			if tt.cookie != "" {
				request.AddCookie(&http.Cookie{Name: getServiceAuthCookieName("svc"), Value: tt.cookie})
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if gotHeader != tt.wantHeader {
				t.Fatalf("Authorization header = %q, want %q", gotHeader, tt.wantHeader)
			}
		})
	}
}

func TestOIDCCookieTakesPrecedenceOverApplicationQueryToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	backend := &serviceTokenMockBackend{}
	jwt := "header.payload.signature"

	router := gin.New()
	router.GET("/system/services/:serviceName/auth",
		GetOIDCServiceAuthCookieMiddleware(),
		GetServiceTokenMiddleware(backend),
		func(c *gin.Context) {
			if got := c.GetHeader("Authorization"); got != "Bearer "+jwt {
				t.Fatalf("Authorization header = %q, want Bearer JWT", got)
			}
			c.Status(http.StatusOK)
		},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/system/services/svc/auth?token="+strings.Repeat("a", tokenLength),
		nil,
	)
	request.AddCookie(&http.Cookie{Name: getServiceAuthCookieName("svc"), Value: jwt})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if backend.listServicesByNameCalled {
		t.Fatal("service-token lookup should not run when an OIDC cookie is present")
	}
}

func TestSetOIDCServiceAuthCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		token      string
		wantCookie bool
	}{
		{name: "sets cookie for JWT", token: "header.payload.signature", wantCookie: true},
		{name: "ignores service token", token: strings.Repeat("a", tokenLength)},
		{name: "ignores malformed token", token: "not-a-jwt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/system/services/:serviceName/auth", func(c *gin.Context) {
				SetOIDCServiceAuthCookie(c)
				c.Status(http.StatusOK)
			})

			request := httptest.NewRequest(http.MethodGet, "/system/services/my-svc/auth", nil)
			request.Header.Set("Authorization", "Bearer "+tt.token)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			cookies := response.Result().Cookies()

			if !tt.wantCookie {
				if len(cookies) != 0 {
					t.Fatalf("unexpected cookies: %v", cookies)
				}
				return
			}

			if len(cookies) != 1 {
				t.Fatalf("cookie count = %d, want 1", len(cookies))
			}
			cookie := cookies[0]
			if cookie.Name != "oscar_service_my_svc_auth" || cookie.Value != tt.token {
				t.Fatalf("unexpected cookie identity: %s=%q", cookie.Name, cookie.Value)
			}
			if cookie.Path != "/system/services/my-svc/exposed" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
				t.Fatalf("unexpected cookie attributes: %+v", cookie)
			}
		})
	}
}
