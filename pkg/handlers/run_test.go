package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
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

type GinResponseRecorder struct {
	http.ResponseWriter
}

func (GinResponseRecorder) CloseNotify() <-chan bool {
	return nil
}

func (GinResponseRecorder) Flush() {
}

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

			r.ServeHTTP(GinResponseRecorder{w}, req)
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
	gin.SetMode(gin.TestMode)

	priv, _ := rsa.GenerateKey(rand.Reader, 1024)

	// Fake OIDC server
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			w.Write([]byte(`{"issuer":"` + server.URL + `","jwks_uri":"` + server.URL + `/keys","userinfo_endpoint":"` + server.URL + `/userinfo"}`))
		case "/keys":
			w.Write([]byte(buildJWKS(&priv.PublicKey)))
		case "/userinfo":
			w.Write([]byte(`{"sub":"user@example.com","group_membership":["/group/test-vo"]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	oidcManager, err := auth.NewOIDCManager(server.URL, "user@example.com", []string{"test-vo"})
	if err != nil {
		t.Fatalf("failed to build oidc manager: %v", err)
	}
	auth.ClusterOidcManagers[server.URL] = oidcManager

	claims := jwt.MapClaims{
		"iss":              server.URL,
		"sub":              "user@example.com",
		"exp":              time.Now().Add(1 * time.Hour).Unix(),
		"iat":              time.Now().Unix(),
		"group_membership": []string{"/group/test-vo"},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test"
	rawToken, _ := token.SignedString(priv)

	service := &types.Service{Name: "hello", Token: "short-token", VO: "test-vo"}
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("proxied-oidc"))
	}))
	defer proxy.Close()
	target, _ := url.Parse(proxy.URL)
	back := &stubSyncBackend{service: service, proxyURL: target}

	r := gin.New()
	r.POST("/run/:serviceName", MakeRunHandler(&types.Config{}, back))

	req := httptest.NewRequest(http.MethodPost, "/run/hello", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)

	resp := &closeNotifierRecorder{ResponseRecorder: httptest.NewRecorder()}
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 from proxied request, got %d", resp.Code)
	}
	if resp.Body.String() != "proxied-oidc" {
		t.Fatalf("unexpected proxy body: %s", resp.Body.String())
	}
}

func buildJWKS(pub *rsa.PublicKey) string {
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	return `{"keys":[{"kty":"RSA","alg":"RS256","use":"sig","kid":"test","n":"` + n + `","e":"` + e + `"}]}`
}

type stubSyncBackend struct {
	service   *types.Service
	proxyURL  *url.URL
	listError error
}

func (s *stubSyncBackend) GetInfo() *types.ServerlessBackendInfo {
	return &types.ServerlessBackendInfo{Name: "stub", Version: "test"}
}
func (s *stubSyncBackend) ListServices() ([]*types.Service, error) {
	return []*types.Service{s.service}, s.listError
}
func (s *stubSyncBackend) CreateService(service types.Service) error       { return nil }
func (s *stubSyncBackend) ReadService(name string) (*types.Service, error) { return s.service, nil }
func (s *stubSyncBackend) UpdateService(service types.Service) error {
	s.service = &service
	return nil
}
func (s *stubSyncBackend) DeleteService(service types.Service) error { return nil }
func (s *stubSyncBackend) GetKubeClientset() kubernetes.Interface    { return fake.NewSimpleClientset() }
func (s *stubSyncBackend) GetProxyDirector(serviceName string) func(req *http.Request) {
	target := s.proxyURL
	return func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = target.Path
	}
}

func TestMakeRunHandlerUnauthorized(t *testing.T) {
	service := &types.Service{Name: "hello", Token: strings.Repeat("a", tokenLength)}
	target, _ := url.Parse("http://example.com/")
	back := &stubSyncBackend{service: service, proxyURL: target}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/run/:serviceName", MakeRunHandler(&types.Config{}, back))

	req := httptest.NewRequest(http.MethodPost, "/run/hello", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status, got %d", resp.Code)
	}
}

func TestMakeRunHandlerWithServiceToken(t *testing.T) {
	service := &types.Service{Name: "hello", Token: strings.Repeat("b", tokenLength)}
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("proxied"))
	}))
	defer proxy.Close()
	target, _ := url.Parse(proxy.URL)
	back := &stubSyncBackend{service: service, proxyURL: target}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/run/:serviceName", MakeRunHandler(&types.Config{}, back))

	req := httptest.NewRequest(http.MethodPost, "/run/hello", nil)
	req.Header.Set("Authorization", "Bearer "+service.Token)

	resp := &closeNotifierRecorder{ResponseRecorder: httptest.NewRecorder()}
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 from proxied request, got %d", resp.Code)
	}
	if body := resp.Body.String(); body != "proxied" {
		t.Fatalf("unexpected proxied body: %s", body)
	}
}

type closeNotifierRecorder struct {
	*httptest.ResponseRecorder
}

func (c *closeNotifierRecorder) CloseNotify() <-chan bool {
	ch := make(chan bool, 1)
	return ch
}
