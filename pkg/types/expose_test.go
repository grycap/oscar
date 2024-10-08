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

	v1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type Action struct {
	Verb     string
	Resource string
}

func CompareActions(actions []k8stesting.Action, expected_actions []Action) bool {
	if len(actions) != len(expected_actions) {
		return false
	}

	for i, action := range actions {
		if action.GetVerb() != expected_actions[i].Verb || action.GetResource().Resource != expected_actions[i].Resource {
			return false
		}
	}
	return true
}

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
	cfg := &Config{ServicesNamespace: "namespace"}

	err := CreateExpose(service, kubeClientset, cfg)

	if err != nil {
		t.Errorf("Error creating expose: %v", err)
	}

	actions := kubeClientset.Actions()
	expected_actions := []Action{
		{Verb: "create", Resource: "deployments"},
		{Verb: "create", Resource: "horizontalpodautoscalers"},
		{Verb: "create", Resource: "services"},
		{Verb: "create", Resource: "ingresses"},
	}

	if CompareActions(actions, expected_actions) == false {
		t.Errorf("Expected %v actions but got %v", expected_actions, actions)
	}
}

func TestDeleteExpose(t *testing.T) {

	K8sObjects := []runtime.Object{
		&autoscalingv1.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-hpa",
				Namespace: "namespace",
			},
		},
		&v1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-dlp",
				Namespace: "namespace",
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-svc",
				Namespace: "namespace",
			},
		},
	}

	kubeClientset := testclient.NewSimpleClientset(K8sObjects...)
	cfg := &Config{ServicesNamespace: "namespace"}

	err := DeleteExpose("service", kubeClientset, cfg)

	if err != nil {
		t.Errorf("Error creating expose: %v", err)
	}

	actions := kubeClientset.Actions()

	expected_actions := []Action{
		{Verb: "delete", Resource: "horizontalpodautoscalers"},
		{Verb: "delete", Resource: "deployments"},
		{Verb: "delete", Resource: "services"},
		{Verb: "get", Resource: "ingresses"},
		{Verb: "delete-collection", Resource: "pods"},
	}

	if CompareActions(actions, expected_actions) == false {
		t.Errorf("Expected %v actions but got %v", expected_actions, actions)
	}
}

func TestUpdateExpose(t *testing.T) {

	K8sObjects := []runtime.Object{
		&autoscalingv1.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-hpa",
				Namespace: "namespace",
			},
		},
		&v1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-dlp",
				Namespace: "namespace",
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-svc",
				Namespace: "namespace",
			},
		},
	}

	kubeClientset := testclient.NewSimpleClientset(K8sObjects...)
	cfg := &Config{ServicesNamespace: "namespace"}

	service := Service{
		Name: "service",
		Expose: Expose{
			MinScale:     1,
			MaxScale:     3,
			CpuThreshold: 80,
		},
	}

	err := UpdateExpose(service, kubeClientset, cfg)

	if err != nil {
		t.Errorf("Error creating expose: %v", err)
	}

	actions := kubeClientset.Actions()

	expected_actions := []Action{
		{Verb: "get", Resource: "deployments"},
		{Verb: "update", Resource: "deployments"},
		{Verb: "get", Resource: "horizontalpodautoscalers"},
		{Verb: "update", Resource: "horizontalpodautoscalers"},
		{Verb: "update", Resource: "services"},
		{Verb: "get", Resource: "ingresses"},
		{Verb: "create", Resource: "ingresses"},
	}

	if CompareActions(actions, expected_actions) == false {
		t.Errorf("Expected %v actions but got %v", expected_actions, actions)
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
