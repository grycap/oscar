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
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/grycap/oscar/v4/pkg/types"
)

// ExpandFederation propagates service definitions to federation members.
// Every member endpoint is validated against the (optional) allowlist before
// any traffic leaves the cluster, and the shared federation client applies
// the timeout, TLS and redirect rules plus the credential forwarding rule to
// each request.
func ExpandFederation(service *types.Service, authHeader string, method string, refreshToken string, allowlist *FederationAllowlist) []error {
	if service == nil || service.Federation == nil || len(service.Federation.Members) == 0 {
		return nil
	}

	defaultGroupID(service)

	var errs []error
	for _, member := range service.Federation.Members {
		worker, err := buildWorkerService(service, member, refreshToken)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		cluster, ok := service.Clusters[member.ClusterID]
		if !ok {
			errs = append(errs, fmt.Errorf("cluster \"%s\" not defined", member.ClusterID))
			continue
		}

		if err := sendFederatedService(worker, cluster, authHeader, method, allowlist); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// VerifyFederationAuth checks that credentials are accepted by target
// clusters. Non-oscar members are skipped; oscar member endpoints are
// validated against the allowlist before any request is sent.
func VerifyFederationAuth(service *types.Service, authHeader string, allowlist *FederationAllowlist) []error {
	if service == nil || service.Federation == nil || len(service.Federation.Members) == 0 {
		return nil
	}

	var errs []error
	for _, member := range service.Federation.Members {
		if strings.ToLower(member.Type) != "oscar" {
			continue
		}
		cluster, ok := service.Clusters[member.ClusterID]
		if !ok {
			errs = append(errs, fmt.Errorf("cluster \"%s\" not defined", member.ClusterID))
			continue
		}
		if err := checkFederatedAuth(member.ServiceName, cluster, authHeader, allowlist); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// RollbackFederationCreate removes federated replicas created during initial
// deployment. Non-oscar members are skipped; oscar member endpoints are
// validated against the allowlist before any request is sent.
func RollbackFederationCreate(service *types.Service, authHeader string, allowlist *FederationAllowlist) []error {
	if service == nil || service.Federation == nil || len(service.Federation.Members) == 0 {
		return nil
	}

	var errs []error
	for _, member := range service.Federation.Members {
		if strings.ToLower(member.Type) != "oscar" {
			continue
		}
		cluster, ok := service.Clusters[member.ClusterID]
		if !ok {
			errs = append(errs, fmt.Errorf("cluster \"%s\" not defined", member.ClusterID))
			continue
		}
		if err := sendFederatedDeleteService(member.ServiceName, cluster, authHeader, allowlist); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// ApplyFederation updates the local service definition based on federation members.
func ApplyFederation(service *types.Service) {
	if service == nil || service.Federation == nil {
		return
	}
	defaultGroupID(service)
}

func defaultGroupID(service *types.Service) {
	if service.Federation == nil {
		return
	}
	if strings.TrimSpace(service.Federation.GroupID) == "" {
		service.Federation.GroupID = service.Name
	}
}

func buildWorkerService(service *types.Service, member types.Replica, refreshToken string) (*types.Service, error) {
	worker := *service
	// Namespace is local to the origin; the target assigns its own user namespace.
	worker.Namespace = ""
	worker.Name = member.ServiceName
	worker.ClusterID = member.ClusterID
	worker.Federation = &types.Federation{
		GroupID:              service.Federation.GroupID,
		Topology:             service.Federation.Topology,
		Delegation:           service.Federation.Delegation,
		ReschedulerThreshold: service.Federation.ReschedulerThreshold,
		Members:              nil,
	}
	worker.Clusters = stripClusterCredentials(service.Clusters)
	if service.ClusterID != "" {
		if worker.Annotations == nil {
			worker.Annotations = make(map[string]string)
		}
		worker.Annotations[types.OriginClusterAnnotation] = service.ClusterID
		worker.Annotations[types.OriginServiceAnnotation] = service.Name
		worker.Annotations[types.FederationWorkerAnnotation] = "true"
	}
	if refreshToken != "" && strings.EqualFold(service.Federation.Topology, "mesh") {
		if worker.Environment.Secrets == nil {
			worker.Environment.Secrets = map[string]string{}
		}
		worker.Environment.Secrets[types.RefreshTokenSecretKey] = refreshToken
	}

	switch strings.ToLower(service.Federation.Topology) {
	case "mesh":
		worker.Federation.Members = buildFederationMeshReplicas(service, member)
	case "star":
		worker.Federation.Members = nil
	default:
		worker.Federation.Members = nil
	}

	return &worker, nil
}

func buildFederationMeshReplicas(service *types.Service, member types.Replica) types.ReplicaList {
	replicas := types.ReplicaList{
		{
			Type:        "oscar",
			ClusterID:   service.ClusterID,
			ServiceName: service.Name,
			Priority:    0,
		},
	}

	for _, candidate := range service.Federation.Members {
		if candidate.ClusterID == member.ClusterID && candidate.ServiceName == member.ServiceName {
			continue
		}
		replicas = append(replicas, candidate)
	}

	return replicas
}

func stripClusterCredentials(clusters map[string]types.Cluster) map[string]types.Cluster {
	if clusters == nil {
		return nil
	}
	clean := make(map[string]types.Cluster, len(clusters))
	for id, cluster := range clusters {
		cluster.AuthUser = ""
		cluster.AuthPassword = ""
		clean[id] = cluster
	}
	return clean
}

// setCallerFederationAuth carries the caller's Authorization header on the
// outbound request. The federation target applies the credential forwarding
// rule (see setFederationAuth) before sending: Bearer/Basic caller tokens are
// only reached by allowlisted destinations, every other destination either
// receives the cluster's own Basic authentication or none at all.
func setCallerFederationAuth(req *http.Request, authHeader string) {
	if strings.HasPrefix(authHeader, "Bearer ") || strings.HasPrefix(authHeader, "Basic ") {
		req.Header.Set("Authorization", authHeader)
	}
}

// sendFederatedService sends the (stripped) service definition to the
// federation collection endpoint of cluster with the given method (POST or
// PUT). The endpoint is validated against the allowlist before the request is
// built.
func sendFederatedService(service *types.Service, cluster types.Cluster, authHeader string, method string, allowlist *FederationAllowlist) error {
	target, err := NewFederationTarget(cluster.Endpoint, cluster, allowlist, federationRequestTimeout)
	if err != nil {
		return err
	}

	body, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to encode service payload: %v", err)
	}

	req, err := http.NewRequest(method, target.url(), bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	setCallerFederationAuth(req, authHeader)

	resp, err := target.do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(body))
		if msg != "" {
			return fmt.Errorf("cluster responded with status %d: %s", resp.StatusCode, msg)
		}
		return fmt.Errorf("cluster responded with status %d", resp.StatusCode)
	}
	return nil
}

// checkFederatedAuth performs the pre-creation authorization check against an
// oscar member endpoint: a read of the member service definition. Only
// non-authorized (401/403) and server errors fail the check; the endpoint is
// validated against the allowlist before the request is built.
func checkFederatedAuth(serviceName string, cluster types.Cluster, authHeader string, allowlist *FederationAllowlist) error {
	target, err := NewFederationTarget(cluster.Endpoint, cluster, allowlist, federationAuthCheckTimeout)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodGet, target.url(serviceName), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	setCallerFederationAuth(req, authHeader)

	resp, err := target.do(req)
	if err != nil {
		return fmt.Errorf("auth check failed for cluster \"%s\": %v", cluster.Endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("auth rejected by cluster \"%s\" (status %d)", cluster.Endpoint, resp.StatusCode)
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("auth check failed for cluster \"%s\" (status %d)", cluster.Endpoint, resp.StatusCode)
	}
	return nil
}

// sendFederatedDeleteService deletes the member service definition on an
// oscar member endpoint during create rollback; a 404 is considered
// success (nothing to roll back). The endpoint is validated against the
// allowlist before the request is built.
func sendFederatedDeleteService(serviceName string, cluster types.Cluster, authHeader string, allowlist *FederationAllowlist) error {
	target, err := NewFederationTarget(cluster.Endpoint, cluster, allowlist, federationRequestTimeout)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodDelete, target.url(serviceName), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	setCallerFederationAuth(req, authHeader)

	resp, err := target.do(req)
	if err != nil {
		return fmt.Errorf("rollback delete failed for cluster \"%s\": %v", cluster.Endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(body))
		if msg != "" {
			return fmt.Errorf("rollback delete failed for cluster \"%s\" (status %d): %s", cluster.Endpoint, resp.StatusCode, msg)
		}
		return fmt.Errorf("rollback delete failed for cluster \"%s\" (status %d)", cluster.Endpoint, resp.StatusCode)
	}
	return nil
}

func buildTLSConfig(verify bool) *tls.Config {
	return &tls.Config{InsecureSkipVerify: !verify} // #nosec G402
}
