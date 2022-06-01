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

import "github.com/grycap/oscar/v2/pkg/types"

// TODO: add comment!
// ReScheduledEvent
type ReScheduledEvent struct {
	StorageProviderID string `json:"storage_provider"`
	Event             string `json:"event"`
}

// TODO: implement:
// get svc configMap (FDL), get service token in the replica, update event with "storage_provider" field
// DelegateJob sends the event to a service's replica
func DelegateJob(service types.Service, event string) error {

	return nil
}

// WrapEvent wraps an event adding the storage_provider field (from the service's cluster_id)
func WrapEvent(providerID string, event string) ReScheduledEvent {
	return ReScheduledEvent{
		StorageProviderID: providerID,
		Event:             event,
	}
}
