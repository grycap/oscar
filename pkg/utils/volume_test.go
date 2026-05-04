package utils

import (
	"testing"

	"github.com/grycap/oscar/v4/pkg/types"
)

func TestValidateManagedVolumeCreateRequest(t *testing.T) {
	tests := []struct {
		name    string
		req    *types.ManagedVolumeCreateRequest
		wantErr bool
	}{
		{"Valid request", &types.ManagedVolumeCreateRequest{Name: "my-volume", Size: "10Gi"}, false},
		{"Empty name", &types.ManagedVolumeCreateRequest{Name: "", Size: "10Gi"}, true},
		{"Empty size", &types.ManagedVolumeCreateRequest{Name: "my-volume", Size: ""}, true},
		{"Invalid name with spaces", &types.ManagedVolumeCreateRequest{Name: "my volume", Size: "10Gi"}, true},
		{"Invalid size", &types.ManagedVolumeCreateRequest{Name: "my-volume", Size: "invalid"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateManagedVolumeCreateRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateManagedVolumeCreateRequest() error = %v", err)
			}
		})
	}
}

func TestValidateVolumeConfig(t *testing.T) {
	tests := []struct {
		name    string
		volume *types.ServiceVolumeConfig
		wantErr bool
	}{
		{"Valid config", &types.ServiceVolumeConfig{Name: "vol", Size: "10Gi", MountPath: "/data"}, false},
		{"Nil volume", nil, false},
		{"Empty mount path", &types.ServiceVolumeConfig{Name: "vol", Size: "10Gi", MountPath: ""}, true},
		{"Both empty name and size", &types.ServiceVolumeConfig{Name: "", Size: "", MountPath: "/data"}, true},
		{"Invalid lifecycle policy", &types.ServiceVolumeConfig{Name: "vol", Size: "10Gi", MountPath: "/data", LifecyclePolicy: "invalid"}, true},
		{"Relative mount path", &types.ServiceVolumeConfig{Name: "vol", Size: "10Gi", MountPath: "data"}, true},
		{"Reserved config path", &types.ServiceVolumeConfig{Name: "vol", Size: "10Gi", MountPath: "/oscar/config"}, true},
		{"Reserved volume path", &types.ServiceVolumeConfig{Name: "vol", Size: "10Gi", MountPath: "/oscar/bin"}, true},
		{"Reserved path prefix", &types.ServiceVolumeConfig{Name: "vol", Size: "10Gi", MountPath: "/oscar/config/data"}, true},
		{"Valid retain policy", &types.ServiceVolumeConfig{Name: "vol", Size: "10Gi", MountPath: "/data", LifecyclePolicy: types.VolumeLifecycleRetain}, false},
		{"Valid delete policy", &types.ServiceVolumeConfig{Name: "vol", Size: "10Gi", MountPath: "/data", LifecyclePolicy: types.VolumeLifecycleDelete}, false},
		{"Lifecycle without size", &types.ServiceVolumeConfig{Name: "", Size: "", MountPath: "/data", LifecyclePolicy: types.VolumeLifecycleRetain}, true},
		{"Default lifecycle", &types.ServiceVolumeConfig{Name: "vol", Size: "10Gi", MountPath: "/data"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVolumeConfig("test-service", tt.volume)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVolumeConfig() error = %v", err)
			}
		})
	}
}

func TestValidationName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid-volume", false},
		{"my-volume", false},
		{"vol123", false},
		{"my volume", true},
		{"my_volume", true},
		{"-start", true},
		{"end-", true},
		{".start", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validationName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("validationName() error = %v", err)
			}
		})
	}
}

func TestValidateSize(t *testing.T) {
	tests := []struct {
		size   string
		wantErr bool
	}{
		{"10Gi", false},
		{"100Mi", false},
		{"1Ti", false},
		{"1", false},
		{"0", true},
		{"-1Gi", true},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			err := validateSize(tt.size)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSize() error = %v", err)
			}
		})
	}
}

func TestSameVolumeConfig(t *testing.T) {
	tests := []struct {
		name  string
		a, b  *types.ServiceVolumeConfig
		expected bool
	}{
		{"Both nil", nil, nil, true},
		{"One nil a", nil, &types.ServiceVolumeConfig{Name: "vol"}, false},
		{"One nil b", &types.ServiceVolumeConfig{Name: "vol"}, nil, false},
		{"Equal configs", &types.ServiceVolumeConfig{Name: "vol", Size: "10Gi"}, &types.ServiceVolumeConfig{Name: "vol", Size: "10Gi"}, true},
		{"Different name", &types.ServiceVolumeConfig{Name: "vol1"}, &types.ServiceVolumeConfig{Name: "vol2"}, false},
		{"Different size", &types.ServiceVolumeConfig{Size: "10Gi"}, &types.ServiceVolumeConfig{Size: "20Gi"}, false},
		{"Different mount path", &types.ServiceVolumeConfig{MountPath: "/data"}, &types.ServiceVolumeConfig{MountPath: "/data2"}, false},
		{"Different lifecycle", &types.ServiceVolumeConfig{LifecyclePolicy: types.VolumeLifecycleDelete}, &types.ServiceVolumeConfig{LifecyclePolicy: types.VolumeLifecycleRetain}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SameVolumeConfig(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("SameVolumeConfig() = %v, want %v", result, tt.expected)
			}
		})
	}
}