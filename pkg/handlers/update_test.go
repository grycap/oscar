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
		Name:  "cowsay",
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

func TestValidateServiceUpdateModelServingErrors(t *testing.T) {
	inferenceService := func() *types.Service {
		return &types.Service{
			Name:           "svc",
			CPU:            "0.2",
			Visibility:     types.PRIVATE,
			IsolationLevel: types.IsolationLevelService,
			ModelServing: &types.ModelServing{
				Spec: &types.KserveInference{
					ModelFormat: "onnx",
					Source:      "s3://model",
					APIVersion:  "v1",
				},
			},
		}
	}

	t.Run("rejects ModelServing type change", func(t *testing.T) {
		oldService := inferenceService()
		newService := inferenceService()
		newService.ModelServing = &types.ModelServing{
			Spec: &types.KserveLLMInference{Source: "hf://model"},
		}

		err := validateServiceUpdate(oldService, newService, &types.Config{
			KserveEnable:             true,
			ExposedServicesRouteKind: types.HTTPROUTE,
		})
		if err == nil || !strings.Contains(err.Error(), `cannot change ModelServing service type from "inference" to "llm_inference"`) {
			t.Fatalf("expected ModelServing type change error, got %v", err)
		}
	})

	t.Run("rejects unsupported Model Serving", func(t *testing.T) {
		oldService := inferenceService()
		newService := inferenceService()

		err := validateServiceUpdate(oldService, newService, &types.Config{
			KserveEnable:             false,
			ExposedServicesRouteKind: types.HTTPROUTE,
		})
		if err == nil || !strings.Contains(err.Error(), "Model Serving is not supported in this OSCAR deployment") {
			t.Fatalf("expected unsupported Model Serving error, got %v", err)
		}
	})
}

func TestMakeUpdateHandlerValidatesModelServingUpdates(t *testing.T) {
	tests := []struct {
		name        string
		oldService  *types.Service
		cfg         *types.Config
		body        string
		wantErrText string
	}{
		{
			name: "ModelServing type change",
			oldService: &types.Service{
				Name:   "svc",
				Script: "echo 'Hello, World!'",
				ModelServing: &types.ModelServing{
					Spec: &types.KserveInference{
						ModelFormat: "onnx",
						Source:      "s3://model",
						APIVersion:  "v1",
						SetAuth:     true,
					},
				},
			},
			cfg: &types.Config{
				KserveEnable:             true,
				ExposedServicesRouteKind: types.HTTPROUTE,
				MinIOProvider:            &types.MinIOProvider{},
			},
			body:        `{"name":"svc","script":"echo","model_serving":{"llm_inference":{"source":"hf://model"}}}`,
			wantErrText: `cannot change ModelServing service type from "inference" to "llm_inference"`,
		},
		{
			name: "unsupported Model Serving",
			oldService: &types.Service{
				Name:   "svc",
				Script: "echo 'Hello, World!'",
				ModelServing: &types.ModelServing{
					Spec: &types.KserveInference{
						ModelFormat: "onnx",
						Source:      "s3://model",
						APIVersion:  "v1",
						SetAuth:     true,
					},
				},
			},
			cfg: &types.Config{
				ExposedServicesRouteKind: types.HTTPROUTE,
				MinIOProvider:            &types.MinIOProvider{},
			},
			body:        `{"name":"svc","script":"echo","model_serving":{"inference":{"model_format":"onnx","source":"s3://model","api_version":"v1"}}}`,
			wantErrText: "Model Serving is not supported in this OSCAR deployment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			back := backends.MakeFakeBackend()
			back.Service = tt.oldService
			router := gin.New()
			router.PUT("/system/services", MakeUpdateHandler(tt.cfg, back))

			req := httptest.NewRequest(http.MethodPut, "/system/services", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "******")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body: %s", resp.Code, http.StatusBadRequest, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), tt.wantErrText) {
				t.Fatalf("response body = %q, want it to contain %q", resp.Body.String(), tt.wantErrText)
			}
			if back.UpdatedService != nil {
				t.Fatalf("expected validation to prevent backend update, got %+v", back.UpdatedService)
			}
		})
	}
}
