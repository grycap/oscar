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
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/grycap/oscar/v4/pkg/testsupport"
	"github.com/grycap/oscar/v4/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

type fakeObjectStorageIAM struct{}

func (fakeObjectStorageIAM) CreateUser(ctx context.Context, accessKey, secretKey string) error {
	return nil
}

func (fakeObjectStorageIAM) CreateGroup(ctx context.Context, group string) error {
	return nil
}

func (fakeObjectStorageIAM) UpdateGroupMembers(ctx context.Context, group string, users []string, remove bool) error {
	return nil
}

func (fakeObjectStorageIAM) GetClient(ctx context.Context) *types.MinIOAdminClient {
	return nil
}

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
		groups: []string{"/group/group1", "/group/group2"},
	}

	ui := &userInfo{
		Subject: "test-subject",
		Groups:  []string{"/group/group1"},
	}

	// Test when the user has the VO
	hasVO := oidcManager.UserHasVO(ui, "/group/group1")
	if !hasVO {
		t.Errorf("expected user to have VO 'group1'")
	}

	// Test when the user does not have the VO
	hasVO = oidcManager.UserHasVO(ui, "/group/group3")
	if hasVO {
		t.Errorf("expected user to not have VO '/group/group3'")
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
	groups := []string{"/group/group1"}

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
		"group_membership": []string{"/group/group1"},
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
		"group_membership": []string{"/group/group2"},
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

/*
func TestGetGroupsKeycloak(t *testing.T) {
	memberships := []string{
		"/group/group1",
		"/group/group2",
	}

	groups := getGroupsKeycloak(memberships)

	if len(groups) != 2 {
		t.Errorf("expected groups length to be 2, got %d", len(groups))
	}

	if groups[0] != "/group/group1" || groups[1] != "/group/group1" {
		t.Errorf("expected groups to be [/group/group1, /group/group1], got %v", groups)
	}
}*/

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
	groups := []string{"group_group1", "group_group2"}

	claims := jwt.MapClaims{
		"iss":              issuer,
		"sub":              subject,
		"exp":              time.Now().Add(1 * time.Hour).Unix(),
		"iat":              time.Now().Unix(),
		"group_membership": []string{"group_group1"},
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

	expectedGroups := []string{"/group/group1"}
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

	// Create base PVC and PV needed by ensureSharedRuntimePVC
	basePVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      types.PVCName,
			Namespace: "oscar-svc",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName:  "test-pv",
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
	basePVC.Status.Phase = corev1.ClaimBound

	basePV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pv",
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			},
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				NFS: &corev1.NFSVolumeSource{
					Server: "nfs.example.com",
					Path:   "/exports",
				},
			},
		},
	}
	kubeClientset.CoreV1().PersistentVolumeClaims("oscar-svc").Create(context.TODO(), basePVC, metav1.CreateOptions{})
	kubeClientset.CoreV1().PersistentVolumes().Create(context.TODO(), basePV, metav1.CreateOptions{})

	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint: server.URL,
			Verify:   false,
		},
		OIDCEnable:       true,
		OIDCSubject:      "123433g",
		OIDCValidIssuers: []string{server.URL},
		OIDCGroups:       []string{"/group/group1", "/group/group2"},
		VolumeAvailable:  "5Gi",
		VolumeMax:        "7Gi",
		VolumeMaxDisk:    "5Gi",
		VolumeMinDisk:    "1Gi",
	}
	issuer := server.URL

	oidcConfig := &oidc.Config{
		InsecureSkipSignatureCheck: true,
		SkipClientIDCheck:          true,
	}
	middleware := getOIDCMiddleware(kubeClientset, fakeObjectStorageIAM{}, &cfg, oidcConfig)
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

func TestOIDCRejectedRequestsDoNotReachProtectedHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	kubeClientset := fake.NewSimpleClientset()
	middleware := getOIDCMiddleware(kubeClientset, fakeObjectStorageIAM{}, &types.Config{}, nil)

	tests := []struct {
		name   string
		header string
		status int
	}{
		{name: "missing bearer token", status: http.StatusUnauthorized},
		{name: "malformed token", header: "Bearer invalid-token", status: http.StatusBadRequest},
		{name: "unlisted issuer", header: "Bearer " + GetToken(jwt.MapClaims{"iss": "https://unlisted.example"}), status: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protectedCalls := 0
			router := gin.New()
			router.Use(middleware)
			router.POST("/system/federation", func(c *gin.Context) {
				protectedCalls++
				c.Status(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodPost, "/system/federation", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Errorf("expected status %d, got %d", tt.status, w.Code)
			}
			if protectedCalls != 0 {
				t.Errorf("protected handler called %d times for rejected request", protectedCalls)
			}
		})
	}
}

func TestOIDCBackendFailuresDoNotReachProtectedHandler(t *testing.T) {
	testsupport.SkipIfCannotListen(t)
	gin.SetMode(gin.TestMode)

	for _, tt := range []struct {
		name         string
		failUserInfo bool
		failSecret   bool
		invalidToken bool
		status       int
	}{
		{name: "invalid token", invalidToken: true, status: http.StatusUnauthorized},
		{name: "userinfo failure", failUserInfo: true, status: http.StatusInternalServerError},
		{name: "secret creation failure", failSecret: true, status: http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var issuer string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch req.URL.Path {
				case "/.well-known/openid-configuration":
					fmt.Fprintf(w, `{"issuer":%q,"userinfo_endpoint":%q}`, issuer, issuer+"/userinfo")
				case "/userinfo":
					if tt.failUserInfo {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					w.Write([]byte(`{"sub":"test-user","group_membership":["test-group"]}`))
				}
			}))
			defer server.Close()
			issuer = server.URL

			client := fake.NewSimpleClientset()
			if tt.failSecret {
				client.PrependReactor("create", "secrets", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("secret creation failed")
				})
			}
			cfg := &types.Config{OIDCValidIssuers: []string{issuer}, OIDCGroups: []string{"test-group"}}
			middleware := getOIDCMiddleware(client, fakeObjectStorageIAM{}, cfg, &oidc.Config{
				InsecureSkipSignatureCheck: true,
				SkipClientIDCheck:          true,
			})
			claims := jwt.MapClaims{"iss": issuer, "sub": "test-user", "exp": time.Now().Add(time.Hour).Unix()}
			if tt.invalidToken {
				claims["exp"] = time.Now().Add(-time.Hour).Unix()
			}
			token := GetToken(claims)
			if tt.failUserInfo {
				// Authentication uses cached groups; the subsequent UserInfo call fails.
				ClusterOidcManagers[issuer].tokenCache[token] = &userInfo{Groups: []string{"test-group"}}
			}

			protectedCalls := 0
			router := gin.New()
			router.Use(middleware)
			router.POST("/system/federation", func(c *gin.Context) {
				protectedCalls++
				c.Status(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodPost, "/system/federation", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Errorf("expected status %d, got %d: %s", tt.status, w.Code, w.Body.String())
			}
			if protectedCalls != 0 {
				t.Errorf("protected handler called %d times for rejected request", protectedCalls)
			}
		})
	}
}

func TestGetUID(t *testing.T) {
	testsupport.SkipIfCannotListen(t)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		if hreq.URL.Path == "/.well-known/openid-configuration" {
			rw.Write([]byte(`{"issuer": "http://` + hreq.Host + `", "userinfo_endpoint": "http://` + hreq.Host + `/userinfo"}`))
		} else if hreq.URL.Path == "/userinfo" {
			rw.Write([]byte(`{"sub": "uid-1234", "group_membership": ["/group/group1"]}`))
		}
	}))
	defer server.Close()

	issuer := server.URL
	oidcManager, err := NewOIDCManager(issuer, "uid-1234", []string{"group1"})
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}
	oidcManager.config.InsecureSkipSignatureCheck = true

	claims := jwt.MapClaims{
		"iss":              issuer,
		"sub":              "uid-1234",
		"exp":              time.Now().Add(1 * time.Hour).Unix(),
		"iat":              time.Now().Unix(),
		"group_membership": []string{"/group/group1"},
	}

	rawToken := GetToken(claims)
	uid, err := oidcManager.GetUID(rawToken)
	if err != nil {
		t.Fatalf("unexpected error getting uid: %v", err)
	}
	if uid != "uid-1234" {
		t.Fatalf("unexpected uid, got %s", uid)
	}
}

func GetToken(claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	privateKey, _ := rsa.GenerateKey(rand.Reader, 1024)
	signedToken, _ := token.SignedString(privateKey)
	return signedToken
}
