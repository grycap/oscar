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
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"

	//"sync"
	"time"

	"github.com/grycap/oscar/v3/pkg/types"
)

const (
	oscarReplicaType    = "oscar"
	endpointReplicaType = "endpoint"
	noDelegateCode      = 101
)

// tokenCache map to store tokens from services and endpoints -> [CLUSTER_ENDPOINT][SERVICE_NAME]
var tokenCache = map[string]map[string]string{}

//var mutex sync.Mutex

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

type Alternative struct {
	Index      int     // Número de la alternativa
	Preference float64 // Valor de la preferencia
}

type JobStatus struct {
	Status       string `json:"status"`
	CreationTime string `json:"creation_time"`
	StartTime    string `json:"start_time"`
	FinishTime   string `json:"finish_time"`
}
type JobStatuses map[string]JobStatus

// Function to execute TOPSIS method
// Normalizes a column by dividing each value by the square root of the sum of squares.
func normalizeMatrix(matrix [][]float64) [][]float64 {
	rows := len(matrix)
	cols := len(matrix[0])
	normalized := make([][]float64, rows)
	for i := range normalized {
		normalized[i] = make([]float64, cols)
	}

	for j := 0; j < cols; j++ {
		// Calculate the norm (square root of the sum of squares of the column)
		add := 0.0
		for i := 0; i < rows; i++ {
			add += matrix[i][j] * matrix[i][j]
		}
		norm := math.Sqrt(add)
		// Normalize the values ​​of the column
		for i := 0; i < rows; i++ {
			normalized[i][j] = matrix[i][j] / norm
		}
	}
	return normalized
}

// Multiply the normalized matrix by the weights.
func weightMatrix(matrix [][]float64, weight []float64) [][]float64 {
	rows := len(matrix)
	cols := len(matrix[0])
	weighted := make([][]float64, rows)
	for i := range weighted {
		weighted[i] = make([]float64, cols)
		for j := 0; j < cols; j++ {
			weighted[i][j] = matrix[i][j] * weight[j]
		}
	}
	return weighted
}

// Calculate the ideal and anti-ideal solutions.
func calculateSolutions(matrix [][]float64) (ideal []float64, antiIdeal []float64) {
	rows := len(matrix)
	cols := len(matrix[0])

	ideal = make([]float64, cols)
	antiIdeal = make([]float64, cols)

	for j := 0; j < cols; j++ {
		// If the criterion is minimization (let's assume that the first criterion is the one we want to minimize)

		if j == 0 || j == 4 || j == 5 {
			// For the ideal solution, we select the minimum value (instead of the maximum)
			ideal[j] = matrix[0][j]
			antiIdeal[j] = matrix[0][j]
			for i := 0; i < rows; i++ {
				if matrix[i][j] < ideal[j] {
					ideal[j] = matrix[i][j]
				}
				if matrix[i][j] > antiIdeal[j] {
					antiIdeal[j] = matrix[i][j]
				}
			}
		} else {
			// For maximization criteria, we normally use the maximum and minimum values
			ideal[j] = matrix[0][j]
			antiIdeal[j] = matrix[0][j]
			for i := 0; i < rows; i++ {
				if matrix[i][j] > ideal[j] {
					ideal[j] = matrix[i][j]
				}
				if matrix[i][j] < antiIdeal[j] {
					antiIdeal[j] = matrix[i][j]
				}
			}
		}
	}
	return ideal, antiIdeal
}

// Calculate the Euclidean distance between an alternative and the ideal or anti-ideal solution
func calculateDistance(alternative []float64, solution []float64) float64 {
	add := 0.0
	for i := 0; i < len(alternative); i++ {
		add += (alternative[i] - solution[i]) * (alternative[i] - solution[i])
	}
	return math.Sqrt(add)
}

// Calculate the preference index for each alternative
func calculatePreferences(matrix [][]float64, ideal []float64, antiIdeal []float64) []float64 {
	rows := len(matrix)
	preferences := make([]float64, rows)

	for i := 0; i < rows; i++ {
		distanceIdeal := calculateDistance(matrix[i], ideal)
		distanceAntiIdeal := calculateDistance(matrix[i], antiIdeal)
		preferences[i] = distanceAntiIdeal / (distanceIdeal + distanceAntiIdeal)
	}
	return preferences
}

// Order the alternatives from best to worst according to the preference index
func sortAlternatives(preferences []float64) []Alternative {
	alternatives := make([]Alternative, len(preferences))

	// Create a list of alternatives with their preference indices
	for i := 0; i < len(preferences); i++ {
		alternatives[i] = Alternative{
			Index:      i + 1, // Alternativa 1, 2, etc.
			Preference: preferences[i],
		}
	}

	// Sort the alternatives in descending order of preference
	sort.Slice(alternatives, func(i, j int) bool {
		return alternatives[i].Preference > alternatives[j].Preference
	})

	return alternatives
}

func distancesFromBetter(alternatives []Alternative) []float64 {
	distances := make([]float64, len(alternatives)-1)

	// Calculate distances with the first element
	for i := 1; i < len(alternatives); i++ {
		distances[i-1] = math.Abs(alternatives[0].Preference - alternatives[i].Preference)
	}

	return distances
}

// Function to randomly rearrange elements whose distance from the first is less than a threshold, including the first
func reorganizeIfNearby(alternatives []Alternative, distances []float64, threshold float64) []Alternative {

	// List of nearby elements (with distance less than the threshold, including the first element)
	nearby := []Alternative{alternatives[0]}

	// Identify the other nearby elements
	for i := 0; i < len(distances); i++ {
		if distances[i] < threshold {
			nearby = append(nearby, alternatives[i+1])
		}
	}

	// Randomly shuffle nearby items
	//rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(nearby), func(i, j int) {
		nearby[i], nearby[j] = nearby[j], nearby[i]
	})

	// Create a new reorganized alternative list
	newAlternatives := []Alternative{}
	j := 0

	// Insert the rearranged or unarranged elements
	for i := 0; i < len(alternatives); i++ {
		if i == 0 || distances[i-1] < threshold {
			newAlternatives = append(newAlternatives, nearby[j]) // Add the rearranged items
			j++
		} else {
			newAlternatives = append(newAlternatives, alternatives[i]) // Keep non-close elements
		}
	}

	return newAlternatives
}

// DelegateJob sends the event to a service's replica
func DelegateJob(service *types.Service, event string, logger *log.Logger) error {

	//Block access before executing the function
	//mutex.Lock()
	//defer mutex.Unlock()

	//Determine priority level of each replica to delegate
	if service.Delegation == "topsis" {
		results := [][]float64{}
		//ServiceCPU, err := strconv.ParseFloat(service.CPU, 64)
		//if err != nil {
		//	fmt.Println("Error to converter CPU of service to int: ", err)
		//	continue
		//}
		for _, cred := range service.Replicas {

			cluster, ok := service.Clusters[cred.ClusterID]
			if !ok {
				//logger.Printf("Error delegating service \"%s\" to ClusterID \"%s\": Cluster not defined\n", service.Name, replica.ClusterID)
				continue
			}

			// Get token
			token, err := getServiceToken(cred, cluster)
			if err != nil {
				//logger.Printf("Error delegating job from service \"%s\" to ClusterID \"%s\": %v\n", service.Name, replica.ClusterID, err)
				continue
			}

			// Parse the cluster's endpoint URL and add the service's path
			JobURL, err := url.Parse(cluster.Endpoint)
			if err != nil {
				//logger.Printf("Error delegating job from service \"%s\" to ClusterID \"%s\": unable to parse cluster endpoint \"%s\": %v\n", service.Name, replica.ClusterID, cluster.Endpoint, err)
				continue
			}
			JobURL.Path = path.Join(JobURL.Path, "/system/logs/", cred.ServiceName)

			// Make request to get service's definition (including token) from cluster
			req, err := http.NewRequest("GET", JobURL.String(), nil)
			if err != nil {
				//logger.Printf("Error delegating job from service \"%s\" to ClusterID \"%s\": unable to make request: %v\n", service.Name, replica.ClusterID, err)
				continue
			}

			// Add Headers
			for k, v := range cred.Headers {
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
			resp, err := client.Do(req)
			if err != nil {
				//logger.Printf("Error delegating job from service \"%s\" to ClusterID \"%s\": unable to send request: %v\n", service.Name, replica.ClusterID, err)
				continue
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body) // Utilizar io.ReadAll para leer el cuerpo
			if err != nil {
				fmt.Printf("Error al leer el cuerpo de la respuesta para %s: %v\n", cred.URL, err)
				results = append(results, []float64{20, 0, 0, 0, 1e6, 1e6})
				continue
			}
			var jobStatuses JobStatuses
			err = json.Unmarshal(body, &jobStatuses)
			if err != nil {
				fmt.Println("Error decoding the JSON of the response:", err)
				results = append(results, []float64{20, 0, 0, 0, 1e6, 1e6})
				continue
			}

			// Show job statuses

			// Count job statuses
			averageExecutionTime, pendingCount := countJobs(jobStatuses)

			JobURL, err = url.Parse(cluster.Endpoint)
			if err != nil {
				//logger.Printf("Error delegating job from service \"%s\" to ClusterID \"%s\": unable to parse cluster endpoint \"%s\": %v\n", service.Name, replica.ClusterID, cluster.Endpoint, err)
				continue
			}
			JobURL.Path = path.Join(JobURL.Path, "/system/status/")
			req1, err := http.NewRequest("GET", JobURL.String(), nil)

			if err != nil {
				fmt.Printf("Error creating request for %s: %v\n", cred.URL, err)
				results = append(results, []float64{20, 0, 0, 0, 1e6, 1e6})
				continue
			}

			// Add Headers
			for k, v := range cred.Headers {
				req.Header.Add(k, v)
			}

			// Add service token to the request
			req.Header.Add("Authorization", "Bearer "+strings.TrimSpace(token))

			// Make the HTTP request
			start := time.Now()
			resp1, err := client.Do(req1)
			duration := time.Since(start)
			if err != nil {
				//fmt.Printf("Error making request for %s: %v\n", cred.URL, err)
				results = append(results, []float64{duration.Seconds(), 0, 0, 0, 1e6, 1e6})
				continue
			}

			defer resp1.Body.Close()
			var clusterStatus GeneralInfo
			err = json.NewDecoder(resp1.Body).Decode(&clusterStatus)
			if err != nil {
				fmt.Println("Error decoding the JSON of the response:", err)
				results = append(results, []float64{duration.Seconds(), 0, 0, 0, 1e6, 1e6})
				continue
			}

			serviceCPU, err := strconv.ParseFloat(service.CPU, 64)

			if err != nil {
				fmt.Println("Error converting service CPU to float: ", err)
				results = append(results, []float64{duration.Seconds(), 0, 0, 0, 1e6, 1e6})
				continue
			}

			results = createParameters(results, duration, clusterStatus, serviceCPU, averageExecutionTime, float64(pendingCount))

		}
		// Print results as a matrix
		fmt.Println("Results matrix:")
		for _, row := range results {
			fmt.Println(row)
		}

		// Criteria weights ()
		weight := []float64{1, 8, 18, 65, 2, 6}
		preferences := topsisMethod(results, weight)
		newAlternatives := sortbyThreshold(preferences, 20)

		// Print reordered alternatives
		fmt.Println("\nAlternatives reordered by threshold:")
		for _, alt := range newAlternatives {
			fmt.Printf("Alternative %d: %f\n", alt.Index, alt.Preference)
			service.Replicas[alt.Index].Priority = uint(alt.Preference)
		}
		fmt.Println("Replicas stable to topsis method: ", service.Replicas)

		//fmt.Println("Priority ", service.Replicas[id].Priority, " with ", service.Delegation, " delegation")
	} else {
		getClusterStatus(service)
		fmt.Println("Replicas: ", service.Replicas)

		// Check if replicas are sorted by priority and sort it if needed
		if !sort.IsSorted(service.Replicas) {
			sort.Stable(service.Replicas)
			fmt.Println("Replicas Stable: ", service.Replicas)
		}
	}

	fmt.Println("Event : ", event)

	/*delegatedEvent := WrapEvent(service.ClusterID, event)

	eventJSON, err := json.Marshal(delegatedEvent)
	if err != nil {
		return fmt.Errorf("error marshalling delegated event: %v", err)
	}*/

	storage_provider := service.ClusterID
	//Create event depending on delegation level

	eventJSON, provider := eventBuild(event, storage_provider)
	fmt.Println("Storage_provider :  ", provider)
	fmt.Println(string(eventJSON))

	//if provider != "" { //storatage _provider not empty in the delegation proccess.

	for _, replica := range service.Replicas {
		// Manage if replica.Type is "oscar" and have the capacity to receive a service
		fmt.Println("Delegation job in ClusterID: ", replica.ClusterID, "with Priority ", replica.Priority)
		if strings.ToLower(replica.Type) == oscarReplicaType && replica.Priority < noDelegateCode {
			// Check ClusterID is defined in 'Clusters'
			fmt.Println("Delegating ...")
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
			//fmt.Println(string(eventJSON))
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
	//} else {
	//	fmt.Println("Error by Storage_Provider empty.")
	//}

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
	fmt.Println("Process to getClusterStatus function.")
	for id, replica := range service.Replicas {
		// Manage if replica.Type is "oscar"
		if strings.ToLower(replica.Type) == oscarReplicaType {
			// Check ClusterID is defined in 'Clusters'
			cluster, ok := service.Clusters[replica.ClusterID]
			if !ok {
				if service.Delegation != "static" {
					service.Replicas[id].Priority = noDelegateCode
					//replica.Priority = noDelegateCode
				}
				fmt.Printf("Error checking to ClusterID \"%s\": Cluster not defined\n", replica.ClusterID)
				continue
			}
			// Parse the cluster's endpoint URL and add the service's path
			getStatusURL, err := url.Parse(cluster.Endpoint)
			if err != nil {
				if service.Delegation != "static" {
					service.Replicas[id].Priority = noDelegateCode
					//replica.Priority = noDelegateCode
				}
				fmt.Printf("Error parsing the cluster's endpoint URL to ClusterID \"%s\": unable to parse cluster endpoint \"%s\": %v\n", replica.ClusterID, cluster.Endpoint, err)
				continue
			}
			getStatusURL.Path = path.Join(getStatusURL.Path, "system", "status")

			// Make request to get status from cluster
			req, err := http.NewRequest(http.MethodGet, getStatusURL.String(), nil)
			if err != nil {
				if service.Delegation != "static" {
					service.Replicas[id].Priority = noDelegateCode
					//replica.Priority = noDelegateCode
				}
				fmt.Printf("Error making request to ClusterID \"%s\": unable to make request: %v\n", replica.ClusterID, err)
				continue
			}
			// Add cluster's basic auth credentials
			req.SetBasicAuth(cluster.AuthUser, cluster.AuthPassword)

			// Make HTTP client
			fmt.Println("SSLVerify :", cluster.SSLVerify)
			var transport http.RoundTripper = &http.Transport{
				// Enable/disable SSL verification
				TLSClientConfig: &tls.Config{InsecureSkipVerify: !cluster.SSLVerify},
			}
			fmt.Println("Transport : ", transport)
			client := &http.Client{
				Transport: transport,
				Timeout:   time.Second * 20,
			}

			// Send the request
			res, err := client.Do(req)
			fmt.Println("StatusCode : ", res.StatusCode)
			if err != nil {
				if service.Delegation != "static" {
					service.Replicas[id].Priority = noDelegateCode
					//replica.Priority = noDelegateCode
				}
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
					if service.Delegation != "static" {
						service.Replicas[id].Priority = noDelegateCode
						//replica.Priority = noDelegateCode
					}
					fmt.Println("Error decoding the JSON of the response:", err)
					continue
				}

				// CPU is in miliCPU
				// CPU required to deploy the service
				serviceCPU, err := strconv.ParseFloat(service.CPU, 64)
				if err != nil {
					if service.Delegation != "static" {
						service.Replicas[id].Priority = noDelegateCode
						//replica.Priority = noDelegateCode
					}
					fmt.Println("Error to converter CPU of service to int: ", err)
					continue
				}
				fmt.Println("serviceCPU: ", serviceCPU)
				maxNodeCPU := float64(clusterStatus.CPUMaxFree)
				fmt.Println("maxNodeCPU: ", maxNodeCPU)
				//Calculate CPU difference to determine whether to delegate a replica to the cluster
				dist := maxNodeCPU - (1000 * serviceCPU)
				fmt.Println("CPU difference:  ", dist)

				//The priority of delegating the service is set based on the free CPU of the cluster as long as it has free CPU on a node to delegate the service.
				if dist >= 0 {
					fmt.Println("Resources available in ClusterID", replica.ClusterID)
					if service.Delegation == "random" {
						randPriority := rand.Intn(noDelegateCode)
						service.Replicas[id].Priority = uint(randPriority)
						//replica.Priority = uint(randPriority)
						fmt.Println("Priority ", service.Replicas[id].Priority, " with ", service.Delegation, " delegation")
					} else if service.Delegation == "load-based" {
						//Map the totalClusterCPU range to a smaller range (input range 0 to 32 cpu to output range 100 to 0 priority)
						totalClusterCPU := clusterStatus.CPUFreeTotal
						mappedCPUPriority := mapToRange(totalClusterCPU, 0, 32000, 100, 0)
						service.Replicas[id].Priority = uint(mappedCPUPriority)
						//replica.Priority = uint(mappedCPUPriority)
						fmt.Println("Priority ", service.Replicas[id].Priority, " with ", service.Delegation, " delegation")

					} else if service.Delegation != "static" {
						service.Replicas[id].Priority = noDelegateCode
						//replica.Priority = noDelegateCode
						fmt.Println("Error when declaring the type of delegation in ClusterID ", replica.ClusterID)
						continue
					}

				} else {
					fmt.Println("No CPU capacity to delegate job in ClusterID ", replica.ClusterID)
					if service.Delegation != "static" {
						service.Replicas[id].Priority = noDelegateCode
						//replica.Priority = noDelegateCode
					}
					continue
				}
				fmt.Println("Status Cluster ", clusterStatus)
				fmt.Println("Priority: ", service.Replicas[id].Priority)

			} else {
				if service.Delegation != "static" {
					service.Replicas[id].Priority = noDelegateCode
					//replica.Priority = noDelegateCode
				}
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

func topsisMethod(results [][]float64, weight []float64) []float64 {

	// Step 1: Normalize the matrix
	matrixNormalized := normalizeMatrix(results)
	//fmt.Println("//Normalized matrix: ")
	//for _, row := range matrizNormalizada {
	//	fmt.Println(row)
	//}

	// Step 2: Weight the matrix
	matrixWeighted := weightMatrix(matrixNormalized, weight)
	//fmt.Println("\n//Weighted Matrix:")
	//for _, row := range matrizPonderada {
	//	fmt.Println(row)
	//}

	// Step 3: Compute the ideal and anti-ideal solution
	ideal, antiIdeal := calculateSolutions(matrixWeighted)
	//fmt.Println("\nIdeal Solution:", ideal)
	//fmt.Println("Anti-Ideal Solution:", antiIdeal)

	// Step 4: Compute the distances and preference index
	preferences := calculatePreferences(matrixWeighted, ideal, antiIdeal)
	fmt.Println("\nPreference index:", preferences)

	return preferences

}

func sortbyThreshold(preferences []float64, umbral int) []Alternative {
	// Step 5: Order alternatives from best to worst
	alternativesSort := sortAlternatives(preferences)

	fmt.Println("\nAlternatives ordered from best to worst:")
	for _, alt := range alternativesSort {
		fmt.Printf("Alternative %d: %f\n", alt.Index, alt.Preference)

		//mapped := mapToRange(int64(alt*100.0), 0, 100, 100, 0)
		//fmt.Printf("Original Preference: %.4f -> Mapped %d\n", alt, mapped)
	}

	distancesFromBetter := distancesFromBetter(alternativesSort)

	// Threshold to reorder nearby elements
	f := float64(umbral) / 100.0
	threshold := alternativesSort[0].Preference * f
	fmt.Printf("The threshold is the %d %% of the best value: %f ", umbral, threshold)

	// Randomly reorder elements whose distance is less than the threshold, including the first one
	newAlternatives := reorganizeIfNearby(alternativesSort, distancesFromBetter, threshold)

	var priority_map int
	for id, alt := range newAlternatives {
		//fmt.Printf("Alternative %d: %d\n", alt.Index, int(alt.Preference*100))
		priority_map = mapToRange(int64(alt.Preference*100.0), 0, 100, 100, 0)
		newAlternatives[id].Preference = float64(priority_map)
		//fmt.Printf("Alternative %d: %f\n", alt.Index, newAlternatives[id].Preference)
	}

	return newAlternatives

}

func countJobs(jobStatuses map[string]JobStatus) (float64, int) {
	totalJobs := 0
	succeededCount := 0
	failedCount := 0
	pendingCount := 0
	totalExecutionTime := 0.0
	for _, status := range jobStatuses {
		totalJobs++
		switch status.Status {
		case "Succeeded":
			succeededCount++
			creationTime, _ := time.Parse(time.RFC3339, status.CreationTime)
			finishTime, _ := time.Parse(time.RFC3339, status.FinishTime)
			duration := finishTime.Sub(creationTime).Seconds() // Duration in seconds
			totalExecutionTime += duration
		case "Failed":
			failedCount++
		case "Pending": // Pending jobs
			pendingCount++
		}
	}

	var averageExecutionTime float64 = 1e6
	if succeededCount > 0 {
		averageExecutionTime = totalExecutionTime / float64(succeededCount)
	}
	return averageExecutionTime, pendingCount

}

func createParameters(results [][]float64, duration time.Duration, clusterStatus GeneralInfo, serviceCPU float64, averageExecutionTime float64, pendingCount float64) [][]float64 {
	maxNodeCPU := float64(clusterStatus.CPUMaxFree)
	dist := maxNodeCPU - (1000 * serviceCPU)

	if dist >= 0 {
		results = append(results, []float64{
			duration.Seconds(),                     // Latency (ms)
			float64(clusterStatus.NumberNodes),     // Number of nodes
			float64(clusterStatus.MemoryFreeTotal), // Total Memory Free
			float64(clusterStatus.CPUFreeTotal),    // Total CPU Free
			averageExecutionTime,                   // averageExecutionTime
			pendingCount + 0.1,                     //pendingCount
			// More criteria can be added here if needed
		})
	} else {
		results = append(results, []float64{duration.Seconds(), 0, 0, 0, 1e6, 1e6})
	}
	return results
}

func eventBuild(event string, storage_provider string) ([]byte, string) {
	fmt.Println("eventBuild Function...")
	var eventMap map[string]interface{}
	var cluster_storage string
	err := json.Unmarshal([]byte(event), &eventMap)
	if err != nil {
		fmt.Println("Error to read the event:", err)
		return nil, ""
	}
	//fmt.Println(len(eventMap))
	var eventJSON []byte
	if storage, exists := eventMap["storage_provider"]; exists {
		fmt.Println("The 'storage_provider' field exists in the event ")
		eventValue := eventMap["event"]
		//eventString, _ := json.Marshal(eventValue)

		//delegatedEvent1 := WrapEvent(storage.(string), string(eventString))
		delegatedEvent1 := WrapEvent(storage.(string), eventValue.(string))
		fmt.Println("New DelegatedEvent:", delegatedEvent1)

		k, err1 := json.Marshal(delegatedEvent1)

		//k, err1 := json.Marshal(event)
		if err1 != nil {
			fmt.Printf("error marshalling delegated event: %v ", err1)
			return nil, ""
		}

		eventJSON = k
		cluster_storage = storage.(string)

	} else {
		fmt.Println("The 'storage_provider' field does NOT exist in the event")

		delegatedEvent := WrapEvent(storage_provider, event)
		fmt.Println("New DelegatedEvent:", delegatedEvent)

		z, err2 := json.Marshal(delegatedEvent)
		if err2 != nil {
			fmt.Printf("error marshalling delegated event: %v", err2)
			return nil, ""
		}
		eventJSON = z
		cluster_storage = storage_provider
	}

	//fmt.Println(string(eventJson))
	return eventJSON, cluster_storage
}
