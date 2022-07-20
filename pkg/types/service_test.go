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
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/barkimedes/go-deepcopy"
	v1 "k8s.io/api/core/v1"
)

var (
	testService Service = Service{
		Name:      "testname",
		ClusterID: "testcluster",
		Image:     "testimage",
		Alpine:    false,
		Memory:    "1Gi",
		CPU:       "1.0",
		Replicas: []Replica{
			{
				Type:        "oscar",
				ClusterID:   "test",
				ServiceName: "testreplicaname",
				Headers: map[string]string{
					"Authorization": "Bearer testtoken",
				},
			},
		},
		ImagePullSecrets: []string{"testcred1", "testcred2"},
		Script:           "testscript",
		Environment: struct {
			Vars map[string]string `json:"Variables"`
		}{
			Vars: map[string]string{
				"TEST_VAR": "testvalue",
			},
		},
		Annotations: map[string]string{
			"testannotation": "testannotationvalue",
		},
		Labels: map[string]string{
			"testlabel": "testlabelvalue",
		},
		StorageProviders: &StorageProviders{
			MinIO: map[string]*MinIOProvider{
				DefaultProvider: {
					Endpoint:  "http://test.minio.endpoint",
					Verify:    true,
					AccessKey: "testaccesskey",
					SecretKey: "testsecretkey",
					Region:    "testregion",
				},
			},
		},
		Clusters: map[string]Cluster{
			"test": {
				Endpoint:     "https://test.oscar.endpoint",
				AuthUser:     "testuser",
				AuthPassword: "testpass",
				SSLVerify:    true,
			},
		},
	}

	testConfig Config = Config{
		WatchdogMaxInflight:  20,
		WatchdogWriteDebug:   true,
		WatchdogExecTimeout:  60,
		WatchdogReadTimeout:  60,
		WatchdogWriteTimeout: 60,
	}
)

func TestCreateResources(t *testing.T) {
	// Deep copy the testService
	copy, err := deepcopy.Anything(testService)
	if err != nil {
		t.Errorf("unable to deep copy the testService: %v", err)
	}
	svc := copy.(Service)

	scenarios := []struct {
		name        string
		cpu         string
		memory      string
		returnError bool
	}{
		{
			"valid",
			"1Gi",
			"1.0",
			false,
		},
		{
			"invalid memory",
			"1g",
			"1.0",
			true,
		},
		{
			"invalid cpu",
			"1Gi",
			"1cpu",
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			svc.Memory = s.memory
			svc.CPU = s.cpu

			_, err := createResources(&svc)

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

func TestGetMinIOWebhookARN(t *testing.T) {
	arn := testService.GetMinIOWebhookARN()
	expectedARN := "arn:minio:sqs:testregion:testname:webhook"
	if arn != expectedARN {
		t.Errorf("invalid ARN. Expected: %s, got: %s", expectedARN, arn)
	}
}

func TestGetSupervisorPath(t *testing.T) {
	// Deep copy the testService
	copy, err := deepcopy.Anything(testService)
	if err != nil {
		t.Errorf("unable to deep copy the testService: %v", err)
	}
	svc := copy.(Service)

	expectedDefault := fmt.Sprintf("%s/%s", VolumePath, SupervisorName)
	expectedAlpine := fmt.Sprintf("%s/%s/%s", VolumePath, AlpineDirectory, SupervisorName)

	path := svc.GetSupervisorPath()

	if path != expectedDefault {
		t.Errorf("invalid supervisor path. Expected: %s, got: %s", expectedDefault, path)
	}

	// Set Alpine to true and test it
	svc.Alpine = true

	path = svc.GetSupervisorPath()

	if path != expectedAlpine {
		t.Errorf("invalid supervisor path. Expected: %s, got: %s", expectedAlpine, path)
	}
}

func TestConvertEnvVars(t *testing.T) {
	vars := map[string]string{
		"TEST": "test",
	}

	expected := []v1.EnvVar{
		{Name: "TEST", Value: "test"},
	}

	res := convertEnvVars(vars)

	if res[0].Name != expected[0].Name && res[0].Value != expected[0].Value {
		t.Errorf("invalid conversion of environment variables. Expected: %v, got %v", expected, res)
	}
}

func TestSetImagePullSecrets(t *testing.T) {
	secrets := []string{"testcred1"}

	expected := []v1.LocalObjectReference{
		{Name: "testcred1"},
	}

	result := setImagePullSecrets(secrets)
	if result[0].Name != expected[0].Name {
		t.Errorf("invalid conversion of local object. Expected: %v, got %v", expected, result)
	}
}

func TestToYAML(t *testing.T) {
	expected := `name: testname
cluster_id: testcluster
memory: 1Gi
cpu: "1.0"
total_memory: ""
total_cpu: ""
synchronous:
  min_scale: 0
  max_scale: 0
replicas:
- type: oscar
  cluster_id: test
  service_name: testreplicaname
  url: ""
  ssl_verify: false
  priority: 0
  headers:
    Authorization: Bearer testtoken
log_level: ""
image: testimage
alpine: false
token: ""
input: []
output: []
script: testscript
image_pull_secrets:
- testcred1
- testcred2
environment:
  Variables:
    TEST_VAR: testvalue
annotations:
  testannotation: testannotationvalue
labels:
  testlabel: testlabelvalue
storage_providers:
  minio:
    default:
      endpoint: http://test.minio.endpoint
      verify: true
      access_key: testaccesskey
      secret_key: testsecretkey
      region: testregion
clusters:
  test:
    endpoint: https://test.oscar.endpoint
    auth_user: testuser
    auth_password: testpass
    ssl_verify: true
`

	str, _ := testService.ToYAML()

	if str != expected {
		t.Errorf("invalid YAML definition. Expected:\n%s\n-----------------------------\nGot:\n%s", expected, str)
	}
}

func TestToYAMLError(t *testing.T) {
	svc := Service{}
	YAMLMarshal = func(interface{}) ([]byte, error) {
		return nil, errors.New("test error")
	}

	str, err := svc.ToYAML()

	if err == nil {
		t.Errorf("expecting error, got:\n%s\n", str)
	}
}

func TestToPodSpec(t *testing.T) {
	scenarios := []struct {
		name        string
		cpu         string
		memory      string
		returnError bool
	}{
		{
			"valid resources",
			"1Gi",
			"1.0",
			false,
		},
		{
			"invalid resources",
			"1g",
			"1cpu",
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			// Deep copy the testService
			copy, err := deepcopy.Anything(testService)
			if err != nil {
				t.Errorf("unable to deep copy the testService: %v", err)
			}
			svc := copy.(Service)
			// Assign resources from scenario
			svc.Memory = s.memory
			svc.CPU = s.cpu
			//svc.ImagePullSecrets = []string{"testcred"}

			podSpec, err := svc.ToPodSpec(&testConfig)

			if s.returnError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if err = checkEnvVars(&testConfig, podSpec); err != nil {
					t.Error(err.Error())
				}
			}
		})
	}
}

func checkEnvVars(cfg *Config, podSpec *v1.PodSpec) error {
	var expected string
	var found = []string{}

	for _, envVar := range podSpec.Containers[0].Env {
		switch envVar.Name {
		case "max_inflight":
			expected = strconv.Itoa(cfg.WatchdogMaxInflight)
			if envVar.Value != expected {
				return fmt.Errorf("the max_inflight environment variable has not the correct value. Expected: %s, got: %s", expected, envVar.Value)
			}
		case "write_debug":
			expected = strconv.FormatBool(cfg.WatchdogWriteDebug)
			if envVar.Value != expected {
				return fmt.Errorf("the write_debug environment variable has not the correct value. Expected: %s, got: %s", expected, envVar.Value)
			}
		case "exec_timeout":
			expected = strconv.Itoa(cfg.WatchdogExecTimeout)
			if envVar.Value != expected {
				return fmt.Errorf("the exec_timeout environment variable has not the correct value. Expected: %s, got: %s", expected, envVar.Value)
			}
		case "read_timeout":
			expected = strconv.Itoa(cfg.WatchdogReadTimeout)
			if envVar.Value != expected {
				return fmt.Errorf("the read_timeout environment variable has not the correct value. Expected: %s, got: %s", expected, envVar.Value)
			}
		case "write_timeout":
			expected = strconv.Itoa(cfg.WatchdogWriteTimeout)
			if envVar.Value != expected {
				return fmt.Errorf("the write_timeout environment variable has not the correct value. Expected: %s, got: %s", expected, envVar.Value)
			}
		default:
			continue
		}
		found = append(found, envVar.Name)
	}

	if len(found) != 5 {
		return fmt.Errorf("only the following watchdog environment variables are correctly defined: %v", found)
	}

	return nil
}
