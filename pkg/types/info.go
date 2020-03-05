// Copyright (C) GRyCAP - I3M - UPV
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

// Info represents the system information to be exposed
type Info struct {
	Version           string                `json:"version"`
	Arch              string                `json:"arch"`
	KubeVersion       string                `json:"kubernetes_version"`
	ServerlessBackend ServerlessBackendInfo `json:"serverless_backend,omitempty"`
}

// ServerlessBackendInfo shows the name and version of the underlying serverless backend
type ServerlessBackendInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
