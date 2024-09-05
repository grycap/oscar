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
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grycap/oscar/v3/pkg/types"
)

const (
	oscarReplicaType    = "oscar"
	endpointReplicaType = "endpoint"
)

// tokenCache map to store tokens from services and endpoints -> [CLUSTER_ENDPOINT][SERVICE_NAME]
var tokenCache = map[string]map[string]string{}

// DelegatedEvent wraps the original input event by adding the storage provider ID
type DelegatedEvent struct {
	StorageProviderID string `json:"storage_provider"`
	Event             string `json:"event"`
}

type GeneralInfo struct {
	NumberNodes     int64      `json:"numberNodes"`
	CPUFreeTotal    int64      `json:"cpuFreeTotal"`
	CPUMaxFree      int64      `json:"cpuMaxFree"`
	MemoryFreeTotal int64      `json:"memoryFreeTotal"`
	MemoryMaxFree   int64      `json:"memoryMaxFree"`
	DetailsNodes    []NodeInfo `json:"detail"`
}

type NodeInfo struct {
	NodeName         string `json:"nodeName"`
	CPUCapacity      string `json:"cpuCapacity"`
	CPUUsage         string `json:"cpuUsage"`
	CPUPercentage    string `json:"cpuPercentage"`
	MemoryCapacity   string `json:"memoryCapacity"`
	MemoryUsage      string `json:"memoryUsage"`
	MemoryPercentage string `json:"memoryPercentage"`
}

// DelegateJob sends the event to a service's replica
func DelegateJob(service *types.Service, event string, logger *log.Logger) error {
	//Determine priority level of each replica to delegate
	getClusterStatus(service)

	// Check if replicas are sorted by priority and sort it if needed
	if !sort.IsSorted(service.Replicas) {
		sort.Stable(service.Replicas)
	}

	delegatedEvent := WrapEvent(service.ClusterID, event)
	eventJSON, err := json.Marshal(delegatedEvent)
	if err != nil {
		return fmt.Errorf("error marshalling delegated event: %v", err)
	}

	for _, replica := range service.Replicas {
		// Manage if replica.Type is "oscar" and have the capacity to receive a service
		if strings.ToLower(replica.Type) == oscarReplicaType && replica.Priority < 101 {
			// Check ClusterID is defined in 'Clusters'
			cluster, ok := service.Clusters[replica.ClusterID]
			if !ok {
				logger.Printf("Error delegating service \"%s\" to ClusterID \"%s\": Cluster not defined\n", service.Name, replica.ClusterID)
				continue
			}

			// Get token
			token, err := getServiceToken(replica, cluster)
			if err != nil {
				logger.Printf("Error delegating job from service \"%s\" to ClusterID \"%s\": %v\n", service.Name, replica.ClusterID, err)
				continue
			}

			// Parse the cluster's endpoint URL and add the service's path
			postJobURL, err := url.Parse(cluster.Endpoint)
			if err != nil {
				logger.Printf("Error delegating job from service \"%s\" to ClusterID \"%s\": unable to parse cluster endpoint \"%s\": %v\n", service.Name, replica.ClusterID, cluster.Endpoint, err)
				continue
			}
			postJobURL.Path = path.Join(postJobURL.Path, "job", replica.ServiceName)

			// Make request to get service's definition (including token) from cluster
			req, err := http.NewRequest(http.MethodPost, postJobURL.String(), bytes.NewBuffer(eventJSON))
			if err != nil {
				logger.Printf("Error delegating job from service \"%s\" to ClusterID \"%s\": unable to make request: %v\n", service.Name, replica.ClusterID, err)
				continue
			}

			// Add Headers
			for k, v := range replica.Headers {
				req.Header.Add(k, v)
			}

			// Add service token to the request
			req.Header.Add("Authorization", "Bearer "+strings.TrimSpace(token))

			// Make HTTP client
			var transport http.RoundTripper = &http.Transport{
				// Enable/disable SSL verification
				TLSClientConfig: &tls.Config{InsecureSkipVerify: !cluster.SSLVerify},
			}
			client := &http.Client{
				Transport: transport,
				Timeout:   time.Second * 20,
			}

			// Send the request
			res, err := client.Do(req)
			if err != nil {
				logger.Printf("Error delegating job from service \"%s\" to ClusterID \"%s\": unable to send request: %v\n", service.Name, replica.ClusterID, err)
				continue
			}

			// Check status code
			if res.StatusCode == http.StatusCreated {
				logger.Printf("Job successfully delegated to cluster \"%s\"\n", replica.ClusterID)
				return nil
			} else if res.StatusCode == http.StatusUnauthorized {
				// Retry updating the token
				token, err := updateServiceToken(replica, cluster)
				if err != nil {
					logger.Printf("Error delegating job from service \"%s\" to ClusterID \"%s\": %v\n", service.Name, replica.ClusterID, err)
					continue
				}
				// Add service token to the request
				req.Header.Add("Authorization", "Bearer "+strings.TrimSpace(token))

				// Send the request
				res, err = client.Do(req)
				if err != nil {
					logger.Printf("Error delegating job from service \"%s\" to ClusterID \"%s\": unable to send request: %v\n", service.Name, replica.ClusterID, err)
					continue
				}
			}
			log.Printf("Error delegating job from service \"%s\" to ClusterID \"%s\": Status code %d\n", service.Name, replica.ClusterID, res.StatusCode)
		}

		// Manage if replica.Type is "endpoint"
		if strings.ToLower(replica.Type) == endpointReplicaType {
			// Parse the replica URL to check if it's valid
			replicaURL, err := url.Parse(replica.URL)
			if err != nil {
				logger.Printf("Error delegating job from service \"%s\" to endpoint \"%s\": unable to parse URL: %v\n", service.Name, replica.URL, err)
				continue
			}

			// Make request to get service's definition (including token) from cluster
			req, err := http.NewRequest(http.MethodPost, replicaURL.String(), bytes.NewBuffer(eventJSON))
			if err != nil {
				logger.Printf("Error delegating job from service \"%s\" to endpoint \"%s\": unable to make request: %v\n", service.Name, replica.URL, err)
				continue
			}

			// Add Headers
			for k, v := range replica.Headers {
				req.Header.Add(k, v)
			}

			// Make HTTP client
			var transport http.RoundTripper = &http.Transport{
				// Enable/disable SSL verification
				TLSClientConfig: &tls.Config{InsecureSkipVerify: !replica.SSLVerify},
			}
			client := &http.Client{
				Transport: transport,
				Timeout:   time.Second * 20,
			}

			// Send the request
			res, err := client.Do(req)
			if err != nil {
				logger.Printf("Error delegating job from service \"%s\" to endpoint \"%s\": unable to send request: %v\n", service.Name, replica.URL, err)
				continue
			}

			// Check status code
			if res.StatusCode == http.StatusOK {
				logger.Printf("Job successfully delegated to endpoint \"%s\"\n", replica.URL)
				return nil
			}
			logger.Printf("Error delegating job from service \"%s\" to endpoint \"%s\": Status code %d\n", service.Name, replica.URL, res.StatusCode)
		}
	}

	return fmt.Errorf("unable to delegate job from service \"%s\" to any replica, scheduling in the current cluster", service.Name)
}

// WrapEvent wraps an event adding the storage_provider field (from the service's cluster_id)
func WrapEvent(providerID string, event string) DelegatedEvent {
	return DelegatedEvent{
		StorageProviderID: providerID,
		Event:             event,
	}
}

func getServiceToken(replica types.Replica, cluster types.Cluster) (string, error) {
	endpoint := strings.Trim(cluster.Endpoint, " /")
	_, ok := tokenCache[endpoint]
	if ok {
		token, ok := tokenCache[endpoint][replica.ServiceName]
		if ok && token != "" {
			return token, nil
		}
	}

	return updateServiceToken(replica, cluster)
}

func updateServiceToken(replica types.Replica, cluster types.Cluster) (string, error) {
	// Clear tokenCache if there are more than 500 tokens stored
	length := 0
	for _, subMap := range tokenCache {
		length += len(subMap)
	}
	if length > 500 {
		tokenCache = map[string]map[string]string{}
	}

	// Parse the cluster's endpoint URL and add the service's path
	getServiceURL, err := url.Parse(cluster.Endpoint)
	if err != nil {
		return "", fmt.Errorf("unable to parse cluster endpoint \"%s\": %v", cluster.Endpoint, err)
	}
	getServiceURL.Path = path.Join(getServiceURL.Path, "system", "services", replica.ServiceName)

	// Make request to get service's definition (including token) from cluster
	req, err := http.NewRequest(http.MethodGet, getServiceURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("unable to make request to cluster endpoint \"%s\": %v", cluster.Endpoint, err)
	}

	// Add cluster's basic auth credentials
	req.SetBasicAuth(cluster.AuthUser, cluster.AuthPassword)

	// Make HTTP client
	var transport http.RoundTripper = &http.Transport{
		// Enable/disable SSL verification
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !cluster.SSLVerify},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   time.Second * 20,
	}

	// Send the request
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("unable to send request to cluster endpoint \"%s\": %v", cluster.Endpoint, err)
	}

	// Check status code (OSCAR always returns 200 if it's OK)
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
	tokenCache[endpoint][replica.ServiceName] = svc.Token

	return svc.Token, nil
}

func getClusterStatus(service *types.Service) {

	for _, replica := range service.Replicas {
		// Manage if replica.Type is "oscar"
		if strings.ToLower(replica.Type) == oscarReplicaType {
			// Check ClusterID is defined in 'Clusters'
			cluster, ok := service.Clusters[replica.ClusterID]
			if !ok {
				replica.Priority = 101
				fmt.Printf("Error checking to ClusterID \"%s\": Cluster not defined\n", replica.ClusterID)
				continue
			}
			// Parse the cluster's endpoint URL and add the service's path
			getStatusURL, err := url.Parse(cluster.Endpoint)
			if err != nil {
				replica.Priority = 101
				fmt.Printf("Error parsing the cluster's endpoint URL to ClusterID \"%s\": unable to parse cluster endpoint \"%s\": %v\n", replica.ClusterID, cluster.Endpoint, err)
				continue
			}
			getStatusURL.Path = path.Join(getStatusURL.Path, "system", "status")

			// Make request to get status from cluster
			req, err := http.NewRequest(http.MethodGet, getStatusURL.String(), nil)
			if err != nil {
				replica.Priority = 101
				fmt.Printf("Error making request to ClusterID \"%s\": unable to make request: %v\n", replica.ClusterID, err)
				continue
			}
			// Add cluster's basic auth credentials
			req.SetBasicAuth(cluster.AuthUser, cluster.AuthPassword)

			// Make HTTP client
			var transport http.RoundTripper = &http.Transport{
				// Enable/disable SSL verification
				TLSClientConfig: &tls.Config{InsecureSkipVerify: !cluster.SSLVerify},
			}
			client := &http.Client{
				Transport: transport,
				Timeout:   time.Second * 20,
			}

			// Send the request
			res, err := client.Do(req)
			if err != nil {
				replica.Priority = 101
				fmt.Printf("Error getting cluster status to ClusterID \"%s\": unable to send request: %v\n", replica.ClusterID, err)
				continue
			}

			// Check status code
			if res.StatusCode == http.StatusOK {
				fmt.Printf("Successful get of cluster status to ClusterID\"%s\"\n", replica.ClusterID)

				//Convert cluster status response to JSON
				var clusterStatus *GeneralInfo
				err = json.NewDecoder(res.Body).Decode(&clusterStatus)
				if err != nil {
					replica.Priority = 101
					fmt.Println("Error decoding the JSON of the response:", err)
					continue
				}

				// CPU is in miliCPU
				// CPU required to deploy the service
				serviceCPU, err := strconv.ParseFloat(service.CPU, 64)
				if err != nil {
					replica.Priority = 101
					fmt.Println("Error to converter CPU of service to int: ", err)
					continue
				}

				maxNodeCPU := float64(clusterStatus.CPUMaxFree)

				//Calculate CPU difference to determine whether to delegate a replica to the cluster
				dist := maxNodeCPU - (1000 * serviceCPU)

				//The priority of delegating the service is set based on the free CPU of the cluster as long as it has free CPU on a node to delegate the service.
				if dist >= 0 {

					if service.Delegation == "random" {
						randPriority := rand.Intn(101)
						replica.Priority = uint(randPriority)
					} else if service.Delegation == "load-based" {
						//Map the totalClusterCPU range to a smaller range (input range 0 to 32 cpu to output range 100 to 0 priority)
						totalClusterCPU := clusterStatus.CPUFreeTotal
						mappedCPUPriority := mapToRange(totalClusterCPU, 0, 32000, 100, 0)
						replica.Priority = uint(mappedCPUPriority)
					} else if service.Delegation == "static" {
						if replica.Priority > 100 {
							replica.Priority = 101
						}
					} else {
						replica.Priority = 101
						fmt.Println("Error when declaring the type of delegation in ClusterID ", replica.ClusterID)
						continue
					}
				} else {
					fmt.Println("No CPU capacity to delegate job in ClusterID ", replica.ClusterID)
					replica.Priority = 101
					continue
				}
				fmt.Println(clusterStatus)

			} else {
				replica.Priority = 101
				fmt.Printf("Error to get of cluster status to ClusterID\"%s\"\n", replica.ClusterID)
			}

		}
	}

}

func mapToRange(value, minInput, maxInput, maxOutput, minOutput int64) int {
	mappedValue := maxOutput - (maxOutput-minOutput)*(value-minInput)/(maxInput-minInput)
	mappedInt := int(mappedValue)
	if mappedInt > int(maxOutput) {
		mappedInt = int(maxOutput)
	}
	if mappedInt < int(minOutput) {
		mappedInt = int(minOutput)
	}

	return mappedInt
}
