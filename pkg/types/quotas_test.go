package types

import (
	"testing"
)

func TestQuotaBackend(t *testing.T) {
	qb := QuotaBackend{
		Kueueclient:   nil,
		KubeClientset: nil,
	}

	if qb.Kueueclient != nil {
		t.Error("Expected Kueueclient to be nil")
	}
	if qb.KubeClientset != nil {
		t.Error("Expected KubeClientset to be nil")
	}
}

func TestQuotaResponse(t *testing.T) {
	response := QuotaResponse{
		UserID:       "user1",
		ClusterQueue: "default",
		Resources: map[string]QuotaValues{
			"cpu":    {Max: 1000, Used: 500},
			"memory": {Max: 2000, Used: 1000},
		},
		Volumes: &VolumeQuotaResponse{
			Disk:           VolumeQuotaValues{Max: "100Gi", Used: "50Gi"},
			Volumes:        VolumeQuotaValues{Max: "10", Used: "5"},
			MaxDiskperVolume: "50Gi",
			MinDiskperVolume: "1Gi",
		},
	}

	if response.UserID != "user1" {
		t.Errorf("Expected UserID = user1, got %s", response.UserID)
	}
	if response.ClusterQueue != "default" {
		t.Errorf("Expected ClusterQueue = default, got %s", response.ClusterQueue)
	}
	if len(response.Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(response.Resources))
	}
	if response.Volumes == nil {
		t.Error("Expected Volumes to be set")
	}
	if response.Volumes.Disk.Max != "100Gi" {
		t.Errorf("Expected Disk.Max = 100Gi, got %s", response.Volumes.Disk.Max)
	}
}

func TestQuotaValues(t *testing.T) {
	qv := QuotaValues{
		Max:  1000,
		Used: 500,
	}

	if qv.Max != 1000 {
		t.Errorf("Expected Max = 1000, got %d", qv.Max)
	}
	if qv.Used != 500 {
		t.Errorf("Expected Used = 500, got %d", qv.Used)
	}
}

func TestVolumeQuotaResponse(t *testing.T) {
	vqr := VolumeQuotaResponse{
		Disk:           VolumeQuotaValues{Max: "100Gi", Used: "50Gi"},
		Volumes:        VolumeQuotaValues{Max: "10", Used: "5"},
		MaxDiskperVolume: "50Gi",
		MinDiskperVolume: "1Gi",
	}

	if vqr.Disk.Max != "100Gi" {
		t.Errorf("Expected Disk.Max = 100Gi, got %s", vqr.Disk.Max)
	}
	if vqr.Volumes.Max != "10" {
		t.Errorf("Expected Volumes.Max = 10, got %s", vqr.Volumes.Max)
	}
	if vqr.MaxDiskperVolume != "50Gi" {
		t.Errorf("Expected MaxDiskperVolume = 50Gi, got %s", vqr.MaxDiskperVolume)
	}
	if vqr.MinDiskperVolume != "1Gi" {
		t.Errorf("Expected MinDiskperVolume = 1Gi, got %s", vqr.MinDiskperVolume)
	}
}

func TestVolumeQuotaValues(t *testing.T) {
	vqv := VolumeQuotaValues{
		Max:  "100Gi",
		Used: "50Gi",
	}

	if vqv.Max != "100Gi" {
		t.Errorf("Expected Max = 100Gi, got %s", vqv.Max)
	}
	if vqv.Used != "50Gi" {
		t.Errorf("Expected Used = 50Gi, got %s", vqv.Used)
	}
}

func TestQuotaUpdateRequest(t *testing.T) {
	req := QuotaUpdateRequest{
		CPU:    "1000m",
		Memory: "2Gi",
		Volumes: &VolumeQuotaUpdate{
			Disk:           "100Gi",
			Volumes:        "10",
			MaxDiskperVolume: "50Gi",
			MinDiskperVolume: "1Gi",
		},
	}

	if req.CPU != "1000m" {
		t.Errorf("Expected CPU = 1000m, got %s", req.CPU)
	}
	if req.Memory != "2Gi" {
		t.Errorf("Expected Memory = 2Gi, got %s", req.Memory)
	}
	if req.Volumes == nil {
		t.Error("Expected Volumes to be set")
	}
	if req.Volumes.Disk != "100Gi" {
		t.Errorf("Expected Volumes.Disk = 100Gi, got %s", req.Volumes.Disk)
	}
}

func TestVolumeQuotaUpdate(t *testing.T) {
	vqu := VolumeQuotaUpdate{
		Disk:            "100Gi",
		Volumes:         "10",
		MaxDiskperVolume: "50Gi",
		MinDiskperVolume: "1Gi",
	}

	if vqu.Disk != "100Gi" {
		t.Errorf("Expected Disk = 100Gi, got %s", vqu.Disk)
	}
	if vqu.Volumes != "10" {
		t.Errorf("Expected Volumes = 10, got %s", vqu.Volumes)
	}
	if vqu.MaxDiskperVolume != "50Gi" {
		t.Errorf("Expected MaxDiskperVolume = 50Gi, got %s", vqu.MaxDiskperVolume)
	}
	if vqu.MinDiskperVolume != "1Gi" {
		t.Errorf("Expected MinDiskperVolume = 1Gi, got %s", vqu.MinDiskperVolume)
	}
}