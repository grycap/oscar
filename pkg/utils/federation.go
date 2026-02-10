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
	"net/url"
	"path"
	"strings"

	"github.com/grycap/oscar/v3/pkg/types"
)

// ExpandFederation propagates service definitions to federation members.
func ExpandFederation(service *types.Service, authHeader string, method string) []error {
	if service == nil || service.Federation == nil || len(service.Federation.Members) == 0 {
		return nil
	}

	defaultGroupID(service)
	setReplicasFromMembers(service)

	var errs []error
	for _, member := range service.Federation.Members {
		worker, err := buildWorkerService(service, member)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		cluster, ok := service.Clusters[member.ClusterID]
		if !ok {
			errs = append(errs, fmt.Errorf("cluster \"%s\" not defined", member.ClusterID))
			continue
		}

		if err := sendFederatedService(worker, cluster, authHeader, method); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// VerifyFederationAuth checks that credentials are accepted by target clusters.
func VerifyFederationAuth(service *types.Service, authHeader string) []error {
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
		if err := checkFederatedAuth(member.ServiceName, cluster, authHeader); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// RollbackFederationCreate removes federated replicas created during initial deployment.
func RollbackFederationCreate(service *types.Service, authHeader string) []error {
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
		if err := sendFederatedDeleteService(member.ServiceName, cluster, authHeader); err != nil {
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
	setReplicasFromMembers(service)
}

func defaultGroupID(service *types.Service) {
	if service.Federation == nil {
		return
	}
	if strings.TrimSpace(service.Federation.GroupID) == "" {
		service.Federation.GroupID = service.Name
	}
}

func setReplicasFromMembers(service *types.Service) {
	if service.Federation == nil {
		return
	}
	if len(service.Federation.Members) == 0 {
		return
	}

	switch strings.ToLower(service.Federation.Topology) {
	case "mesh", "star":
		service.Replicas = append(types.ReplicaList{}, service.Federation.Members...)
	default:
	}
}

func buildWorkerService(service *types.Service, member types.Replica) (*types.Service, error) {
	worker := *service
	worker.Name = member.ServiceName
	worker.ClusterID = member.ClusterID
	worker.Federation = &types.Federation{
		GroupID:    service.Federation.GroupID,
		Topology:   service.Federation.Topology,
		Delegation: service.Federation.Delegation,
		Members:    nil,
	}
	worker.Clusters = stripClusterCredentials(service.Clusters)
	if service.ClusterID != "" {
		if worker.Annotations == nil {
			worker.Annotations = make(map[string]string)
		}
		worker.Annotations[types.OriginClusterAnnotation] = service.ClusterID
		worker.Annotations[types.OriginServiceAnnotation] = service.Name
	}

	switch strings.ToLower(service.Federation.Topology) {
	case "mesh":
		worker.Replicas = buildFederationMeshReplicas(service, member)
	case "star":
		worker.Replicas = nil
	default:
		worker.Replicas = nil
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

func sendFederatedService(service *types.Service, cluster types.Cluster, authHeader string, method string) error {
	endpoint := strings.TrimSpace(cluster.Endpoint)
	if endpoint == "" {
		return fmt.Errorf("empty cluster endpoint")
	}
	targetURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid cluster endpoint \"%s\": %v", endpoint, err)
	}
	targetURL.Path = path.Join(targetURL.Path, "system", "services")

	body, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to encode service payload: %v", err)
	}

	req, err := http.NewRequest(method, targetURL.String(), bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if strings.HasPrefix(authHeader, "Bearer ") {
		req.Header.Set("Authorization", authHeader)
	} else if strings.HasPrefix(authHeader, "Basic ") {
		req.Header.Set("Authorization", authHeader)
	} else if cluster.AuthUser != "" || cluster.AuthPassword != "" {
		req.SetBasicAuth(cluster.AuthUser, cluster.AuthPassword)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: buildTLSConfig(cluster.SSLVerify),
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

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

func checkFederatedAuth(serviceName string, cluster types.Cluster, authHeader string) error {
	endpoint := strings.TrimSpace(cluster.Endpoint)
	if endpoint == "" {
		return fmt.Errorf("empty cluster endpoint")
	}
	targetURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid cluster endpoint \"%s\": %v", endpoint, err)
	}
	targetURL.Path = path.Join(targetURL.Path, "system", "services", serviceName)

	req, err := http.NewRequest(http.MethodGet, targetURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	if strings.HasPrefix(authHeader, "Bearer ") || strings.HasPrefix(authHeader, "Basic ") {
		req.Header.Set("Authorization", authHeader)
	} else if cluster.AuthUser != "" || cluster.AuthPassword != "" {
		req.SetBasicAuth(cluster.AuthUser, cluster.AuthPassword)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: buildTLSConfig(cluster.SSLVerify),
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("auth check failed for cluster \"%s\": %v", cluster.Endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("auth rejected by cluster \"%s\" (status %d)", cluster.Endpoint, resp.StatusCode)
	}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("auth check failed for cluster \"%s\" (status %d)", cluster.Endpoint, resp.StatusCode)
	}
	return nil
}

func sendFederatedDeleteService(serviceName string, cluster types.Cluster, authHeader string) error {
	endpoint := strings.TrimSpace(cluster.Endpoint)
	if endpoint == "" {
		return fmt.Errorf("empty cluster endpoint")
	}
	targetURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid cluster endpoint \"%s\": %v", endpoint, err)
	}
	targetURL.Path = path.Join(targetURL.Path, "system", "services", serviceName)

	req, err := http.NewRequest(http.MethodDelete, targetURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	if strings.HasPrefix(authHeader, "Bearer ") || strings.HasPrefix(authHeader, "Basic ") {
		req.Header.Set("Authorization", authHeader)
	} else if cluster.AuthUser != "" || cluster.AuthPassword != "" {
		req.SetBasicAuth(cluster.AuthUser, cluster.AuthPassword)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: buildTLSConfig(cluster.SSLVerify),
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("rollback delete failed for cluster \"%s\": %v", cluster.Endpoint, err)
	}
	defer resp.Body.Close()

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
	return &tls.Config{InsecureSkipVerify: !verify}
}
