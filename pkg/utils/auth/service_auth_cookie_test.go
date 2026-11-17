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
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/types"
)

func TestGetServiceAuthCookieScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &types.Config{IngressHost: "oscar.example.com", OIDCEnable: true}

	tests := []struct {
		name                string
		isServiceScopedName bool
		isPathBased         bool
		wantName            string
		wantPath            string
		wantDomain          string
	}{
		{
			name:                "OIDC cookie in subdomain routing uses global name and parent domain",
			isServiceScopedName: false,
			isPathBased:         false,
			wantName:            GLOBAL_SERVICE_AUTH_COOKIE_NAME,
			wantPath:            "/",
			wantDomain:          ".oscar.example.com",
		},
		{
			name:                "service token cookie in subdomain routing uses service name and parent domain",
			isServiceScopedName: true,
			isPathBased:         false,
			wantName:            getServiceAuthCookieName("my-svc"),
			wantPath:            "/",
			wantDomain:          ".oscar.example.com",
		},
		{
			name:                "path-based routing scopes the cookie to the service path",
			isServiceScopedName: true,
			isPathBased:         true,
			wantName:            getServiceAuthCookieName("my-svc"),
			wantPath:            getServiceAuthCookiePath("my-svc"),
			wantDomain:          "",
		},
		{
			name:                "path-based routing forces the service-scoped name",
			isServiceScopedName: false,
			isPathBased:         true,
			wantName:            getServiceAuthCookieName("my-svc"),
			wantPath:            getServiceAuthCookiePath("my-svc"),
			wantDomain:          "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := getServiceAuthCookieScope("my-svc", tt.isServiceScopedName, tt.isPathBased, cfg)
			if scope.name != tt.wantName || scope.path != tt.wantPath || scope.domain != tt.wantDomain {
				t.Errorf("scope = (%s, %q, %q), want (%s, %q, %q)",
					scope.name, scope.path, scope.domain, tt.wantName, tt.wantPath, tt.wantDomain)
			}
		})
	}
}

// issueOIDCServiceAuthCookie drives the public OIDC issuer call path
// (MakeServiceAuthHandler -> SetOIDCServiceAuthCookie) and returns the cookie
// it stores in a browser.
func issueOIDCServiceAuthCookie(t *testing.T, cfg *types.Config, forwardedProto string) *http.Cookie {
	t.Helper()

	router := gin.New()
	router.GET("/system/services/:serviceName/auth", func(c *gin.Context) {
		SetOIDCServiceAuthCookie(c, cfg)
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodGet, "/system/services/my-svc/auth", nil)
	request.Header.Set("Authorization", "Bearer header.payload.signature")
	if forwardedProto != "" {
		request.Header.Set("X-Forwarded-Proto", forwardedProto)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("issued cookie count = %d, want 1", len(cookies))
	}
	return cookies[0]
}

// expireOIDCServiceAuthCookie drives the public stale-cookie expiry call path
// (GetOIDCServiceAuthCookieMiddleware -> setExpiredServiceCookie) with the
// browser sending back a stale credential under the issued cookie name.
func expireOIDCServiceAuthCookie(t *testing.T, cfg *types.Config, staleCookieName, forwardedProto string) *http.Cookie {
	t.Helper()

	router := gin.New()
	router.GET("/system/services/:serviceName/auth",
		GetOIDCServiceAuthCookieMiddleware(cfg),
		func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/system/services/my-svc/auth", nil)
	request.AddCookie(&http.Cookie{Name: staleCookieName, Value: "stale.payload.signature"})
	if forwardedProto != "" {
		request.Header.Set("X-Forwarded-Proto", forwardedProto)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expired cookie count = %d, want 1", len(cookies))
	}
	return cookies[0]
}

func TestSetExpiredServiceCookieMatchesIssuedScope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		cfg            *types.Config
		forwardedProto string
		wantSecure     bool
		wantName       string
		wantPath       string
		wantDomain     string
	}{
		// Note: wantDomain holds the value as re-parsed from the Set-Cookie
		// response; the RFC 6265 parser strips the leading dot of the Domain
		// attribute while keeping the cookie domain-scoped.
		{
			name:           "path-based routing over https",
			cfg:            &types.Config{OIDCEnable: true},
			forwardedProto: "https",
			wantSecure:     true,
			wantName:       "oscar_service_my_svc_auth",
			wantPath:       "/system/services/my-svc/exposed",
			wantDomain:     "",
		},
		{
			name:       "path-based routing over plain http",
			cfg:        &types.Config{OIDCEnable: true},
			wantSecure: false,
			wantName:   "oscar_service_my_svc_auth",
			wantPath:   "/system/services/my-svc/exposed",
			wantDomain: "",
		},
		{
			name:           "subdomain routing over https",
			cfg:            &types.Config{IngressHost: "oscar.example.com", ExposedServicesUseSubdomainRoute: true, OIDCEnable: true},
			forwardedProto: "https",
			wantSecure:     true,
			wantName:       GLOBAL_SERVICE_AUTH_COOKIE_NAME,
			wantPath:       "/",
			wantDomain:     "oscar.example.com",
		},
		{
			name:       "subdomain routing over plain http",
			cfg:        &types.Config{IngressHost: "oscar.example.com", ExposedServicesUseSubdomainRoute: true, OIDCEnable: true},
			wantSecure: false,
			wantName:   GLOBAL_SERVICE_AUTH_COOKIE_NAME,
			wantPath:   "/",
			wantDomain: "oscar.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issued := issueOIDCServiceAuthCookie(t, tt.cfg, tt.forwardedProto)
			expired := expireOIDCServiceAuthCookie(t, tt.cfg, issued.Name, tt.forwardedProto)

			// The expiry must address the cookie exactly as it was issued:
			// RFC 6265 cookie identity is the name, domain and path triple.
			if expired.Name != issued.Name || expired.Path != issued.Path || expired.Domain != issued.Domain {
				t.Errorf("expired cookie scope = (%s, %q, %q), want issued scope (%s, %q, %q)",
					expired.Name, expired.Path, expired.Domain, issued.Name, issued.Path, issued.Domain)
			}
			if expired.Secure != issued.Secure {
				t.Errorf("expired cookie Secure = %v, want %v", expired.Secure, issued.Secure)
			}
			if !expired.HttpOnly || expired.SameSite != http.SameSiteLaxMode {
				t.Errorf("expired cookie security flags = HttpOnly:%v SameSite:%v, want HttpOnly:true SameSite:Lax",
					expired.HttpOnly, expired.SameSite)
			}
			if expired.MaxAge >= 0 {
				t.Errorf("expired cookie Max-Age = %d, want negative", expired.MaxAge)
			}
			if expired.Value != "" {
				t.Errorf("expired cookie value = %q, want empty", expired.Value)
			}

			// Pin the issued scope itself as a regression guard.
			if issued.Name != tt.wantName || issued.Path != tt.wantPath || issued.Domain != tt.wantDomain || issued.Secure != tt.wantSecure {
				t.Errorf("issued cookie = (%s, %q, %q, Secure:%v), want (%s, %q, %q, Secure:%v)",
					issued.Name, issued.Path, issued.Domain, issued.Secure, tt.wantName, tt.wantPath, tt.wantDomain, tt.wantSecure)
			}
		})
	}
}

func TestSetExpiredServiceCookieSecureOnTLSConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/system/services/:serviceName/auth",
		GetOIDCServiceAuthCookieMiddleware(&types.Config{OIDCEnable: true}),
		func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/system/services/my-svc/auth", nil)
	request.TLS = &tls.ConnectionState{} // simulate an HTTPS connection
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expired cookie count = %d, want 1", len(cookies))
	}
	if !cookies[0].Secure {
		t.Errorf("expiry over a TLS connection must set the Secure attribute")
	}
}

// TestSetExpiredServiceCookieClearsIssuedScopeInCookieJar applies the issued
// and the expired Set-Cookie headers to the standard library's RFC 6265 cookie
// storage model and verifies that the expiry removes the stored credential
// from its original scope. This is a jar-level check of the Set-Cookie
// attributes, not a live-browser test.
func TestSetExpiredServiceCookieClearsIssuedScopeInCookieJar(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		cfg       *types.Config
		issueURL  string
		expireURL string
		checkURL  string
	}{
		{
			name:      "path-based routing",
			cfg:       &types.Config{OIDCEnable: true},
			issueURL:  "https://oscar.example.com/system/services/my-svc/auth",
			expireURL: "https://oscar.example.com/system/services/my-svc/auth",
			checkURL:  "https://oscar.example.com/system/services/my-svc/exposed/app.js",
		},
		{
			name:      "subdomain routing",
			cfg:       &types.Config{IngressHost: "oscar.example.com", ExposedServicesUseSubdomainRoute: true, OIDCEnable: true},
			issueURL:  "https://api.oscar.example.com/system/services/my-svc/auth",
			expireURL: "https://api.oscar.example.com/system/services/my-svc/auth",
			checkURL:  "https://my-svc.oscar.example.com/some/app/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issued := issueOIDCServiceAuthCookie(t, tt.cfg, "https")
			expired := expireOIDCServiceAuthCookie(t, tt.cfg, issued.Name, "https")

			issueURL, err := url.Parse(tt.issueURL)
			if err != nil {
				t.Fatalf("unexpected error parsing issue URL: %v", err)
			}
			expireURL, err := url.Parse(tt.expireURL)
			if err != nil {
				t.Fatalf("unexpected error parsing expire URL: %v", err)
			}
			checkURL, err := url.Parse(tt.checkURL)
			if err != nil {
				t.Fatalf("unexpected error parsing check URL: %v", err)
			}

			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatalf("unexpected error creating cookie jar: %v", err)
			}

			jar.SetCookies(issueURL, []*http.Cookie{issued})
			if stored := jar.Cookies(checkURL); len(stored) != 1 || stored[0].Value != "header.payload.signature" {
				t.Fatalf("issued cookie not stored in jar: %v", stored)
			}

			jar.SetCookies(expireURL, []*http.Cookie{expired})
			if stored := jar.Cookies(checkURL); len(stored) != 0 {
				t.Fatalf("stale credential survived the scoped expiry: %v", stored)
			}
		})
	}
}
