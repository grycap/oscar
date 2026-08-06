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

// SecretUpdateRequest is the payload for PUT /system/secrets/{service}.
// The keys/values are merged into the service secret, creating the keys that
// do not already exist.
type SecretUpdateRequest struct {
	// Secrets key-value pairs to merge into the service secret.
	Secrets map[string]string `json:"secrets" binding:"required"`
}

// ServiceSecrets represents the response for service secret operations.
type ServiceSecrets struct {
	// Service the OSCAR service name.
	Service string `json:"service"`
	// Secrets the service environment secrets (key-value pairs).
	Secrets map[string]string `json:"secrets"`
}
