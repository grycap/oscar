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
	"reflect"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/grycap/oscar/v3/pkg/testsupport"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewOIDCManager(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

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

func TestUserHasVO(t *testing.T) {
	oidcManager := &oidcManager{
		groups: []string{"group1", "group2"},
	}

	ui := &userInfo{
		Subject: "test-subject",
		Groups:  []string{"group1"},
	}

	// Test when the user has the VO
	hasVO := oidcManager.UserHasVO(ui, "group1")
	if !hasVO {
		t.Errorf("expected user to have VO 'group1'")
	}

	// Test when the user does not have the VO
	hasVO = oidcManager.UserHasVO(ui, "group3")
	if hasVO {
		t.Errorf("expected user to not have VO 'group3'")
	}
}

func TestIsAuthorised(t *testing.T) {
	testsupport.SkipIfCannotListen(t)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		if hreq.URL.Path == "/.well-known/openid-configuration" {
			rw.Write([]byte(`{"issuer": "http://` + hreq.Host + `", "userinfo_endpoint": "http://` + hreq.Host + `/userinfo"}`))
		} else if hreq.URL.Path == "/userinfo" {
			rw.Write([]byte(`{"sub": "123433g", "group_membership": ["/group/group1"]}`))
		}
	}))

	issuer := server.URL
	subject := "123433g"
	groups := []string{"group1"}

	oidcManager, err := NewOIDCManager(issuer, subject, groups)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	oidcManager.config.InsecureSkipSignatureCheck = true

	claims1 := jwt.MapClaims{
		"iss":              issuer,
		"sub":              subject,
		"exp":              time.Now().Add(1 * time.Hour).Unix(),
		"iat":              time.Now().Unix(),
		"group_membership": []string{"group1"},
	}

	token1 := GetToken(claims1)
	fmt.Println(token1)
	// Test when the token is authorised
	if !oidcManager.IsAuthorised(token1) {
		t.Errorf("expected token1 to be authorised")
	}
	claims2 := jwt.MapClaims{
		"iss":              "asdfas2123",
		"sub":              subject,
		"exp":              time.Now().Add(1 * time.Hour).Unix(),
		"iat":              time.Now().Unix(),
		"group_membership": []string{"group2"},
	}
	// Test when the token is not authorised
	token2 := GetToken(claims2)
	fmt.Println(token2)
	if oidcManager.IsAuthorised(token2) {
		t.Errorf("expected token2 to not be authorised")
	}
}

func TestGetGroupsEGI(t *testing.T) {
	urns := []string{
		"urn:mace:egi.eu:group:group1",
		"urn:mace:egi.eu:group:group2",
	}

	groups := getGroupsEGI(urns)

	if len(groups) != 2 {
		t.Errorf("expected groups length to be 2, got %d", len(groups))
	}

	if groups[0] != "group1" || groups[1] != "group2" {
		t.Errorf("expected groups to be [group1, group2], got %v", groups)
	}
}

func TestGetGroupsKeycloak(t *testing.T) {
	memberships := []string{
		"/group/group1",
		"/group/group2",
	}

	groups := getGroupsKeycloak(memberships)

	if len(groups) != 2 {
		t.Errorf("expected groups length to be 2, got %d", len(groups))
	}

	if groups[0] != "group1" || groups[1] != "group2" {
		t.Errorf("expected groups to be [group1, group2], got %v", groups)
	}
}

func TestGetIssuerFromToken(t *testing.T) {
	claims := jwt.MapClaims{
		"iss":                   "http://example.com",
		"sub":                   "test-subject",
		"exp":                   time.Now().Add(1 * time.Hour).Unix(),
		"iat":                   time.Now().Unix(),
		"eduperson_entitlement": []string{"/group/group1"},
	}
	rawToken := GetToken(claims)

	issuer, err := GetIssuerFromToken(rawToken)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if issuer != "http://example.com" {
		t.Errorf("expected issuer to be http://example.com, got %v", issuer)
	}
}

func TestGetUserInfo(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		if hreq.URL.Path == "/.well-known/openid-configuration" {
			rw.Write([]byte(`{"issuer": "http://` + hreq.Host + `", "userinfo_endpoint": "http://` + hreq.Host + `/userinfo"}`))
		} else if hreq.URL.Path == "/userinfo" {
			rw.Write([]byte(`{"sub": "user1@egi.eu", "group_membership": ["/group/group1"]}`))
		}
	}))

	issuer := server.URL
	subject := "test-subject"
	groups := []string{"group1", "group2"}

	claims := jwt.MapClaims{
		"iss":              issuer,
		"sub":              subject,
		"exp":              time.Now().Add(1 * time.Hour).Unix(),
		"iat":              time.Now().Unix(),
		"group_membership": []string{"/group/group1"},
	}

	oidcManager, err := NewOIDCManager(issuer, subject, groups)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	rawToken := GetToken(claims)
	ui, err := oidcManager.GetUserInfo(rawToken)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	expectedGroups := []string{"group1"}
	if !reflect.DeepEqual(ui.Groups, expectedGroups) {
		t.Errorf("expected Groups to be %v, got %v", expectedGroups, ui.Groups)
	}
}

func TestGetOIDCMiddleware(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		if hreq.URL.Path == "/.well-known/openid-configuration" {
			rw.Write([]byte(`{"issuer": "http://` + hreq.Host + `", "userinfo_endpoint": "http://` + hreq.Host + `/userinfo"}`))
		} else if hreq.URL.Path == "/userinfo" {
			rw.Write([]byte(`{"sub": "123433g", "group_membership": ["/group/group1"]}`))
		} else if hreq.URL.Path == "/minio/admin/v3/info" {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"Mode": "local", "Region": "us-east-1"}`))
		} else {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"status": "success"}`))
		}
	}))

	kubeClientset := fake.NewSimpleClientset()
	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint: server.URL,
			Verify:   false,
		},
		OIDCEnable:       true,
		OIDCSubject:      "123433g",
		OIDCValidIssuers: []string{server.URL},
		OIDCGroups:       []string{"group1", "group2"},
	}
	minIOAdminClient, _ := utils.MakeMinIOAdminClient(&cfg)
	issuer := server.URL

	oidcConfig := &oidc.Config{
		InsecureSkipSignatureCheck: true,
		SkipClientIDCheck:          true,
	}
	middleware := getOIDCMiddleware(kubeClientset, minIOAdminClient, &cfg, oidcConfig)
	if middleware == nil {
		t.Errorf("expected middleware to be non-nil")
	}
	validClaims := jwt.MapClaims{
		"iss":                   issuer,
		"sub":                   cfg.OIDCSubject,
		"exp":                   time.Now().Add(1 * time.Hour).Unix(),
		"iat":                   time.Now().Unix(),
		"eduperson_entitlement": []string{"/group/group1"},
	}

	scenarios := []struct {
		token string
		code  int
		name  string
	}{
		{
			name:  "invalid-token",
			token: "invalid-token",
			code:  http.StatusBadRequest,
		},
		{
			name:  "valid-token",
			token: GetToken(validClaims),
			code:  http.StatusOK,
		},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			// Create a new Gin context
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Test the middleware with an invalid token
			c.Request = &http.Request{
				Header: http.Header{
					"Authorization": []string{"Bearer " + s.token},
				},
			}
			middleware(c)
			if c.Writer.Status() != s.code {
				t.Errorf("expected status to be %v, got %v", s.code, c.Writer.Status())
			}
		})
	}
}

func GetToken(claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	privateKey, _ := rsa.GenerateKey(rand.Reader, 1024)
	signedToken, _ := token.SignedString(privateKey)
	return signedToken
}
