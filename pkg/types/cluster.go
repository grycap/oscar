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

// Cluster struct to store cluster access data
type Cluster struct {
	// Endpoint endpoint of the OSCAR cluster API
	Endpoint string `json:"endpoint"`
	// AuthUser username to connect to the cluster (basic auth)
	AuthUser string `json:"auth_user"`
	// AuthPassword password to connect to the cluster (basic auth)
	AuthPassword string `json:"auth_password"`
	// SSLVerify parameter to enable or disable the verification of SSL certificates
	SSLVerify bool `json:"ssl_verify"`
}
