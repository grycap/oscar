package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/grycap/oscar/v4/pkg/testsupport"
)

func TestOIDCSignedAudienceAllowlist(t *testing.T) {
	testsupport.SkipIfCannotListen(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var issuer string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			fmt.Fprintf(w, `{"issuer":%q,"jwks_uri":%q,"userinfo_endpoint":%q}`, issuer, issuer+"/keys", issuer+"/userinfo")
		case "/keys":
			n := base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes())
			e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes())
			fmt.Fprintf(w, `{"keys":[{"kty":"RSA","kid":"test-key","use":"sig","alg":"RS256","n":%q,"e":%q}]}`, n, e)
		case "/userinfo":
			fmt.Fprint(w, `{"sub":"test-user","group_membership":["allowed-group"]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	issuer = server.URL
	manager, err := NewOIDCManager(issuer, "test-user", []string{"allowed-group"}, "oscar-client", "shared-api")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		aud  string
		exp  time.Time
		want bool
	}{
		{"OSCAR audience", "oscar-client", time.Now().Add(time.Hour), true},
		{"shared audience", "shared-api", time.Now().Add(time.Hour), true},
		{"other client", "other-client", time.Now().Add(time.Hour), false},
		{"expired allowed token", "oscar-client", time.Now().Add(-time.Hour), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
				"iss": issuer, "sub": "test-user", "aud": tc.aud, "exp": tc.exp.Unix(),
			})
			token.Header["kid"] = "test-key"
			raw, err := token.SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			if got := manager.IsAuthorised(raw); got != tc.want {
				t.Errorf("IsAuthorised() = %v; want %v", got, tc.want)
			}
		})
	}
}
