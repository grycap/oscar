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

func TestGetForwardAuthBootstrapMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtToken := strings.Repeat("a", 32) + "." + strings.Repeat("b", 32) + "." + strings.Repeat("c", 32)
	tests := []struct {
		name          string
		authHeader    string
		targetPath    string
		forwardedURI  string
		cookieToken   string
		formToken     string
		wantBearer    string
		wantSetCookie bool
	}{
		{name: "accepts bearer header", authHeader: "Bearer " + jwtToken, wantBearer: jwtToken, wantSetCookie: true},
		{name: "accepts query bearer", targetPath: "/system/services/svc/auth?token=" + jwtToken, wantBearer: jwtToken, wantSetCookie: true},
		{name: "accepts form bearer", formToken: jwtToken, wantBearer: jwtToken, wantSetCookie: true},
		{name: "accepts forwarded URI bearer", forwardedURI: "/system/services/svc/exposed/?token=" + jwtToken, wantBearer: jwtToken, wantSetCookie: true},
		{name: "accepts service auth cookie bearer", cookieToken: jwtToken, wantBearer: jwtToken, wantSetCookie: true},
		{name: "ignores non-JWT application token", targetPath: "/system/services/svc/auth?token=application-session-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			gotBearer := ""
			router.Any("/system/services/:serviceName/auth",
				GetForwardAuthBootstrapMiddleware(),
				func(c *gin.Context) {
					gotBearer, _ = isAuthBearer(c)
					SetForwardAuthCookie(c)
					c.Status(http.StatusOK)
				},
			)

			targetPath := tt.targetPath
			if targetPath == "" {
				targetPath = "/system/services/svc/auth"
			}
			method := http.MethodGet
			body := strings.NewReader("")
			if tt.formToken != "" {
				method = http.MethodPost
				body = strings.NewReader(url.Values{"token": []string{tt.formToken}}.Encode())
			}
			req := httptest.NewRequest(method, targetPath, body)
			if tt.formToken != "" {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			if tt.forwardedURI != "" {
				req.Header.Set("X-Forwarded-Uri", tt.forwardedURI)
			}
			if tt.cookieToken != "" {
				req.AddCookie(&http.Cookie{Name: getServiceAuthCookieName("svc"), Value: tt.cookieToken})
			}

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if gotBearer != tt.wantBearer {
				t.Fatalf("bearer = %q, want %q", gotBearer, tt.wantBearer)
			}
			setCookie := resp.Header().Get("Set-Cookie")
			if tt.wantSetCookie && !strings.Contains(setCookie, getServiceAuthCookieName("svc")+"=") {
				t.Fatalf("expected service auth cookie, got %q", setCookie)
			}
			if !tt.wantSetCookie && setCookie != "" {
				t.Fatalf("unexpected service auth cookie %q", setCookie)
			}
		})
	}
}

func TestForwardAuthCookieIsNotSetBeforeAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jwtToken := strings.Repeat("a", 32) + "." + strings.Repeat("b", 32) + "." + strings.Repeat("c", 32)

	router := gin.New()
	router.GET("/system/services/:serviceName/auth",
		GetForwardAuthBootstrapMiddleware(),
		func(c *gin.Context) {
			c.AbortWithStatus(http.StatusForbidden)
		},
		func(c *gin.Context) {
			SetForwardAuthCookie(c)
			c.Status(http.StatusOK)
		},
	)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/system/services/svc/auth?token="+jwtToken, nil)
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusForbidden)
	}
	if cookie := resp.Header().Get("Set-Cookie"); cookie != "" {
		t.Fatalf("unexpected auth cookie on denied request: %q", cookie)
	}
}
