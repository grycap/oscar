package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/testsupport"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	testclient "k8s.io/client-go/kubernetes/fake"
)

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
			"alpine": false,
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
	if strings.Contains(back.UpdatedService.Script, "\r") {
		t.Fatalf("expected script without CR characters, got %q", back.UpdatedService.Script)
	}
}
