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

type Federation struct {
	//
	GroupID string `json:"group_id,omitempty"`
	//Topology Mode: Defines the network structure that the replicated services will have within the federation.
	//Optional (default: none)
	//"mesh": Mesh structure of federated services.
	//"tree": Tree structure of federated services.
	//"none": No structure defined
	Topology string `json:"topology,omitempty"`
	//Delegation Mode of job delegation for replicas
	// Opcional (default: manual)
	//"static" The user select the priority to delegate jobs to the replicas.
	//"random" The job delegation priority is generated randomly among the clusters of the available replicas.
	//"load-based" The job delegation priority is generated depending on the CPU and Memory available in the replica clusters.
	//Delegation string `json:"delegation"`
	Delegation string `json:"delegation"`
	// Cluster where the replica services are located
	Members []Members `json:"members,omitempty"`
}

// Clusters struct to define service's replicas in other clusters or endpoints
type Members struct {
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

// Members list of replicas implementing sort.Interface
type MembersList []Members

// Len method to implement sort.Interface
func (rl MembersList) Len() int {
	return len(rl)
}

// Swap method to implement sort.Interface
func (rl MembersList) Swap(i, j int) {
	rl[i], rl[j] = rl[j], rl[i]
}

// Less method to implement sort.Interface ordering by Replica.Priority
func (rl MembersList) Less(i, j int) bool {
	return rl[i].Priority < rl[j].Priority
}
