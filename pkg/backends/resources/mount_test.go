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
	"testing"

	"github.com/grycap/oscar/v3/pkg/types"
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
					{
						Name:      ephemeralVolumeName,
						MountPath: ephemeralVolumeMount,
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

	if len(container.VolumeMounts) != 4 {
		t.Fatalf("expected 4 volume mounts, got %d", len(container.VolumeMounts))
	}

	if len(podSpec.Volumes) != 2 {
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
