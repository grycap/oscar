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
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/testsupport"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestMakeCreateHandler(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	back := backends.MakeFakeBackend()
	kubeClientset := testclient.NewSimpleClientset()

	// Create a fake MinIO server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		fmt.Printf("Received request: %s %s\n", hreq.Method, hreq.URL.String())
		if hreq.URL.Path != "/test" && hreq.URL.Path != "/" && hreq.URL.Path != "/test/" && hreq.URL.Path != "/test/input/" && hreq.URL.Path != "/test/output/" && hreq.URL.Path != "/test/mount/" && !strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/") && !strings.HasPrefix(hreq.URL.Path, "/test-somelongui") {
			t.Errorf("Unexpected path in request, got: %s", hreq.URL.Path)
		}

		if hreq.URL.Path == "/minio/admin/v3/info" {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"Mode": "local", "Region": "us-east-1"}`))
		} else if strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/info-canned-policy") {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{
				"Version": "2012-10-17",
				"Statement": [
					{
						"Effect": "Allow",
						"Action": [
							"s3:*"
						],
						"Resource": [
							"arn:aws:s3:::test/*",
						]
					}
				]
			}`))
		} else if strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/set-user-or-group-policy") {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"status":"success","binding":"done"}`))
		} else if hreq.Method == http.MethodGet && strings.HasPrefix(hreq.URL.Path, "/test") && hreq.URL.RawQuery == "location=" {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/"/>`))
		} else {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"status": "success"}`))
		}
	}))

	// and set the MinIO endpoint to the fake server
	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			Verify:    false,
		},
	}

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "somelonguid@egi.eu")
		c.Set("multitenancyConfig", auth.NewMultitenancyConfig(kubeClientset, "somelonguid@egi.eu"))
		c.Next()
	})
	r.POST("/system/services", MakeCreateHandler(&cfg, back))

	scenarios := []struct {
		name           string
		visibility     string
		allowedUsers   []string
		expectedStatus int
	}{
		{"PublicVisibility", "public", []string{}, http.StatusCreated},
		{"InvalidVisibility", "private", []string{}, http.StatusCreated},
		{"EmptyVisibility", "", []string{}, http.StatusCreated}, // Assuming default is allowed
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			allowedUsersJSON := "["
			for i, user := range s.allowedUsers {
				if i > 0 {
					allowedUsersJSON += ","
				}
				allowedUsersJSON += `"` + user + `"`
			}
			allowedUsersJSON += "]"

			body := strings.NewReader(`
				{
					"name": "cowsay",
					"cluster_id": "oscar",
					"memory": "1Gi",
					"cpu": "1.0",
					"log_level": "CRITICAL",
					"image": "ghcr.io/grycap/cowsay",
					"alpine": false,
					"script": "test",
					"input": [
					],
					"output": [
					],
					"mount": {
						"storage_provider": "minio",
						"path": "test/mount"
					},
					"storage_providers": {
						"webdav": {
							"id": {
								"hostname": "` + server.URL + `",
								"login": "user",
								"password": "pass"
							}
						}
					},
					"isolation_level": "",
					"bucket_list": [],
					"visibility": "` + s.visibility + `",
					"allowed_users": []
				}`)

			req, _ := http.NewRequest("POST", "/system/services", body)
			req.Header.Add("Authorization", "Bearer token")
			r.ServeHTTP(w, req)

			if w.Code != s.expectedStatus {
				fmt.Println("response: ", w.Body)
				t.Errorf("expecting code %d, got %d", s.expectedStatus, w.Code)
			}
		})
	}

	// Close the fake MinIO server
	defer server.Close()
}

func TestMakeCreateHandlerWebhookError(t *testing.T) {
	testsupport.SkipIfCannotListen(t)
	back := backends.MakeFakeBackend()
	kubeClientset := testclient.NewSimpleClientset()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		// Simulate MinIO info ok, but webhook restart error
		if hreq.URL.Path == "/minio/admin/v3/info" {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"Mode": "local", "Region": "us-east-1"}`))
			return
		}
		if strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/service") {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte(`{"Status": "success"}`))
	}))
	defer server.Close()

	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "ak",
			SecretKey: "sk",
			Verify:    false,
		},
	}

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "owner@example.com")
		c.Set("multitenancyConfig", auth.NewMultitenancyConfig(kubeClientset, "owner@example.com"))
		c.Next()
	})
	r.POST("/system/services", MakeCreateHandler(&cfg, back))

	body := strings.NewReader(`{"name":"svc","image":"img","script":"echo","mount":{"storage_provider":"minio","path":"test/mount"},"visibility":"public"}`)
	req := httptest.NewRequest(http.MethodPost, "/system/services", body)
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on webhook failure, got %d", resp.Code)
	}
}

func TestCheckValuesDefaults(t *testing.T) {
	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint: "http://minio:9000",
			Region:   "us-east-1",
			Verify:   false,
		},
	}

	service := types.Service{
		Name:        "demo",
		LogLevel:    "invalid",
		Labels:      nil,
		Annotations: map[string]string{},
	}

	checkValues(&service, &cfg)

	if service.Memory != defaultMemory {
		t.Fatalf("expected default memory %s, got %s", defaultMemory, service.Memory)
	}
	if service.CPU != defaultCPU {
		t.Fatalf("expected default cpu %s, got %s", defaultCPU, service.CPU)
	}
	if service.Visibility != utils.PRIVATE {
		t.Fatalf("expected visibility %s, got %s", utils.PRIVATE, service.Visibility)
	}
	if service.LogLevel != defaultLogLevel {
		t.Fatalf("expected log level %s, got %s", defaultLogLevel, service.LogLevel)
	}
	if service.Labels[types.ServiceLabel] != service.Name {
		t.Fatalf("expected service label to be set for %s", service.Name)
	}
	if service.StorageProviders == nil || service.StorageProviders.MinIO == nil {
		t.Fatalf("expected default MinIO provider to be created")
	}
	if service.StorageProviders.MinIO[types.DefaultProvider].AccessKey != "hidden" {
		t.Fatalf("expected hidden credentials to be used")
	}
	if service.Token == "" {
		t.Fatalf("expected token to be generated")
	}
}

func TestGetProviderInfo(t *testing.T) {
	provID, provName := getProviderInfo("minio.custom")
	if provName != types.MinIOName || provID != "custom" {
		t.Fatalf("unexpected provider info: %s %s", provName, provID)
	}

	provID, provName = getProviderInfo("rucio")
	if provName != types.RucioName || provID != types.DefaultProvider {
		t.Fatalf("expected default provider id, got %s %s", provName, provID)
	}
}

func TestIsStorageProviderDefined(t *testing.T) {
	providers := &types.StorageProviders{
		MinIO:   map[string]*types.MinIOProvider{"custom": {}},
		S3:      map[string]*types.S3Provider{"s3": {}},
		Onedata: map[string]*types.OnedataProvider{"op": {}},
		WebDav:  map[string]*types.WebDavProvider{"wd": {}},
		Rucio:   map[string]*types.Rucio{"ru": {}},
	}
	tests := []struct {
		name     string
		id       string
		expected bool
	}{
		{types.MinIOName, "custom", true},
		{types.S3Name, "s3", true},
		{types.OnedataName, "op", true},
		{types.WebDavName, "wd", true},
		{types.RucioName, "ru", true},
		{types.MinIOName, "missing", false},
	}

	for _, tt := range tests {
		if isStorageProviderDefined(tt.name, tt.id, providers) != tt.expected {
			t.Fatalf("unexpected result for %s.%s", tt.name, tt.id)
		}
	}
}

func TestMakeCreateHandlerInvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	cfg := &types.Config{MinIOProvider: &types.MinIOProvider{}}

	r := gin.New()
	r.POST("/system/services", MakeCreateHandler(cfg, back))

	req := httptest.NewRequest(http.MethodPost, "/system/services", strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d", resp.Code)
	}
}

func TestCheckIdentity(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 1024)
	jwk := buildRSAJWK(&priv.PublicKey)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			w.Write([]byte(`{"issuer":"` + server.URL + `","userinfo_endpoint":"` + server.URL + `/userinfo","jwks_uri":"` + server.URL + `/keys"}`))
		case "/userinfo":
			w.Write([]byte(`{"sub":"user@example.com","group_membership":["/group/test-vo"]}`))
		case "/keys":
			w.Write([]byte(jwk))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	manager, err := auth.NewOIDCManager(server.URL, "user@example.com", []string{"test-vo"})
	if err != nil {
		t.Fatalf("unexpected error creating oidc manager: %v", err)
	}
	auth.ClusterOidcManagers[server.URL] = manager

	claims := jwt.MapClaims{
		"iss":              server.URL,
		"sub":              "user@example.com",
		"exp":              time.Now().Add(1 * time.Hour).Unix(),
		"iat":              time.Now().Unix(),
		"group_membership": []string{"/group/test-vo"},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	rawToken, _ := token.SignedString(priv)

	service := &types.Service{
		VO:     "test-vo",
		Labels: map[string]string{},
	}

	if err := checkIdentity(service, "Bearer "+rawToken); err != nil {
		t.Fatalf("expected identity check to pass, got %v", err)
	}
	if service.Labels["vo"] != service.VO {
		t.Fatalf("expected service labels to include vo %q, got %q", service.VO, service.Labels["vo"])
	}
}

func buildRSAJWK(pub *rsa.PublicKey) string {
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	return `{"keys":[{"kty":"RSA","alg":"RS256","use":"sig","kid":"test","n":"` + n + `","e":"` + e + `"}]}`
}
