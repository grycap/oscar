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

// Federation defines a group of services replicated across clusters.
type Federation struct {
	// GroupID identifies the federation network.
	GroupID string `json:"group_id"`
	// Topology defines the federation topology: none, star, mesh.
	Topology string `json:"topology"`
	// Delegation defines the delegation policy: static, random, load-based.
	Delegation string `json:"delegation,omitempty"`
	// Members list of replica references in the federation.
	Members ReplicaList `json:"members,omitempty"`
}
