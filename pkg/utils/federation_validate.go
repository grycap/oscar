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
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/grycap/oscar/v4/pkg/types"
)

// Timeouts applied to outbound federation operations:
//   - every request performed by the federation client,
//   - the credential-verification check performed before a federated creation,
//   - the DNS resolution executed while validating an endpoint.
const (
	federationRequestTimeout   = 30 * time.Second
	federationAuthCheckTimeout = 10 * time.Second
	federationDNSLookupTimeout = 5 * time.Second
	// maxFederationRedirects bounds how many redirects the federation client
	// follows; every hop is revalidated with the same endpoint rules.
	maxFederationRedirects = 5
)

// allowedFederationSchemes restricts federation destinations to plain and
// TLS HTTP. Any other scheme (file://, ftp://, gopher://, ...) is rejected by
// the endpoint validator below with an explicit error.
var allowedFederationSchemes = map[string]bool{"http": true, "https": true}

// allowlistEntry is one parsed entry of the allowed_federation_endpoints list.
// An empty port means "any port on that host"; a port present in the entry
// must match exactly.
type allowlistEntry struct {
	host string
	port string
}

// FederationAllowlist holds the parsed allowed_federation_endpoints values
// from the additional configuration ConfigMap.
//
// A nil or empty allowlist is the safe default: nothing is explicitly
// trusted, caller credentials are never forwarded, and endpoint validation
// rejects loopback, link-local and private ranges, leaving only publicly
// reachable clusters as valid federation destinations until an operator adds
// entries to the list.
type FederationAllowlist struct {
	entries []allowlistEntry
}

// NewFederationAllowlist parses the allowed_federation_endpoints values. Malformed
// entries are logged and skipped so a single bad value does not disable the
// whole list. The allowlist instance may be used as-is (a nil one is treated
// as an empty one by every method).
func NewFederationAllowlist(values []string) *FederationAllowlist {
	al := &FederationAllowlist{}
	for _, raw := range values {
		entry, err := parseAllowlistEntry(raw)
		if err != nil {
			log.Printf("WARNING: skipping malformed allowed_federation_endpoints entry %q: %v", raw, err)
			continue
		}
		al.entries = append(al.entries, entry)
	}
	return al
}

// ValidateFederationAllowlistEntries checks every allowed_federation_endpoints
// value and returns one error message per malformed entry. An empty result
// means the whole list is valid.
func ValidateFederationAllowlistEntries(values []string) []string {
	invalid := make([]string, 0, len(values))
	for _, raw := range values {
		if _, err := parseAllowlistEntry(raw); err != nil {
			invalid = append(invalid, fmt.Sprintf("invalid allowed federation endpoint %q: %v", raw, err))
		}
	}
	return invalid
}

// parseAllowlistEntry parses one entry: "host" or "host:port". IPv6 literals
// use the standard bracket form: "[2001:db8::1]" or "[2001:db8::1]:8080".
func parseAllowlistEntry(raw string) (allowlistEntry, error) {
	entry := strings.TrimSpace(raw)
	if entry == "" {
		return allowlistEntry{}, fmt.Errorf("empty entry")
	}
	// IPv6 literal: [addr] or [addr]:port
	if strings.HasPrefix(entry, "[") {
		end := strings.IndexByte(entry, ']')
		if end <= 1 {
			return allowlistEntry{}, fmt.Errorf("unterminated IPv6 literal")
		}
		host := entry[1:end]
		rest := entry[end+1:]
		if rest == "" {
			return allowlistEntry{host: host}, nil
		}
		if !strings.HasPrefix(rest, ":") {
			return allowlistEntry{}, fmt.Errorf("unexpected content after IPv6 literal")
		}
		port := rest[1:]
		if port == "" {
			return allowlistEntry{}, fmt.Errorf("empty port after IPv6 literal")
		}
		return allowlistEntry{host: host, port: port}, nil
	}
	if !strings.Contains(entry, ":") {
		return allowlistEntry{host: entry}, nil
	}
	host, port, err := net.SplitHostPort(entry)
	if err != nil {
		return allowlistEntry{}, fmt.Errorf("invalid host:port value: %v", err)
	}
	if host == "" || port == "" {
		return allowlistEntry{}, fmt.Errorf("invalid host:port value")
	}
	return allowlistEntry{host: host, port: port}, nil
}

// allows reports whether the (lower-case) host and port match a list entry.
// Host comparison is case-insensitive; an entry without a port matches any
// port on that host, an entry with a port only matches that exact port.
func (al *FederationAllowlist) allows(host, port string) bool {
	if al == nil || len(al.entries) == 0 {
		return false
	}
	host = strings.ToLower(host)
	for _, entry := range al.entries {
		entryHost := strings.ToLower(entry.host)
		if entryHost != host {
			continue
		}
		if entry.port == "" || entry.port == port {
			return true
		}
	}
	return false
}

// federationDeniedNets are the ranges that must never be reachable through a
// federation request whose endpoint is not explicitly allowlisted. They cover
// loopback, link-local (including the cloud metadata endpoint
// 169.254.169.254) and private/unique-local ranges, in both IPv4 and IPv6.
var federationDeniedNets = func() []*net.IPNet {
	cidrs := []string{
		"127.0.0.0/8",    // IPv4 loopback
		"169.254.0.0/16", // IPv4 link-local
		"10.0.0.0/8",     // RFC 1918 private
		"172.16.0.0/12",  // RFC 1918 private
		"192.168.0.0/16", // RFC 1918 private
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique-local (ULA)
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("utils: invalid static CIDR %q: %v", cidr, err))
		}
		nets = append(nets, ipNet)
	}
	return nets
}()

// isDisallowedFederationIP reports whether ip falls into one of the ranges in
// federationDeniedNets. IPv4-mapped IPv6 addresses (e.g. ::ffff:127.0.0.1)
// and host addresses on unspecified interfaces are always disallowed.
func isDisallowedFederationIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() {
		return true
	}
	v4 := ip.To4()
	if v4 != nil {
		for _, ipNet := range federationDeniedNets {
			if ipNet.Contains(v4) {
				return true
			}
		}
		return false
	}
	for _, ipNet := range federationDeniedNets {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// endpointPort returns the explicit port of u, defaulted according to the
// scheme, mirroring how an HTTP client would dial it.
func endpointPort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	return "80"
}

// validateFederationEndpoint verifies that endpoint is a federation
// destination the caller is allowed to use, in this order:
//
//  1. the endpoint has a host and uses one of the http/https schemes
//     (file://, ftp://, gopher:// and similar are rejected with an error
//     that names the offending scheme) and does not carry embedded
//     userinfo credentials,
//  2. the host and port are part of the allowlist, when one is configured
//     (a "host" entry matches any port on that host, a "host:port" entry
//     matches only that exact port, hosts are compared case-insensitively);
//     allowlisted endpoints are trusted and skip step 3, which is how
//     operators enable internal federation peers,
//  3. the host resolves (bounded by federationDNSLookupTimeout) and none of
//     the resolved addresses falls inside a loopback, link-local or private
//     range.
//
// A nil allowlist allows nothing: step 2 never matches and every endpoint is
// subjected to the IP range checks.
//
// The exported ValidateFederationEndpoint wrapper exists so API handlers can
// reject unsafe endpoints with a 400 error before any federation traffic is
// generated, without constructing a FederationTarget.
func ValidateFederationEndpoint(endpoint string, allowlist *FederationAllowlist) error {
	return validateFederationEndpoint(endpoint, allowlist)
}

func validateFederationEndpoint(endpoint string, allowlist *FederationAllowlist) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return fmt.Errorf("empty federation endpoint")
	}
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid federation endpoint %q: %v", endpoint, err)
	}
	scheme := strings.ToLower(endpointURL.Scheme)
	if !allowedFederationSchemes[scheme] {
		return fmt.Errorf("federation endpoint %q uses unsupported scheme %q (only http and https are allowed)", endpoint, endpointURL.Scheme)
	}
	if endpointURL.User != nil {
		return fmt.Errorf("federation endpoint %q must not contain embedded credentials", endpoint)
	}
	host := endpointURL.Hostname()
	if host == "" {
		return fmt.Errorf("federation endpoint %q does not contain a host", endpoint)
	}
	port := endpointPort(endpointURL)
	if allowlist.allows(host, port) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), federationDNSLookupTimeout)
	defer cancel()
	resolver := new(net.Resolver)
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("federation endpoint %q: could not resolve host %q: %v", endpoint, host, err)
	}
	for _, ip := range ips {
		if isDisallowedFederationIP(ip.IP) {
			return fmt.Errorf("federation endpoint %q resolves to internal address %s, which is not allowed; add the endpoint to allowed_federation_endpoints to trust it",
				endpoint, ip.IP.String())
		}
	}
	return nil
}

// FederationTarget is a federation destination that passed the endpoint
// validation against the allowlist. It carries the request base URL and the
// shared HTTP client used for every request sent to the destination.
type FederationTarget struct {
	// baseURL is the validated endpoint with the federation collection path
	// (system/services) appended.
	baseURL *url.URL
	// Client enforces the timeout, the cluster TLS configuration and the
	// redirect validation described in NewFederationTarget.
	Client *http.Client

	cluster   types.Cluster
	allowlist *FederationAllowlist
}

// url returns the request URL for the destination, appending the given path
// segments (for example a service name) to the collection path.
func (t *FederationTarget) url(segments ...string) string {
	u := *t.baseURL
	if len(segments) > 0 {
		parts := make([]string, 0, len(segments)+1)
		parts = append(parts, u.Path)
		parts = append(parts, segments...)
		u.Path = path.Join(parts...)
	}
	// Make sure the path stays absolute even when the endpoint carried no
	// path of its own.
	if u.Path != "" && !strings.HasPrefix(u.Path, "/") {
		u.Path = "/" + u.Path
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// do executes req against the destination applying the credential forwarding
// rule to its Authorization header (see setFederationAuth).
func (t *FederationTarget) do(req *http.Request) (*http.Response, error) {
	setFederationAuth(req, &t.cluster, t.allowlist)
	return t.Client.Do(req)
}

// NewFederationTarget validates endpoint with the federation endpoint rules
// (scheme, allowlist, IP resolution — see validateFederationEndpoint) and
// returns a ready-to-use destination for it.
//
// The returned client:
//   - fails the whole request after timeout (redirects included),
//   - uses cluster.SSLVerify to decide whether to verify TLS certificates,
//   - revalidates every redirect Location with the same rules as the
//     original endpoint, aborting the request when a redirect points to a
//     destination that is not allowed.
func NewFederationTarget(endpoint string, cluster types.Cluster, allowlist *FederationAllowlist, timeout time.Duration) (*FederationTarget, error) {
	endpointURL, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return nil, fmt.Errorf("invalid federation endpoint %q: %v", endpoint, err)
	}
	if err := validateFederationEndpoint(endpoint, allowlist); err != nil {
		return nil, err
	}
	endpointURL.Path = path.Join(endpointURL.Path, "system", "services")
	if endpointURL.Path != "" && !strings.HasPrefix(endpointURL.Path, "/") {
		endpointURL.Path = "/" + endpointURL.Path
	}
	endpointURL.RawQuery = ""
	endpointURL.Fragment = ""
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: buildTLSConfig(cluster.SSLVerify),
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxFederationRedirects {
				return fmt.Errorf("federation request to %s exceeded %d redirects", via[0].URL, maxFederationRedirects)
			}
			if err := validateFederationEndpoint(req.URL.String(), allowlist); err != nil {
				log.Printf("federation request aborted: redirect to %s is not allowed: %v", req.URL, err)
				return fmt.Errorf("redirect to %s is not allowed: %v", req.URL, err)
			}
			return nil
		},
	}
	return &FederationTarget{
		baseURL:   endpointURL,
		Client:    client,
		cluster:   cluster,
		allowlist: allowlist,
	}, nil
}

// setFederationAuth sets the Authorization header used to talk to a
// federation destination, implementing the credential forwarding rule:
//
//   - destinations listed in the allowlist (allowed_federation_endpoints)
//     are trusted: when the caller carries an Authorization header, it is
//     forwarded to them unchanged,
//   - every other destination must never see the caller credentials: the
//     request carries only the cluster's own Basic authentication
//     (cluster.AuthUser/cluster.AuthPassword) when it is configured, and
//     none at all when it is not.
//
// The safe default, when no allowlist is configured, is therefore to never
// forward the caller credentials to any federation destination.
func setFederationAuth(req *http.Request, cluster *types.Cluster, allowlist *FederationAllowlist) {
	callerAuth := req.Header.Get("Authorization")
	req.Header.Del("Authorization")
	host := req.URL.Hostname()
	port := endpointPort(req.URL)
	if allowlist.allows(host, port) {
		if callerAuth != "" {
			req.Header.Set("Authorization", callerAuth)
			return
		}
	}
	if cluster != nil && (cluster.AuthUser != "" || cluster.AuthPassword != "") {
		req.SetBasicAuth(cluster.AuthUser, cluster.AuthPassword)
	}
}
