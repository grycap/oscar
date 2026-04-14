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

const (
	VolumeCreationModeService = "service"
	VolumeCreationModeAPI     = "api"

	VolumeLifecycleDelete = "delete"
	VolumeLifecycleRetain = "retain"

	VolumePhasePending  = "pending"
	VolumePhaseReady    = "ready"
	VolumePhaseInUse    = "in_use"
	VolumePhaseError    = "error"
	VolumePhaseDeleting = "deleting"
	VolumePhaseDeleted  = "deleted"
)

type VolumeInfo struct {
	VolumeLimits  VolumeLimits    `json:"volume_limits"`
	ManagedVolume []ManagedVolume `json:"managed_volume"`
}

type VolumeLimits struct {
	DiskAvailable    string `json:"disk_available"`
	MaxVolumes       string `json:"max_volumes"`
	MaxDiskperVolume string `json:"max_disk_per_volume"`
	MinDiskperVolume string `json:"min_disk_per_volume"`
}

// ManagedVolume represents the API response for a managed OSCAR volume.
type ManagedVolume struct {
	Name             string       `json:"name"`
	Namespace        string       `json:"namespace,omitempty"`
	PVCName          string       `json:"pvc_name,omitempty"`
	Size             string       `json:"size,omitempty"`
	OwnerUser        string       `json:"owner_user,omitempty"`
	CreatedByService string       `json:"created_by_service,omitempty"`
	CreationMode     string       `json:"creation_mode,omitempty"`
	LifecyclePolicy  string       `json:"lifecycle_policy,omitempty"`
	Status           VolumeStatus `json:"status"`
}

// ManagedVolumeCreateRequest is the payload for POST /system/volumes.
type ManagedVolumeCreateRequest struct {
	Name string `json:"name" binding:"required"`
	Size string `json:"size" binding:"required"`
}

// VolumeStatus contains minimal managed-volume status information.
type VolumeStatus struct {
	Phase           string `json:"phase,omitempty"`
	Message         string `json:"message,omitempty"`
	AttachmentCount int    `json:"attachment_count,omitempty"`
}
