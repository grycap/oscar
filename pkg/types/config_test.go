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
	scenarios := []struct {
		name        string
		environment map[string]string
		returnError bool
	}{
		{
			"Valid values",
			map[string]string{
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
			},
			false,
		},
		{
			"Invalid MINIO_TLS_VERIFY",
			map[string]string{
				"OSCAR_USERNAME":                "testuser",
				"OSCAR_PASSWORD":                "testpass",
				"MINIO_ACCESS_KEY":              "testminioaccess",
				"MINIO_SECRET_KEY":              "testminiosecret",
				"MINIO_REGION":                  "testminioregion",
				"MINIO_TLS_VERIFY":              "test",
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
			},
			true,
		},
		{
			"Invalid MINIO_ENDPOINT",
			map[string]string{
				"OSCAR_USERNAME":                "testuser",
				"OSCAR_PASSWORD":                "testpass",
				"MINIO_ACCESS_KEY":              "testminioaccess",
				"MINIO_SECRET_KEY":              "testminiosecret",
				"MINIO_REGION":                  "testminioregion",
				"MINIO_TLS_VERIFY":              "true",
				"MINIO_ENDPOINT":                " htt://testendpoint",
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
			},
			true,
		},
		{
			"Invalid WATCHDOG_MAX_INFLIGHT",
			map[string]string{
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
				"WATCHDOG_MAX_INFLIGHT":         "test",
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
			},
			true,
		},
		{
			"Invalid WATCHDOG_WRITE_DEBUG",
			map[string]string{
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
				"WATCHDOG_WRITE_DEBUG":          "test",
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
			},
			true,
		},
		{
			"Invalid WATCHDOG_EXEC_TIMEOUT",
			map[string]string{
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
				"WATCHDOG_EXEC_TIMEOUT":         "test",
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
			},
			true,
		},
		{
			"Invalid WATCHDOG_READ_TIMEOUT",
			map[string]string{
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
				"WATCHDOG_READ_TIMEOUT":         "test",
				"WATCHDOG_WRITE_TIMEOUT":        "50",
				"WATCHDOG_HEALTHCHECK_INTERVAL": "50",
				"READ_TIMEOUT":                  "50",
				"WRITE_TIMEOUT":                 "50",
				"OSCAR_SERVICE_PORT":            "8000",
				"YUNIKORN_ENABLE":               "true",
				"YUNIKORN_NAMESPACE":            "testyunikornnamespace",
				"YUNIKORN_CONFIGMAP":            "testyunikornconfigmap",
				"YUNIKORN_CONFIG_FILENAME":      "testyunikornconfigfilename",
			},
			true,
		},
		{
			"Invalid WATCHDOG_WRITE_TIMEOUT",
			map[string]string{
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
				"WATCHDOG_WRITE_TIMEOUT":        "test",
				"WATCHDOG_HEALTHCHECK_INTERVAL": "50",
				"READ_TIMEOUT":                  "50",
				"WRITE_TIMEOUT":                 "50",
				"OSCAR_SERVICE_PORT":            "8000",
				"YUNIKORN_ENABLE":               "true",
				"YUNIKORN_NAMESPACE":            "testyunikornnamespace",
				"YUNIKORN_CONFIGMAP":            "testyunikornconfigmap",
				"YUNIKORN_CONFIG_FILENAME":      "testyunikornconfigfilename",
			},
			true,
		},
		{
			"Invalid WATCHDOG_HEALTHCHECK_INTERVAL",
			map[string]string{
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
				"WATCHDOG_HEALTHCHECK_INTERVAL": "test",
				"READ_TIMEOUT":                  "50",
				"WRITE_TIMEOUT":                 "50",
				"OSCAR_SERVICE_PORT":            "8000",
				"YUNIKORN_ENABLE":               "true",
				"YUNIKORN_NAMESPACE":            "testyunikornnamespace",
				"YUNIKORN_CONFIGMAP":            "testyunikornconfigmap",
				"YUNIKORN_CONFIG_FILENAME":      "testyunikornconfigfilename",
			},
			true,
		},
		{
			"Invalid READ_TIMEOUT",
			map[string]string{
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
				"READ_TIMEOUT":                  "test",
				"WRITE_TIMEOUT":                 "50",
				"OSCAR_SERVICE_PORT":            "8000",
				"YUNIKORN_ENABLE":               "true",
				"YUNIKORN_NAMESPACE":            "testyunikornnamespace",
				"YUNIKORN_CONFIGMAP":            "testyunikornconfigmap",
				"YUNIKORN_CONFIG_FILENAME":      "testyunikornconfigfilename",
			},
			true,
		},
		{
			"Invalid WRITE_TIMEOUT",
			map[string]string{
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
				"WRITE_TIMEOUT":                 "test",
				"OSCAR_SERVICE_PORT":            "8000",
				"YUNIKORN_ENABLE":               "true",
				"YUNIKORN_NAMESPACE":            "testyunikornnamespace",
				"YUNIKORN_CONFIGMAP":            "testyunikornconfigmap",
				"YUNIKORN_CONFIG_FILENAME":      "testyunikornconfigfilename",
			},
			true,
		},
		{
			"Invalid OSCAR_SERVICE_PORT",
			map[string]string{
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
				"OSCAR_SERVICE_PORT":            "test",
				"YUNIKORN_ENABLE":               "true",
				"YUNIKORN_NAMESPACE":            "testyunikornnamespace",
				"YUNIKORN_CONFIGMAP":            "testyunikornconfigmap",
				"YUNIKORN_CONFIG_FILENAME":      "testyunikornconfigfilename",
			},
			true,
		},
		{
			"Invalid YUNIKORN_ENABLE",
			map[string]string{
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
				"YUNIKORN_ENABLE":               "test",
				"YUNIKORN_NAMESPACE":            "testyunikornnamespace",
				"YUNIKORN_CONFIGMAP":            "testyunikornconfigmap",
				"YUNIKORN_CONFIG_FILENAME":      "testyunikornconfigfilename",
			},
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			for k, v := range s.environment {
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

func TestServerlessBackend(t *testing.T) {
	scenarios := []struct {
		name        string
		environment map[string]string
		returnError bool
	}{
		{
			"Valid \"openfaas\"",
			map[string]string{
				"OSCAR_USERNAME":     "testuser",
				"OSCAR_PASSWORD":     "testpass",
				"MINIO_ACCESS_KEY":   "testminioaccess",
				"MINIO_SECRET_KEY":   "testminiosecret",
				"SERVERLESS_BACKEND": "openfaas",
			},
			false,
		},
		{
			"Valid \"knative\"",
			map[string]string{
				"OSCAR_USERNAME":     "testuser",
				"OSCAR_PASSWORD":     "testpass",
				"MINIO_ACCESS_KEY":   "testminioaccess",
				"MINIO_SECRET_KEY":   "testminiosecret",
				"SERVERLESS_BACKEND": "knative",
			},
			false,
		},
		{
			"Valid \"OPENFAAS\"",
			map[string]string{
				"OSCAR_USERNAME":     "testuser",
				"OSCAR_PASSWORD":     "testpass",
				"MINIO_ACCESS_KEY":   "testminioaccess",
				"MINIO_SECRET_KEY":   "testminiosecret",
				"SERVERLESS_BACKEND": "OPENFAAS",
			},
			false,
		},
		{
			"Invalid",
			map[string]string{
				"OSCAR_USERNAME":     "testuser",
				"OSCAR_PASSWORD":     "testpass",
				"MINIO_ACCESS_KEY":   "testminioaccess",
				"MINIO_SECRET_KEY":   "testminiosecret",
				"SERVERLESS_BACKEND": "test",
			},
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			for k, v := range s.environment {
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
