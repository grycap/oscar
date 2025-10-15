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
)

func TestParseSeconds(t *testing.T) {
	scenarios := []struct {
		name        string
		value       string
		returnError bool
	}{
		{"Invalid: text", "asdf", true},
		{"Invalid: negative", "-25", true},
		{"Valid", "15", false},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			val, err := parseSeconds(s.value)

			if s.returnError {
				if err == nil {
					t.Errorf("expected error, got: %v", val)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestNoUsername(t *testing.T) {
	_, err := ReadConfig()

	if err == nil {
		t.Error("OSCAR_USERNAME must be mandatory")
	}
}

func TestNoPassword(t *testing.T) {
	t.Setenv("OSCAR_USERNAME", "testuser")

	_, err := ReadConfig()

	if err == nil {
		t.Error("OSCAR_PASSWORD must be mandatory")
	}
}

func TestNoMinIOAccessKey(t *testing.T) {
	t.Setenv("OSCAR_USERNAME", "testuser")
	t.Setenv("OSCAR_PASSWORD", "testpass")

	_, err := ReadConfig()

	if err == nil {
		t.Error("MINIO_ACCESS_KEY must be mandatory")
	}
}

func TestNoMinIOSecretKey(t *testing.T) {
	t.Setenv("OSCAR_USERNAME", "testuser")
	t.Setenv("OSCAR_PASSWORD", "testpass")
	t.Setenv("MINIO_ACCESS_KEY", "minioaccess")

	_, err := ReadConfig()

	if err == nil {
		t.Error("MINIO_SECRET_KEY must be mandatory")
	}
}

func TestRequiredValues(t *testing.T) {
	t.Setenv("OSCAR_USERNAME", "testuser")
	t.Setenv("OSCAR_PASSWORD", "testpass")
	t.Setenv("MINIO_ACCESS_KEY", "minioaccess")
	t.Setenv("MINIO_SECRET_KEY", "miniosecret")

	cfg, err := ReadConfig()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.Username != "testuser" {
		t.Errorf("expected username: %s, got: %s", "testuser", cfg.Username)
	}

	if cfg.Password != "testpass" {
		t.Errorf("expected password: %s, got: %s", "testpass", cfg.Password)
	}

	if cfg.MinIOProvider.AccessKey != "minioaccess" {
		t.Errorf("expected minio access key: %s, got: %s", "minioaccess", cfg.MinIOProvider.AccessKey)
	}

	if cfg.MinIOProvider.SecretKey != "miniosecret" {
		t.Errorf("expected minio secret key: %s, got: %s", "miniosecret", cfg.MinIOProvider.SecretKey)
	}
}

func TestCustomValues(t *testing.T) {
	environment := map[string]string{
		"OSCAR_USERNAME":                "testuser",
		"OSCAR_PASSWORD":                "testpass",
		"MINIO_ACCESS_KEY":              "testminioaccess",
		"MINIO_SECRET_KEY":              "testminiosecret",
		"MINIO_REGION":                  "testminioregion",
		"MINIO_TLS_VERIFY":              "true",
		"MINIO_ENDPOINT":                "https://test.minio.endpoint",
		"OSCAR_NAME":                    "testname",
		"OSCAR_NAMESPACE":               "testnamespace",
		"OSCAR_SERVICES_NAMESPACE":      "testservicesnamespace",
		"WATCHDOG_MAX_INFLIGHT":         "20",
		"WATCHDOG_WRITE_DEBUG":          "false",
		"WATCHDOG_EXEC_TIMEOUT":         "50",
		"WATCHDOG_READ_TIMEOUT":         "50",
		"WATCHDOG_WRITE_TIMEOUT":        "50",
		"WATCHDOG_HEALTHCHECK_INTERVAL": "50",
		"READ_TIMEOUT":                  "50",
		"WRITE_TIMEOUT":                 "50",
		"OSCAR_SERVICE_PORT":            "8000",
		"YUNIKORN_ENABLE":               "true",
		"YUNIKORN_NAMESPACE":            "testyunikornnamespace",
		"YUNIKORN_CONFIGMAP":            "testyunikornconfigmap",
		"YUNIKORN_CONFIG_FILENAME":      "testyunikornconfigfilename",
	}

	scenarios := []struct {
		name        string
		envVarKey   string
		envVarValue string
		returnError bool
	}{
		{
			"Valid values",
			"test",
			"test",
			false,
		},
		{
			"Invalid bool",
			"MINIO_TLS_VERIFY",
			"test",
			true,
		},
		{
			"Invalid URL",
			"MINIO_ENDPOINT",
			" htt://testendpoint",
			true,
		},
		{
			"Invalid int",
			"WATCHDOG_MAX_INFLIGHT",
			"test",
			true,
		},
		{
			"Invalid seconds",
			"READ_TIMEOUT",
			"test",
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			for k, v := range environment {
				t.Setenv(k, v)
			}
			t.Setenv(s.envVarKey, s.envVarValue)

			_, err := ReadConfig()

			if s.returnError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSetAllowedImageRepositories(t *testing.T) {
	cfg := &Config{}
	input := []string{"GhCr.io/ORG/Repo ", " docker.io ", "docker.io/library/oscar"}
	if err := cfg.SetAllowedImageRepositories(input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repos := cfg.GetAllowedImageRepositories()
	expected := []string{"ghcr.io/org/repo", "docker.io", "docker.io/library/oscar"}
	if len(repos) != len(expected) {
		t.Fatalf("expected %d repos, got %d", len(expected), len(repos))
	}
	for i := range expected {
		if repos[i] != expected[i] {
			t.Fatalf("expected repo %q, got %q", expected[i], repos[i])
		}
	}
}

func TestValidateImageRepository(t *testing.T) {
	cfg := &Config{}
	if err := cfg.SetAllowedImageRepositories([]string{"ghcr.io/org", "docker.io/library/oscar"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name    string
		image   string
		wantErr bool
	}{
		{"Allowed same repo", "ghcr.io/org/service:latest", false},
		{"Allowed subpath", "ghcr.io/org/sub/service:1.0", false},
		{"Allowed docker hub canonical", "docker.io/library/oscar:1.0", false},
		{"Allowed docker hub implicit", "oscar:latest", false},
		{"Disallowed registry", "quay.io/org/service:v1", true},
		{"Disallowed path prefix", "ghcr.io/org-other/service:1.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cfg.ValidateImageRepository(tt.image)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for image %q", tt.image)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for image %q: %v", tt.image, err)
			}
		})
	}
}

func TestReadConfigAllowedRepositories(t *testing.T) {
	t.Setenv("OSCAR_USERNAME", "testuser")
	t.Setenv("OSCAR_PASSWORD", "testpass")
	t.Setenv("MINIO_ACCESS_KEY", "testminioaccess")
	t.Setenv("MINIO_SECRET_KEY", "testminiosecret")
	t.Setenv("ALLOWED_IMAGE_REPOSITORIES", "ghcr.io/grycap/oscar,docker.io/library/safe")

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repos := cfg.GetAllowedImageRepositories()
	expected := []string{"ghcr.io/grycap/oscar", "docker.io/library/safe"}
	if len(repos) != len(expected) {
		t.Fatalf("expected %d repos, got %d", len(expected), len(repos))
	}
	for i := range expected {
		if repos[i] != expected[i] {
			t.Fatalf("expected repo %q, got %q", expected[i], repos[i])
		}
	}
}

func TestReadConfigInvalidAllowedRepository(t *testing.T) {
	t.Setenv("OSCAR_USERNAME", "testuser")
	t.Setenv("OSCAR_PASSWORD", "testpass")
	t.Setenv("MINIO_ACCESS_KEY", "testminioaccess")
	t.Setenv("MINIO_SECRET_KEY", "testminiosecret")
	t.Setenv("ALLOWED_IMAGE_REPOSITORIES", "https://invalid")

	if _, err := ReadConfig(); err == nil {
		t.Fatalf("expected error for invalid allowed repository")
	}
}

func TestServerlessBackend(t *testing.T) {
	environment := map[string]string{
		"OSCAR_USERNAME":   "testuser",
		"OSCAR_PASSWORD":   "testpass",
		"MINIO_ACCESS_KEY": "testminioaccess",
		"MINIO_SECRET_KEY": "testminiosecret",
	}

	scenarios := []struct {
		name              string
		serverlessBackend string
		returnError       bool
	}{
		{
			"Valid \"openfaas\"",
			"openfaas",
			false,
		},
		{
			"Valid \"knative\"",
			"knative",
			false,
		},
		{
			"Valid \"OPENFAAS\"",
			"OPENFAAS",

			false,
		},
		{
			"Invalid",
			"test",

			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			environment["SERVERLESS_BACKEND"] = s.serverlessBackend
			for k, v := range environment {
				t.Setenv(k, v)
			}

			_, err := ReadConfig()

			if s.returnError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
