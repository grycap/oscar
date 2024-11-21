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
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/grycap/oscar/v3/pkg/utils"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewOIDCManager(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		if hreq.URL.Path == "/.well-known/openid-configuration" {
			rw.Write([]byte(`{"issuer": "http://` + hreq.Host + `"}`))
		}
	}))

	issuer := server.URL
	subject := "test-subject"
	groups := []string{"group1", "group2"}

	oidcManager, err := NewOIDCManager(issuer, subject, groups)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if oidcManager == nil {
		t.Errorf("expected oidcManager to be non-nil")
	}
}

func TestGetUserInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		fmt.Println(hreq.URL.Path)
		rw.Header().Set("Content-Type", "application/json")
		if hreq.URL.Path == "/.well-known/openid-configuration" {
			rw.Write([]byte(`{"issuer": "http://` + hreq.Host + `", "userinfo_endpoint": "http://` + hreq.Host + `/userinfo"}`))
		} else if hreq.URL.Path == "/userinfo" {
			rw.Write([]byte(`{"sub": "test-subject", "eduperson_entitlement": ["urn:mace:egi.eu:group:group1"]}`))
		}
	}))

	issuer := server.URL
	subject := "test-subject"
	groups := []string{"group1", "group2"}

	oidcManager, err := NewOIDCManager(issuer, subject, groups)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	rawToken := "test-token"
	ui, err := oidcManager.GetUserInfo(rawToken)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if ui.Subject != "test-subject" {
		t.Errorf("expected subject to be %v, got %v", "test-subject", ui.Subject)
	}
	if len(ui.Groups) != 1 || ui.Groups[0] != "group1" {
		t.Errorf("expected groups to be %v, got %v", []string{"group1"}, ui.Groups)
	}
}

func TestIsAuthorised(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		if hreq.URL.Path == "/.well-known/openid-configuration" {
			rw.Write([]byte(`{"issuer": "http://` + hreq.Host + `", "userinfo_endpoint": "http://` + hreq.Host + `/userinfo"}`))
		} else if hreq.URL.Path == "/userinfo" {
			rw.Write([]byte(`{"sub": "test-subject", "eduperson_entitlement": ["urn:mace:egi.eu:group:group1"]}`))
		}
	}))

	issuer := server.URL
	subject := "test-subject"
	groups := []string{"group1", "group2"}

	oidcManager, err := NewOIDCManager(issuer, subject, groups)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	claims := jwt.MapClaims{
		"iss": issuer,
		"sub": subject,
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	privateKey, _ := rsa.GenerateKey(rand.Reader, 1024)
	rawToken, _ := token.SignedString(privateKey)
	oidcManager.config.InsecureSkipSignatureCheck = true

	if !oidcManager.IsAuthorised(rawToken) {
		t.Errorf("expected token to be authorised")
	}
}

func TestGetOIDCMiddleware(t *testing.T) {
	kubeClientset := fake.NewSimpleClientset()
	minIOAdminClient := &utils.MinIOAdminClient{}
	issuer := "https://example.com"
	subject := "test-subject"
	groups := []string{"group1", "group2"}

	middleware := getOIDCMiddleware(kubeClientset, minIOAdminClient, issuer, subject, groups)
	if middleware == nil {
		t.Errorf("expected middleware to be non-nil")
	}

	// Create a new Gin context
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	// Test the middleware with an invalid token
	c.Request = &http.Request{
		Header: http.Header{
			"Authorization": []string{"Bearer invalid-token"},
		},
	}
	middleware(c)
	if c.Writer.Status() != http.StatusUnauthorized {
		t.Errorf("expected status to be %v, got %v", http.StatusUnauthorized, c.Writer.Status())
	}
}
