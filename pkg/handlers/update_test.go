package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/backends"
	"github.com/grycap/oscar/v4/pkg/testsupport"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestCreateAndUpdateRejectUnsafeMountBeforeMutation(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		for _, options := range []string{"--rc", "--rc-addr=0.0.0.0:5572", "--config /tmp/config", "--buffer-size --rc"} {
			t.Run(method+"/"+options, func(t *testing.T) {
				back := backends.MakeFakeBackend()
				r := gin.New()
				cfg := &types.Config{MinIOProvider: &types.MinIOProvider{}}
				if method == http.MethodPost {
					r.POST("/system/services", MakeCreateHandler(cfg, back))
				} else {
					r.PUT("/system/services", MakeUpdateHandler(cfg, back))
				}
				body := fmt.Sprintf(`{"name":"svc","image":"img","script":"echo test","mount":{"storage_provider":"minio.default","path":"notebook","options":%q}}`, options)
				resp := httptest.NewRecorder()
				r.ServeHTTP(resp, httptest.NewRequest(method, "/system/services", strings.NewReader(body)))
				if resp.Code != http.StatusBadRequest || !strings.Contains(resp.Body.String(), "mount option") {
					t.Fatalf("expected mount option 400, got %d: %s", resp.Code, resp.Body.String())
				}
				if back.Service != nil || back.UpdatedService != nil {
					t.Fatal("invalid options reached backend")
				}
			})
		}
	}
}

func TestValidateServiceMountOptions(t *testing.T) {
	for _, tc := range []struct {
		provider string
		options  string
		valid    bool
	}{
		{"", "", true},
		{"minio.default", "", true},
		{"minio.default", "--buffer-size 128M --vfs-read-chunk-size 64M", true},
		{"webdav.default", "--timeout=30s", true},
		{"minio.default", "--rc", false},
		{"", "--read-only", false},
	} {
		if err := validateServiceMountOptions(&types.Service{Mount: types.StorageIOConfig{Provider: tc.provider, Options: tc.options}}); (err == nil) != tc.valid {
			t.Errorf("provider %q options %q: err %v; valid %t", tc.provider, tc.options, err, tc.valid)
		}
	}
}

func TestMakeUpdateHandlerRejectsForgedOwnerBeforeSecretCopy(t *testing.T) {
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{Name: "svc", Owner: "owner", Namespace: "owner-ns"}
	client := testclient.NewSimpleClientset()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "owner")
		c.Set("multitenancyConfig", auth.NewMultitenancyConfig(client, "owner"))
		c.Next()
	})
	r.PUT("/system/services", MakeUpdateHandler(&types.Config{MinIOProvider: &types.MinIOProvider{}}, back))
	req := httptest.NewRequest(http.MethodPut, "/system/services", strings.NewReader(`{"name":"svc","owner":"victim","image":"img","script":"echo"}`))
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", resp.Code, resp.Body.String())
	}
	for _, action := range client.Actions() {
		if action.GetResource().Resource == "secrets" {
			t.Fatalf("foreign owner triggered secret action %s", action.GetVerb())
		}
	}
	if back.UpdatedService != nil {
		t.Fatal("forged owner updated service")
	}
}

func TestMakeUpdateHandler(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	back := backends.MakeFakeBackend()
	kubeClientset := testclient.NewSimpleClientset()
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		if hreq.URL.Path != "/input" && hreq.URL.Path != "/output" && !strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/") {
			t.Errorf("Unexpected path in request, got: %s", hreq.URL.Path)
		}
		if hreq.URL.Path == "/minio/admin/v3/info" {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"Mode": "local", "Region": "us-east-1"}`))
		} else {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"status": "success"}`))
		}
	}))

	svc := &types.Service{
		Token: "11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf",
		CPU:   "2.0",
		StorageProviders: &types.StorageProviders{
			MinIO: map[string]*types.MinIOProvider{types.DefaultProvider: {
				Region:    "us-east-1",
				Endpoint:  server.URL,
				AccessKey: "ak",
				SecretKey: "sk"}},
		},
		Owner:        "somelonguid@egi.eu",
		AllowedUsers: []string{}}
	back.Service = svc

	// and set the MinIO endpoint to the fake server
	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Region:    "us-east-1",
			Endpoint:  server.URL,
			AccessKey: "ak",
			SecretKey: "sk",
		},
	}

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "somelonguid@egi.eu")
		c.Set("multitenancyConfig", auth.NewMultitenancyConfig(kubeClientset, "somelonguid@egi.eu"))
		c.Next()
	})
	r.PUT("/system/services", MakeUpdateHandler(&cfg, back))

	w := httptest.NewRecorder()
	body := strings.NewReader(`
		{
			"name": "cowsay",
			"cluster_id": "oscar",
			"memory": "1Gi",
			"cpu": "1.0",
			"log_level": "CRITICAL",
			"image": "ghcr.io/grycap/cowsay",
			"script": "line1\r\nline2\r\n",
			"input": [
  			],
			"output": [
  			],
			"storage_providers": {
				"webdav": {
					"id": {
						"hostname": "` + server.URL + `",
						"login": "user",
						"password": "pass"
					}
				}
			},
			"allowed_users": []
		}
	`)
	req, _ := http.NewRequest("PUT", "/system/services", body)
	req.Header.Set("Authorization", "Bearer token")
	r.ServeHTTP(w, req)

	// Close the fake MinIO server
	defer server.Close()

	if w.Code != http.StatusNoContent {
		fmt.Println("Response body:", w.Body.String())

		fmt.Println(w.Body)
		t.Errorf("expecting code %d, got %d", http.StatusNoContent, w.Code)
	}

	if back.UpdatedService == nil {
		t.Fatal("expected backend to receive updated service, got nil")
	}
	if back.UpdatedService.Token != svc.Token {
		t.Fatalf("expected service token to be preserved on update, got %q", back.UpdatedService.Token)
	}
	if strings.Contains(back.UpdatedService.Script, "\r") {
		t.Fatalf("expected script without CR characters, got %q", back.UpdatedService.Script)
	}
}

func TestMakeUpdateHandlerUnauthorizedVO(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	back := backends.MakeFakeBackend()
	cfg := types.Config{
		OIDCGroups: []string{"vo1"},
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  "http://minio:9000",
			Region:    "us-east-1",
			AccessKey: "ak",
			SecretKey: "sk",
		},
	}
	// Existing service owned by user with different VO
	back.Service = &types.Service{Name: "svc", Owner: "owner", VO: "vo2"}

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "owner")
		c.Next()
	})
	r.PUT("/system/services", MakeUpdateHandler(&cfg, back))

	body := `{"name":"svc","vo":"vo1","token":"t","visibility":"private"}`
	req := httptest.NewRequest(http.MethodPut, "/system/services", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when VO not authorized, got %d", resp.Code)
	}
}

func TestMakeUpdateHandlerForbiddenOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{Name: "svc", Owner: "owner"}
	cfg := &types.Config{MinIOProvider: &types.MinIOProvider{}}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "other")
		c.Next()
	})
	r.PUT("/system/services", MakeUpdateHandler(cfg, back))

	body := `{"name":"svc","image":"img","script":"echo","token":"t","visibility":"private"}`
	req := httptest.NewRequest(http.MethodPut, "/system/services", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for different owner, got %d: %s", resp.Code, resp.Body.String())
	}
	if back.UpdatedService != nil {
		t.Fatalf("expected backend not to update service, got %+v", back.UpdatedService)
	}
}

func TestMakeUpdateHandlerRejectsVolumeMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	back := backends.MakeFakeBackend()
	back.Service = &types.Service{
		Name:  "svc",
		Owner: "owner",
		Volume: &types.ServiceVolumeConfig{
			Size:      "1Gi",
			MountPath: "/data",
		},
	}
	cfg := &types.Config{
		MinIOProvider: &types.MinIOProvider{},
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "owner")
		c.Next()
	})
	r.PUT("/system/services", MakeUpdateHandler(cfg, back))

	body := `{"name":"svc","image":"img","script":"echo","volume":{"size":"2Gi","mount_path":"/data"}}`
	req := httptest.NewRequest(http.MethodPut, "/system/services", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for volume mutation, got %d", resp.Code)
	}
}

func TestMakeUpdateHandlerLegacyServiceWithoutVolume(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	back := backends.MakeFakeBackend()
	kubeClientset := testclient.NewSimpleClientset()
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		if hreq.URL.Path != "/input" && hreq.URL.Path != "/output" && !strings.HasPrefix(hreq.URL.Path, "/minio/admin/v3/") {
			t.Errorf("Unexpected path in request, got: %s", hreq.URL.Path)
		}
		if hreq.URL.Path == "/minio/admin/v3/info" {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"Mode": "local", "Region": "us-east-1"}`))
			return
		}
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte(`{"status": "success"}`))
	}))
	defer server.Close()

	back.Service = &types.Service{
		Name:  "svc",
		Token: "token",
		CPU:   "1.0",
		Owner: "owner@example.com",
		StorageProviders: &types.StorageProviders{
			MinIO: map[string]*types.MinIOProvider{types.DefaultProvider: {
				Region:    "us-east-1",
				Endpoint:  server.URL,
				AccessKey: "ak",
				SecretKey: "sk",
			}},
		},
	}

	cfg := &types.Config{
		MinIOProvider: &types.MinIOProvider{
			Endpoint:  server.URL,
			Region:    "us-east-1",
			AccessKey: "ak",
			SecretKey: "sk",
		},
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "owner@example.com")
		c.Set("multitenancyConfig", auth.NewMultitenancyConfig(kubeClientset, "owner@example.com"))
		c.Next()
	})
	r.PUT("/system/services", MakeUpdateHandler(cfg, back))

	body := `{"name":"svc","cluster_id":"oscar","memory":"1Gi","cpu":"1.0","log_level":"CRITICAL","image":"ghcr.io/grycap/cowsay","script":"echo hi","allowed_users":[]}`
	req := httptest.NewRequest(http.MethodPut, "/system/services", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for legacy service update, got %d: %s", resp.Code, resp.Body.String())
	}
	if back.UpdatedService == nil {
		t.Fatal("expected backend to receive updated service")
	}
	if back.UpdatedService.Volume != nil {
		t.Fatalf("expected legacy service update to keep volume nil, got %+v", back.UpdatedService.Volume)
	}
}
