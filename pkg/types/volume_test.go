package types

import (
	"testing"
)

func TestVolumeConstants(t *testing.T) {
	if VolumeCreationModeService != "service" {
		t.Errorf("Expected VolumeCreationModeService = service, got %s", VolumeCreationModeService)
	}
	if VolumeCreationModeAPI != "api" {
		t.Errorf("Expected VolumeCreationModeAPI = api, got %s", VolumeCreationModeAPI)
	}
	if VolumeLifecycleDelete != "delete" {
		t.Errorf("Expected VolumeLifecycleDelete = delete, got %s", VolumeLifecycleDelete)
	}
	if VolumeLifecycleRetain != "retain" {
		t.Errorf("Expected VolumeLifecycleRetain = retain, got %s", VolumeLifecycleRetain)
	}
}

func TestVolumePhaseConstants(t *testing.T) {
	if VolumePhasePending != "pending" {
		t.Errorf("Expected VolumePhasePending = pending, got %s", VolumePhasePending)
	}
	if VolumePhaseReady != "ready" {
		t.Errorf("Expected VolumePhaseReady = ready, got %s", VolumePhaseReady)
	}
	if VolumePhaseInUse != "in_use" {
		t.Errorf("Expected VolumePhaseInUse = in_use, got %s", VolumePhaseInUse)
	}
	if VolumePhaseError != "error" {
		t.Errorf("Expected VolumePhaseError = error, got %s", VolumePhaseError)
	}
	if VolumePhaseDeleting != "deleting" {
		t.Errorf("Expected VolumePhaseDeleting = deleting, got %s", VolumePhaseDeleting)
	}
	if VolumePhaseDeleted != "deleted" {
		t.Errorf("Expected VolumePhaseDeleted = deleted, got %s", VolumePhaseDeleted)
	}
}

func TestVolumeLimits(t *testing.T) {
	limits := VolumeLimits{
		DiskAvailable:    "100Gi",
		MaxVolumes:       "10",
		MaxDiskperVolume: "50Gi",
		MinDiskperVolume: "1Gi",
	}

	if limits.DiskAvailable != "100Gi" {
		t.Errorf("Expected DiskAvailable = 100Gi, got %s", limits.DiskAvailable)
	}
	if limits.MaxVolumes != "10" {
		t.Errorf("Expected MaxVolumes = 10, got %s", limits.MaxVolumes)
	}
	if limits.MaxDiskperVolume != "50Gi" {
		t.Errorf("Expected MaxDiskperVolume = 50Gi, got %s", limits.MaxDiskperVolume)
	}
	if limits.MinDiskperVolume != "1Gi" {
		t.Errorf("Expected MinDiskperVolume = 1Gi, got %s", limits.MinDiskperVolume)
	}
}

func TestManagedVolume(t *testing.T) {
	vol := ManagedVolume{
		Name:             "test-volume",
		Namespace:        "default",
		PVCName:          "pvc-test-volume",
		Size:             "10Gi",
		OwnerUser:        "user1",
		CreatedByService: "test-service",
		CreationMode:    VolumeCreationModeService,
		LifecyclePolicy: VolumeLifecycleRetain,
		Attachments: []VolumeAttachmentReference{
			{ServiceName: "svc1", MountPath: "/data"},
		},
		Status: VolumeStatus{
			Phase:           VolumePhaseReady,
			Message:         "Volume is ready",
			AttachmentCount: 1,
		},
	}

	if vol.Name != "test-volume" {
		t.Errorf("Expected Name = test-volume, got %s", vol.Name)
	}
	if vol.Namespace != "default" {
		t.Errorf("Expected Namespace = default, got %s", vol.Namespace)
	}
	if vol.PVCName != "pvc-test-volume" {
		t.Errorf("Expected PVCName = pvc-test-volume, got %s", vol.PVCName)
	}
	if vol.Size != "10Gi" {
		t.Errorf("Expected Size = 10Gi, got %s", vol.Size)
	}
	if vol.OwnerUser != "user1" {
		t.Errorf("Expected OwnerUser = user1, got %s", vol.OwnerUser)
	}
	if vol.CreatedByService != "test-service" {
		t.Errorf("Expected CreatedByService = test-service, got %s", vol.CreatedByService)
	}
	if vol.CreationMode != VolumeCreationModeService {
		t.Errorf("Expected CreationMode = service, got %s", vol.CreationMode)
	}
	if vol.LifecyclePolicy != VolumeLifecycleRetain {
		t.Errorf("Expected LifecyclePolicy = retain, got %s", vol.LifecyclePolicy)
	}
	if len(vol.Attachments) != 1 {
		t.Errorf("Expected 1 attachment, got %d", len(vol.Attachments))
	}
	if vol.Status.Phase != VolumePhaseReady {
		t.Errorf("Expected Status.Phase = ready, got %s", vol.Status.Phase)
	}
	if vol.Status.AttachmentCount != 1 {
		t.Errorf("Expected Status.AttachmentCount = 1, got %d", vol.Status.AttachmentCount)
	}
}

func TestManagedVolumeCreateRequest(t *testing.T) {
	req := ManagedVolumeCreateRequest{
		Name: "my-volume",
		Size: "10Gi",
	}

	if req.Name != "my-volume" {
		t.Errorf("Expected Name = my-volume, got %s", req.Name)
	}
	if req.Size != "10Gi" {
		t.Errorf("Expected Size = 10Gi, got %s", req.Size)
	}
}

func TestVolumeAttachmentReference(t *testing.T) {
	ref := VolumeAttachmentReference{
		ServiceName: "test-service",
		MountPath:   "/data",
	}

	if ref.ServiceName != "test-service" {
		t.Errorf("Expected ServiceName = test-service, got %s", ref.ServiceName)
	}
	if ref.MountPath != "/data" {
		t.Errorf("Expected MountPath = /data, got %s", ref.MountPath)
	}
}

func TestVolumeStatus(t *testing.T) {
	status := VolumeStatus{
		Phase:           VolumePhaseReady,
		Message:         "Volume is ready",
		AttachmentCount: 1,
	}

	if status.Phase != VolumePhaseReady {
		t.Errorf("Expected Phase = ready, got %s", status.Phase)
	}
	if status.Message != "Volume is ready" {
		t.Errorf("Expected Message = Volume is ready, got %s", status.Message)
	}
	if status.AttachmentCount != 1 {
		t.Errorf("Expected AttachmentCount = 1, got %d", status.AttachmentCount)
	}
}