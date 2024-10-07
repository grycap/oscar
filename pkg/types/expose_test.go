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

	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestCreateExpose(t *testing.T) {

	kubeClientset := testclient.NewSimpleClientset()

	service := Service{
		Name: "test-service",
		Expose: Expose{
			MinScale:     1,
			MaxScale:     3,
			CpuThreshold: 80,
		},
	}
	cfg := &Config{Namespace: "namespace"}

	err := CreateExpose(service, kubeClientset, cfg)

	if err != nil {
		t.Errorf("Error creating expose: %v", err)
	}

	actions := kubeClientset.Actions()
	if len(actions) != 4 {
		t.Errorf("Expected 4 actions but got %d", len(actions))
	}

	if actions[0].GetVerb() != "create" || actions[0].GetResource().Resource != "deployments" {
		t.Errorf("Expected create deployment action but got %v", actions[0])
	}

	if actions[1].GetVerb() != "create" || actions[1].GetResource().Resource != "horizontalpodautoscalers" {
		t.Errorf("Expected create horizontalpodautoscalers action but got %v", actions[1])
	}

	if actions[2].GetVerb() != "create" || actions[2].GetResource().Resource != "services" {
		t.Errorf("Expected create service action but got %v", actions[2])
	}

	if actions[3].GetVerb() != "create" || actions[3].GetResource().Resource != "ingresses" {
		t.Errorf("Expected create ingress action but got %v", actions[3])
	}
}

func TestServiceSpec(t *testing.T) {

	service := Service{
		Name: "test-service",
		Expose: Expose{
			MinScale:     1,
			MaxScale:     3,
			CpuThreshold: 40,
			APIPort:      8080,
		},
	}
	cfg := &Config{Namespace: "namespace"}
	res := getServiceSpec(service, cfg)
	if res.Spec.Ports[0].TargetPort.IntVal != 8080 {
		t.Errorf("Expected port 8080 but got %d", res.Spec.Ports[0].Port)
	}
}

func TestHortizontalAutoScaleSpec(t *testing.T) {

	service := Service{
		Name: "test-service",
		Expose: Expose{
			MinScale:     1,
			MaxScale:     3,
			CpuThreshold: 40,
		},
	}
	cfg := &Config{Namespace: "namespace"}
	res := getHortizontalAutoScaleSpec(service, cfg)
	if *res.Spec.MinReplicas != 1 {
		t.Errorf("Expected min replicas 1 but got %d", res.Spec.MinReplicas)
	}
	if res.Spec.MaxReplicas != 3 {
		t.Errorf("Expected max replicas 3 but got %d", res.Spec.MaxReplicas)
	}
	if *res.Spec.TargetCPUUtilizationPercentage != 40 {
		t.Errorf("Expected target cpu 40 but got %d", res.Spec.TargetCPUUtilizationPercentage)
	}
}
