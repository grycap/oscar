package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/testsupport"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	testclient "k8s.io/client-go/kubernetes/fake"
)

var (
	testMinIOProviderRun types.MinIOProvider = types.MinIOProvider{
		Endpoint:  "http://minio.minio:30300",
		Verify:    true,
		AccessKey: "minio",
		SecretKey: "ZjhhMWZk",
		Region:    "us-east-1",
	}

	testConfigValidRun types.Config = types.Config{
		MinIOProvider: &testMinIOProviderRun,
	}
)

func TestMakeRunHandler(t *testing.T) {

	scenarios := []struct {
		name        string
		returnError bool
		errType     string
	}{
		//{"Valid service test", false, ""},
		{"Service Not Found test", true, "404"},
		{"Internal Server Error test", true, "500"},
		{"Bad token: split token", true, "splitErr"},
		{"Bad token: diff service token", true, "diffErr"},
	}
	for _, s := range scenarios {
		back := backends.MakeFakeSyncBackend()
		http.DefaultClient.Timeout = 400 * time.Second
		r := gin.Default()
		r.POST("/run/:serviceName", MakeRunHandler(&testConfigValidRun, back))

		t.Run(s.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			serviceName := "testName"

			req, _ := http.NewRequest("POST", "/run/"+serviceName, nil)
			req.Header.Set("Authorization", "Bearer 11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf")

			if s.returnError {
				switch s.errType {
				case "404":
					back.AddError("ReadService", k8serr.NewGone("Not Found"))
				case "500":
					err := errors.New("Not found")
					back.AddError("ReadService", k8serr.NewInternalError(err))
				case "splitErr":
					req.Header.Set("Authorization", "11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf")
				case "diffErr":
					req.Header.Set("Authorization", "Bearer 11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513dfg")
				}
			}

			r.ServeHTTP(w, req)
			if s.returnError {

				if s.errType == "splitErr" || s.errType == "diffErr" {
					if w.Code != http.StatusUnauthorized {
						t.Errorf("expecting code %d, got %d", http.StatusUnauthorized, w.Code)
					}
				}

				if s.errType == "404" && w.Code != http.StatusNotFound {
					t.Errorf("expecting code %d, got %d", http.StatusNotFound, w.Code)
				}

				if s.errType == "500" && w.Code != http.StatusInternalServerError {
					t.Errorf("expecting code %d, got %d", http.StatusInternalServerError, w.Code)
				}

			} else {
				if w.Code != http.StatusOK {
					t.Errorf("expecting code %d, got %d", http.StatusOK, w.Code)
				}
			}
		})
	}
}

func TestMakeRunHandlerOIDCPath(t *testing.T) {
	testsupport.SkipIfCannotListen(t)

	back := backends.MakeFakeBackend()
	kubeClientset := testclient.NewSimpleClientset()
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, hreq *http.Request) {
		fmt.Println("hreq.URL.Path:", hreq.URL.Path)
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
	rawToken := "11e387cf727630d899925d57fceb4578f478c44be6cde0ae3fe886d8be513acf"
	svc := &types.Service{
		Name:  "hello",
		Token: rawToken,
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
	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Region:    "us-east-1",
			Endpoint:  server.URL,
			AccessKey: "ak",
			SecretKey: "sk",
		},
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "somelonguid@egi.eu")
		c.Set("multitenancyConfig", auth.NewMultitenancyConfig(kubeClientset, "somelonguid@egi.eu"))
	})
	r.POST("/run/:serviceName", MakeRunHandler(&cfg, back))

	req := httptest.NewRequest(http.MethodPost, "/run/hello", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)

	resp := &closeNotifierRecorder{ResponseRecorder: httptest.NewRecorder()}
	r.ServeHTTP(resp, req)
	defer server.Close()

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 from proxied request, got %d", resp.Code)
	}
}

func TestMakeRunHandlerUnauthorized(t *testing.T) {
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

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "somelonguid@egi.eu")
		c.Set("multitenancyConfig", auth.NewMultitenancyConfig(kubeClientset, "somelonguid@egi.eu"))
		c.Next()
	})

	r.POST("/run/:serviceName", MakeRunHandler(&testConfigValidRun, back))

	req := httptest.NewRequest(http.MethodPost, "/run/hello", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	defer server.Close()

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status, got %d", resp.Code)
	}
}

func TestMakeRunHandlerWithServiceToken(t *testing.T) {
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
	cfg := types.Config{
		MinIOProvider: &types.MinIOProvider{
			Region:    "us-east-1",
			Endpoint:  server.URL,
			AccessKey: "ak",
			SecretKey: "sk",
		},
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("uidOrigin", "somelonguid@egi.eu")
		c.Set("multitenancyConfig", auth.NewMultitenancyConfig(kubeClientset, "somelonguid@egi.eu"))
	})
	r.POST("/run/:serviceName", MakeRunHandler(&cfg, back))

	req := httptest.NewRequest(http.MethodPost, "/run/hello", nil)
	req.Header.Set("Authorization", "Bearer "+svc.Token)

	resp := &closeNotifierRecorder{ResponseRecorder: httptest.NewRecorder()}
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 from proxied request, got %d", resp.Code)
	}
}

type closeNotifierRecorder struct {
	*httptest.ResponseRecorder
}

func (c *closeNotifierRecorder) CloseNotify() <-chan bool {
	ch := make(chan bool, 1)
	return ch
}
