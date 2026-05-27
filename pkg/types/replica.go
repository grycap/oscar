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

package types

// Replica struct to define service's replicas in other clusters or endpoints
type Replica struct {
	// Type of the replica to re-send events (can be "oscar" or "endpoint")
	Type string `json:"type"`
	// ClusterID identifier of the cluster as defined in the "clusters" FDL field
	// Only used if Type is "oscar"
	ClusterID string `json:"cluster_id"`
	// ServiceName name of the service in the replica cluster.
	// Only used if Type is "oscar"
	ServiceName string `json:"service_name"`
	// URL url of the endpoint to re-send events (HTTP POST).
	// Only used if Type is "endpoint"
	URL string `json:"url"`
	// SSLVerify parameter to enable or disable the verification of SSL certificates.
	// Only used if Type is "endpoint"
	// Optional. (default: true)
	SSLVerify bool `json:"ssl_verify"`
	// Priority value to define delegation priority. Highest priority is defined as 0.
	// If a delegation fails, OSCAR will try to delegate to another replica with lower priority
	// Optional. (default: 0)
	Priority uint `json:"priority"`
	// Headers headers to send in delegation requests
	// Optional
	Headers map[string]string `json:"headers"`
}

// ReplicaList list of replicas implementing sort.Interface
type ReplicaList []Replica

// Len method to implement sort.Interface
func (rl ReplicaList) Len() int {
	return len(rl)
}

// Swap method to implement sort.Interface
func (rl ReplicaList) Swap(i, j int) {
	rl[i], rl[j] = rl[j], rl[i]
}

// Less method to implement sort.Interface ordering by Replica.Priority
func (rl ReplicaList) Less(i, j int) bool {
	return rl[i].Priority < rl[j].Priority
}

// FederationResponse response payload for federation API.
type FederationResponse struct {
	Topology string      `json:"topology"`
	Members  ReplicaList `json:"members"`
}

// FederationRequest payload for federation API.
type FederationRequest struct {
	Members          ReplicaList        `json:"members"`
	Update           ReplicaList        `json:"update,omitempty"`
	Clusters         map[string]Cluster `json:"clusters,omitempty"`
	StorageProviders *StorageProviders  `json:"storage_providers,omitempty"`
}
