package types

import (
	"reflect"
	"testing"
)

func TestOIDCAudiencesConfig(t *testing.T) {
	for _, pair := range [][2]string{{"OSCAR_USERNAME", "testuser"}, {"OSCAR_PASSWORD", "testpass"}, {"MINIO_ACCESS_KEY", "testaccess"}, {"MINIO_SECRET_KEY", "testsecret"}} {
		t.Setenv(pair[0], pair[1])
	}
	for _, tc := range []struct {
		name  string
		value string
		valid bool
	}{
		{"two issuer allowlists", `{"https://issuer-one":["oscar-client","shared-api"],"https://issuer-two":["another-client"]}`, true},
		{"malformed JSON", `{`, false},
		{"empty audience", `{"https://issuer-one":[""]}`, false},
		{"empty list", `{"https://issuer-one":[]}`, false},
		{"null map", `null`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OIDC_AUDIENCES", tc.value)
			cfg, err := ReadConfig()
			if (err == nil) != tc.valid {
				t.Fatalf("ReadConfig() error = %v, valid = %v", err, tc.valid)
			}
			if tc.valid && !reflect.DeepEqual(cfg.OIDCAudiences["https://issuer-one"], []string{"oscar-client", "shared-api"}) {
				t.Fatalf("unexpected audience allowlist: %v", cfg.OIDCAudiences)
			}
		})
	}
}
