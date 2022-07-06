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

package resourcemanager

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/grycap/oscar/v2/pkg/types"
)

const (
	oscarReplicaType    = "oscar"
	endpointReplicaType = "endpoint"
)

// tokenCache map to store tokens from services and endpoints -> [CLUSTER_ENDPOINT][SERVICE_NAME]
var tokenCache = map[string]map[string]string{}

// cacheReads variable to store the number of accesses to tokenCache
// automatically clear the cache every 500 reads
var cacheReads = 0

// DelegatedEvent wraps the original input event by adding the storage provider ID
type DelegatedEvent struct {
	StorageProviderID string `json:"storage_provider"`
	Event             string `json:"event"`
}

// TODO: implement:
// get svc configMap (FDL), get service token in the replica, update event with "storage_provider" field
// read the service token from cache -> if is not valid or not available get it!
// DelegateJob sends the event to a service's replica
func DelegateJob(service *types.Service, event string) error {
	// Check if replicas are sorted by priority and sort it if needed
	if !sort.IsSorted(service.Replicas) {
		sort.Stable(service.Replicas)
	}

	delegatedEvent := WrapEvent(service.ClusterID, event)

	// TODO: implement!!
	for _, replica := range service.Replicas {
		// Manage if replica.Type is "oscar"
		if strings.ToLower(replica.Type) == oscarReplicaType {
			// Check ClusterID is defined in 'Clusters'
			cluster, ok := service.Clusters[replica.ClusterID]
			if !ok {
				log.Printf("Error delegating service \"%s\" to ClusterID \"%s\": Cluster not defined\n", service.Name, replica.ClusterID)
				continue
			}

			// Get token
			token, err := getServiceToken(replica, cluster)
			if err != nil {
				log.Printf("Error delegating service \"%s\" to ClusterID \"%s\": %v\n", service.Name, replica.ClusterID, err)
				continue
			}

			// TODO: make request...
		}

		// Manage if replica.Type is "endpoint"
		if strings.ToLower(replica.Type) == endpointReplicaType {
			// Check ClusterID is defined in 'Clusters'
		}
	}

	return fmt.Errorf("unable to delegate job from service \"%s\" to any replica", service.Name)
}

// WrapEvent wraps an event adding the storage_provider field (from the service's cluster_id)
func WrapEvent(providerID string, event string) DelegatedEvent {
	return DelegatedEvent{
		StorageProviderID: providerID,
		Event:             event,
	}
}

func getServiceToken(serviceName string, replica types.Replica, cluster types.Cluster) (string, error) {
	// Clear tokenCache if cacheReads > 500
	if cacheReads > 500 {
		tokenCache = map[string]map[string]string{}
	}

	endpoint := strings.Trim(cluster.Endpoint, " /")
	_, ok := tokenCache[endpoint]
	if ok {
		token, ok := tokenCache[endpoint][serviceName]
		if ok && token != "" {
			cacheReads++
			return token, nil
		}
	}

	return updateServiceToken(serviceName, replica, cluster)
}

func updateServiceToken(serviceName string, replica types.Replica, cluster types.Cluster) (string, error) {
	// Parse the cluster's endpoint URL and add the service's path
	getServiceURL, err := url.Parse(cluster.Endpoint)
	if err != nil {
		return "", fmt.Errorf("unable to parse cluster endpoint \"%s\": %v", cluster.Endpoint, err)
	}
	getServiceURL.Path = path.Join(getServiceURL.Path, "services", serviceName)

	// Make request to get service's definition (including token) from cluster
	req, err := http.NewRequest(http.MethodGet, getServiceURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("unable to make request to cluster endpoint \"%s\": %v", cluster.Endpoint, err)
	}

	// Add cluster's basic auth credentials
	req.SetBasicAuth(cluster.AuthUser, cluster.AuthPassword)

	// Send the request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("unable to send request to cluster endpoint \"%s\": %v", cluster.Endpoint, err)
	}

	// Check status code (OSCAR always returns)
	if res.StatusCode != 200 {
		return "", fmt.Errorf("error in response from cluster endpoint \"%s\": Status code %d", cluster.Endpoint, res.StatusCode)
	}

	// Decode the response body into the info struct
	var svc *types.Service
	err = json.NewDecoder(res.Body).Decode(&svc)
	if err != nil {
		return "", fmt.Errorf("error decoding service info from cluster endpoint \"%s\":%v", cluster.Endpoint, err)
	}

	// Update (or create) the service's token entry in tokenCache
	endpoint := strings.Trim(cluster.Endpoint, " /")
	_, ok := tokenCache[endpoint]
	if !ok {
		// Create empty map if nil
		tokenCache[endpoint] = map[string]string{}
	}
	tokenCache[endpoint][serviceName] = svc.Token

	return svc.Token, nil
}
