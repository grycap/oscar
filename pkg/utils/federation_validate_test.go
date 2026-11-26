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

package utils

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/grycap/oscar/v4/pkg/types"
)

func TestValidateFederationEndpointRejectedBuiltinRanges(t *testing.T) {
	cases := []struct {
		endpoint   string
		wantSubstr string
	}{
		{"http://127.0.0.1/", "resolves to internal address"},
		{"http://169.254.169.254/", "169.254.169.254"},
		{"http://10.0.0.5/", "10.0.0.5"},
		{"http://172.16.4.2/", "172.16.4.2"},
		{"http://192.168.1.10/", "192.168.1.10"},
		{"http://[::1]/", "federation endpoint"},
		{"", "empty federation endpoint"},
		{"http://user:pass@peer.invalid/", "embedded credentials"},
		{"http:///path", "does not contain a host"},
	}
	for _, tc := range cases {
		t.Run(tc.endpoint, func(t *testing.T) {
			err := validateFederationEndpoint(tc.endpoint, nil)
			if err == nil {
				t.Fatalf("endpoint %q accepted, want rejection", tc.endpoint)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

func TestValidateFederationEndpointAllowlist(t *testing.T) {
	t.Run("allowlisted public host matches any port and is case-insensitive", func(t *testing.T) {
		allowlist := NewFederationAllowlist([]string{"peer.local"})
		if err := validateFederationEndpoint("http://peer.local/", allowlist); err != nil {
			t.Errorf("expected peer.local to be allowed, got: %v", err)
		}
		if err := validateFederationEndpoint("http://PEER.LOCAL:8080/x", allowlist); err != nil {
			t.Errorf("expected PEER.LOCAL:8080 to be allowed, got: %v", err)
		}
	})

	t.Run("allowlisted host:port only matches that exact port", func(t *testing.T) {
		allowlist := NewFederationAllowlist([]string{"internalhost:8443"})
		if err := validateFederationEndpoint("https://internalhost:8443/", allowlist); err != nil {
			t.Errorf("expected internalhost:8443 to be allowed, got: %v", err)
		}
		if err := validateFederationEndpoint("https://internalhost:80/", allowlist); err == nil {
			t.Error("expected internalhost:80 to be rejected, it matched an entry pinned to port 8443")
		}
	})

	t.Run("allowlisted private endpoint skips the IP range check", func(t *testing.T) {
		allowlist := NewFederationAllowlist([]string{"10.0.0.7:8080"})
		if err := validateFederationEndpoint("http://10.0.0.7:8080/", allowlist); err != nil {
			t.Errorf("expected allowlisted private endpoint to be allowed, got: %v", err)
		}
		if err := validateFederationEndpoint("http://10.0.0.7:8081/", allowlist); err == nil {
			t.Error("expected the same private endpoint on another port to be rejected")
		}
	})

	t.Run("public IP literal is allowed without an allowlist", func(t *testing.T) {
		if err := validateFederationEndpoint("http://93.184.216.34/", nil); err != nil {
			t.Errorf("expected public IP literal to be allowed, got: %v", err)
		}
	})
}

func TestValidateFederationEndpointSchemes(t *testing.T) {
	for _, endpoint := range []string{"file:///etc/passwd", "ftp://peer.local", "gopher://peer.local"} {
		err := validateFederationEndpoint(endpoint, nil)
		if err == nil {
			t.Errorf("scheme %q accepted, want rejection", endpoint)
		} else if !strings.Contains(err.Error(), "unsupported scheme") {
			t.Errorf("error %q does not name the scheme problem", err.Error())
		}
	}
}

func TestValidateFederationAllowlistEntries(t *testing.T) {
	valid := []string{"peer.local", "peer2:8080", "[2001:db8::1]", "[2001:db8::1]:8443", "  spaced.local  "}
	if errs := ValidateFederationAllowlistEntries(valid); len(errs) != 0 {
		t.Fatalf("expected all valid entries to pass, got: %v", errs)
	}

	invalid := []string{"", ":", "[", "[::1]junk"}
	errs := ValidateFederationAllowlistEntries(invalid)
	if len(errs) != len(invalid) {
		t.Fatalf("expected %d errors, got: %v", len(invalid), errs)
	}
	for i, raw := range invalid {
		if !strings.Contains(errs[i], raw) {
			t.Errorf("expected error %d to name entry %q, got: %s", i, raw, errs[i])
		}
	}

	mixed := []string{"peer.local", "[::1]junk"}
	if errs := ValidateFederationAllowlistEntries(mixed); len(errs) != 1 {
		t.Fatalf("expected exactly one error for the mixed list, got: %v", errs)
	} else if !strings.Contains(errs[0], "[::1]junk") {
		t.Errorf("expected the error to name the bad entry, got: %s", errs[0])
	}
}

func TestIsDisallowedFederationIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"169.254.169.254", true},
		{"10.0.0.5", true},
		{"172.16.4.2", true},
		{"172.31.255.255", true},
		{"192.168.1.10", true},
		{"::1", true},
		{"fe80::1", true},
		{"fc00::5", true},
		{"fdff:ffff:0:0:0:0:0:0", true},
		{"::ffff:127.0.0.1", true},
		{"0.0.0.0", true},
		{"8.8.8.8", false},
		{"172.15.0.1", false},
		{"172.32.0.1", false},
		{"93.184.216.34", false},
		{"2001:db8::1", false},
	}
	for _, tc := range cases {
		ip := net.ParseIP(tc.ip)
		if ip == nil {
			t.Fatalf("test bug: %q is not an IP", tc.ip)
		}
		if got := isDisallowedFederationIP(ip); got != tc.want {
			t.Errorf("isDisallowedFederationIP(%s) = %v, want %v", tc.ip, got, tc.want)
		}
	}
	if got := isDisallowedFederationIP(nil); !got {
		t.Error("isDisallowedFederationIP(nil) = false, want true")
	}
}

func TestSetFederationAuth(t *testing.T) {
	allowlisted := NewFederationAllowlist([]string{"peer.local:8080"})
	cluster := &types.Cluster{AuthUser: "u", AuthPassword: "p"}
	emptyCluster := &types.Cluster{}
	noCluster := (*types.Cluster)(nil)

	cases := []struct {
		name      string
		host      string
		allowlist *FederationAllowlist
		cluster   *types.Cluster
		caller    string
		wantAuth  string
	}{
		{"caller token forwarded to allowlisted destination", "peer.local:8080", allowlisted, noCluster, "Bearer caller-tok", "Bearer caller-tok"},
		{"allowlisted destination without caller falls back to cluster basic auth", "peer.local:8080", allowlisted, cluster, "", "Basic dTpw"},
		{"non-allowlisted destination drops the caller token and uses cluster basic auth", "other.local:8080", allowlisted, cluster, "Bearer caller-tok", "Basic dTpw"},
		{"non-allowlisted destination without cluster credentials carries no credentials", "other.local:8080", allowlisted, emptyCluster, "Bearer caller-tok", ""},
		{"nil allowlist never forwards the caller token", "peer.local:8080", nil, cluster, "Bearer caller-tok", "Basic dTpw"},
		{"nil allowlist without cluster credentials carries no credentials", "peer.local:8080", nil, noCluster, "Bearer caller-tok", ""},
		{"no caller token and no cluster credentials leaves the header empty", "other.local:8080", allowlisted, emptyCluster, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "http://"+tc.host+"/x", nil)
			if err != nil {
				t.Fatalf("failed to build request: %v", err)
			}
			req.Header.Set("Authorization", tc.caller)
			setFederationAuth(req, tc.cluster, tc.allowlist)
			if got := req.Header.Get("Authorization"); got != tc.wantAuth {
				t.Errorf("Authorization = %q, want %q", got, tc.wantAuth)
			}
		})
	}
}

// TestFederationTargetEnforcesValidation runs real HTTP traffic against a
// local httptest server whose host:port is allowlisted, covering: redirect
// revalidation (blocked and bounded redirects), the credential forwarding
// rule, and allowlisted internal endpoints succeeding end to end.
func TestFederationTargetEnforcesValidation(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/system/services/blocked-redirect", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://169.254.169.254/")
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/system/services/hop1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/system/services/hop2")
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/system/services/hop2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/system/services/hop3")
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/system/services/hop3", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/system/services/loop", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/system/services/loop")
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/system/services/echo-auth", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Header.Get("Authorization")))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	srvURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	allowlist := NewFederationAllowlist([]string{srvURL.Host})
	target, err := NewFederationTarget(srv.URL, types.Cluster{}, allowlist, 10*time.Second)
	if err != nil {
		t.Fatalf("expected the allowlisted test server endpoint to be accepted, got: %v", err)
	}

	t.Run("redirect chain on the same destination succeeds", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, target.url("hop1"), nil)
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}
		resp, err := target.do(req)
		if err != nil {
			t.Fatalf("followed redirect chain: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "ok" {
			t.Errorf("expected body 'ok', got %q", string(body))
		}
	})

	t.Run("redirect to a blocked internal address is rejected", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, target.url("blocked-redirect"), nil)
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}
		_, err = target.do(req)
		if err == nil {
			t.Fatal("expected the redirect to 169.254.169.254 to be rejected")
		}
		if !strings.Contains(err.Error(), "is not allowed") {
			t.Errorf("error %q does not name the blocked redirect", err.Error())
		}
	})

	t.Run("redirect loop is bounded", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, target.url("loop"), nil)
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}
		_, err = target.do(req)
		if err == nil {
			t.Fatal("expected the redirect loop to be cut off")
		}
		if !strings.Contains(err.Error(), "exceeded") {
			t.Errorf("error %q does not mention the redirect limit", err.Error())
		}
	})

	t.Run("caller credentials are forwarded to the allowlisted destination", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, target.url("echo-auth"), nil)
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer caller-tok")
		resp, err := target.do(req)
		if err != nil {
			t.Fatalf("request to allowlisted destination failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "Bearer caller-tok" {
			t.Errorf("server saw Authorization %q, want %q", string(body), "Bearer caller-tok")
		}
	})
}
