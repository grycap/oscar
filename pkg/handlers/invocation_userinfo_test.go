package handlers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/grycap/oscar/v4/pkg/backends"
	"github.com/grycap/oscar/v4/pkg/testsupport"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// invocationOIDCFixture serves discovery and UserInfo locally; its signed token
// lets the cached-token test exercise the real verifier.
func invocationOIDCFixture(t *testing.T) (string, func() bool, *atomic.Int32, *atomic.Bool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var fail atomic.Bool
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_, _ = w.Write([]byte(`{"issuer":"` + server.URL + `","userinfo_endpoint":"` + server.URL + `/userinfo","jwks_uri":"` + server.URL + `/keys"}`))
		case "/userinfo":
			calls.Add(1)
			if fail.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`{"sub":"test-user","group_membership":["test-group"]}`))
		case "/keys":
			_, _ = w.Write([]byte(buildRSAJWK(&key.PublicKey)))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	manager, err := auth.NewOIDCManager(server.URL, "test-user", []string{"test-group"})
	if err != nil {
		t.Fatal(err)
	}
	auth.ClusterOidcManagers[server.URL] = manager
	t.Cleanup(func() { delete(auth.ClusterOidcManagers, server.URL) })
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": server.URL, "sub": "test-user", "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return token, func() bool { return manager.IsAuthorised(token) }, &calls, &fail
}

func TestJobUserInfoFailureDoesNotPanic(t *testing.T) {
	testsupport.SkipIfCannotListen(t)
	token, _, calls, fail := invocationOIDCFixture(t)
	fail.Store(true)
	back := backends.MakeFakeBackend()
	back.Services = []*types.Service{{Name: "test-service"}}
	r := gin.New() // without Gin recovery: a panic fails the test
	r.POST("/job/:serviceName", MakeJobHandler(&types.Config{}, types.QuotaBackend{}, back, nil))
	req := httptest.NewRequest(http.MethodPost, "/job/test-service", strings.NewReader("input"))
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadGateway {
		t.Fatalf("expected provider error 502, got %d: %s", resp.Code, resp.Body.String())
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected exactly one UserInfo request, got %d", got)
	}
}

func TestRunUserInfoFailureDoesNotPanic(t *testing.T) {
	for _, cached := range []bool{false, true} {
		name := "uncached"
		if cached {
			name = "cached valid token"
		}
		t.Run(name, func(t *testing.T) {
			testsupport.SkipIfCannotListen(t)
			token, authorize, calls, fail := invocationOIDCFixture(t)
			if cached && !authorize() {
				t.Fatal("expected valid token to be cached")
			}
			before := calls.Load()
			fail.Store(true)
			back := backends.MakeFakeSyncBackend()
			back.Services = []*types.Service{{Name: "test-service"}}
			r := gin.New() // without Gin recovery: a panic fails the test
			r.POST("/run/:serviceName", MakeRunHandler(&types.Config{}, back))
			req := httptest.NewRequest(http.MethodPost, "/run/test-service", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			resp := httptest.NewRecorder()
			r.ServeHTTP(resp, req)
			if resp.Code != http.StatusBadGateway {
				t.Fatalf("expected provider error 502, got %d: %s", resp.Code, resp.Body.String())
			}
			if got := calls.Load() - before; got != 1 {
				t.Fatalf("expected exactly one additional UserInfo request, got %d", got)
			}
		})
	}
}

func TestRunWithOIDCUserInfoSuccess(t *testing.T) {
	testsupport.SkipIfCannotListen(t)
	token, _, calls, _ := invocationOIDCFixture(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	back := backends.MakeFakeSyncBackend()
	back.Services = []*types.Service{{Name: "test-service", Owner: "test-user"}}
	back.ProxyDirector = func(req *http.Request) {
		req.URL.Scheme = "http"
		req.URL.Host = upstream.Listener.Addr().String()
	}
	r := gin.New()
	r.POST("/run/:serviceName", MakeRunHandler(&types.Config{OIDCGroups: []string{"test-group"}}, back))
	req := httptest.NewRequest(http.MethodPost, "/run/test-service", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := &closeNotifierRecorder{ResponseRecorder: httptest.NewRecorder()}
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected successful OIDC invocation, got %d: %s", resp.Code, resp.Body.String())
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected exactly one UserInfo request, got %d", got)
	}
}

func TestJobWithOIDCUserInfoSuccess(t *testing.T) {
	testsupport.SkipIfCannotListen(t)
	token, _, calls, _ := invocationOIDCFixture(t)
	back := backends.MakeFakeBackend()
	back.Services = []*types.Service{{Name: "test-service", Owner: "test-user", Namespace: auth.ServicesNamespace, CPU: "100m", Memory: "128Mi", Labels: map[string]string{}}}
	client := back.GetKubeClientset()
	if _, err := client.CoreV1().Secrets(auth.ServicesNamespace).Create(context.Background(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: auth.FormatUID("test-user"), Namespace: auth.ServicesNamespace},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	r := gin.New() // without Gin recovery
	r.POST("/job/:serviceName", MakeJobHandler(&types.Config{OIDCGroups: []string{"test-group"}, ServicesNamespace: auth.ServicesNamespace}, types.QuotaBackend{KubeClientset: client}, back, nil))
	req := httptest.NewRequest(http.MethodPost, "/job/test-service", strings.NewReader("input"))
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected successful OIDC job, got %d: %s", resp.Code, resp.Body.String())
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected exactly one UserInfo request, got %d", got)
	}
}

func TestInvocationStillVerifiesTokenWithValidUserInfo(t *testing.T) {
	for _, path := range []string{"job", "run"} {
		t.Run(path, func(t *testing.T) {
			testsupport.SkipIfCannotListen(t)
			token, _, calls, _ := invocationOIDCFixture(t)
			parts := strings.Split(token, ".")
			if parts[2][0] == 'A' {
				parts[2] = "B" + parts[2][1:]
			} else {
				parts[2] = "A" + parts[2][1:]
			}
			invalidToken := strings.Join(parts, ".")
			r := gin.New()
			expected := http.StatusUnauthorized
			if path == "job" {
				back := backends.MakeFakeBackend()
				back.Services = []*types.Service{{Name: "test-service", Owner: "test-user", Labels: map[string]string{}}}
				r.POST("/job/:serviceName", MakeJobHandler(&types.Config{}, types.QuotaBackend{}, back, nil))
			} else {
				back := backends.MakeFakeSyncBackend()
				back.Services = []*types.Service{{Name: "test-service", Owner: "test-user"}}
				r.POST("/run/:serviceName", MakeRunHandler(&types.Config{}, back))
				expected = http.StatusNotFound
			}
			req := httptest.NewRequest(http.MethodPost, "/"+path+"/test-service", nil)
			req.Header.Set("Authorization", "Bearer "+invalidToken)
			resp := httptest.NewRecorder()
			r.ServeHTTP(resp, req)
			if resp.Code != expected {
				t.Fatalf("expected invalid token rejection %d, got %d: %s", expected, resp.Code, resp.Body.String())
			}
			if got := calls.Load(); got != 1 {
				t.Fatalf("expected exactly one UserInfo request, got %d", got)
			}
		})
	}
}

func TestConcurrentCachedTokenAuthorizationWithUserInfo(t *testing.T) {
	testsupport.SkipIfCannotListen(t)
	token, authorize, calls, _ := invocationOIDCFixture(t)
	if !authorize() {
		t.Fatal("expected valid token to be cached")
	}
	issuer, err := auth.GetIssuerFromToken(token)
	if err != nil {
		t.Fatal(err)
	}
	manager := auth.ClusterOidcManagers[issuer]
	ui, err := manager.GetUserInfo(token)
	if err != nil {
		t.Fatal(err)
	}
	before := calls.Load()
	var wg sync.WaitGroup
	results := make(chan bool, 8)
	start := make(chan struct{})
	for i := 0; i < cap(results); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- manager.IsAuthorisedWithUserInfo(token, ui)
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for valid := range results {
		if !valid {
			t.Error("cached valid token should remain authorized")
		}
	}
	if got := calls.Load() - before; got != 0 {
		t.Fatalf("authorization fetched UserInfo %d additional times", got)
	}
}
