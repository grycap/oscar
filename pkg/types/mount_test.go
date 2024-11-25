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

import (
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestSetMount(t *testing.T) {
	podSpec := &v1.PodSpec{}
	service := Service{
		Mount: StorageIOConfig{
			Provider: "minio.provider",
			Path:     "test-bucket",
		},
		StorageProviders: &StorageProviders{
			MinIO: map[string]*MinIOProvider{
				"provider": {
					AccessKey: "test-access-key",
					SecretKey: "test-secret-key",
					Endpoint:  "test-endpoint",
				},
			},
		},
	}
	cfg := &Config{}

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
	service := Service{
		Mount: StorageIOConfig{
			Path: "test-bucket",
		},
		StorageProviders: &StorageProviders{
			MinIO: map[string]*MinIOProvider{
				"provider": {
					AccessKey: "test-access-key",
					SecretKey: "test-secret-key",
					Endpoint:  "test-endpoint",
				},
			},
		},
	}
	providerId := "provider"

	envVars := setMinIOEnvVars(service, providerId)

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

func TestSetWebDavEnvVars(t *testing.T) {
	service := Service{
		Mount: StorageIOConfig{
			Path: "test-folder",
		},
		StorageProviders: &StorageProviders{
			WebDav: map[string]*WebDavProvider{
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
