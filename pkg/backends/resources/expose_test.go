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
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
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

	service := types.Service{
		Name: "test-service",
		Expose: types.Expose{
			MinScale:     1,
			MaxScale:     3,
			CpuThreshold: 80,
			SetAuth:      true,
		},
	}
	cfg := &types.Config{ServicesNamespace: "namespace"}

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
		{Verb: "create", Resource: "secrets"},
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
		&appsv1.Deployment{
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
	cfg := &types.Config{ServicesNamespace: "namespace"}

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
		&appsv1.Deployment{
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
	cfg := &types.Config{ServicesNamespace: "namespace"}

	service := types.Service{
		Name: "service",
		Expose: types.Expose{
			MinScale:     1,
			MaxScale:     3,
			CpuThreshold: 80,
			SetAuth:      true,
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
		{Verb: "create", Resource: "secrets"},
	}

	if CompareActions(actions, expected_actions) == false {
		t.Errorf("Expected %v actions but got %v", expected_actions, actions)
	}
}

func TestServiceSpec(t *testing.T) {

	service := types.Service{
		Name: "test-service",
		Expose: types.Expose{
			MinScale:     1,
			MaxScale:     3,
			CpuThreshold: 40,
			APIPort:      8080,
			SetAuth:      true,
		},
	}
	cfg := &types.Config{Namespace: "namespace"}
	res := getServiceSpec(service, cfg)
	if res.Spec.Ports[0].TargetPort.IntVal != 8080 {
		t.Errorf("Expected port 8080 but got %d", res.Spec.Ports[0].Port)
	}
}

func TestHortizontalAutoScaleSpec(t *testing.T) {

	service := types.Service{
		Name: "test-service",
		Expose: types.Expose{
			MinScale:     1,
			MaxScale:     3,
			CpuThreshold: 40,
		},
	}
	cfg := &types.Config{Namespace: "namespace"}
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

func TestListIngress(t *testing.T) {

	K8sObjects := []runtime.Object{
		&netv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-ing",
				Namespace: "namespace",
			},
		},
	}

	kubeClientset := testclient.NewSimpleClientset(K8sObjects...)
	cfg := &types.Config{ServicesNamespace: "namespace"}

	_, err := listIngress(kubeClientset, cfg)

	if err != nil {
		t.Errorf("Error listing ingresses: %v", err)
	}
}

func TestUpdateIngress(t *testing.T) {

	K8sObjects := []runtime.Object{
		&netv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-ing",
				Namespace: "namespace",
			},
		},
	}

	service := types.Service{
		Name: "service",
	}

	kubeClientset := testclient.NewSimpleClientset(K8sObjects...)
	cfg := &types.Config{ServicesNamespace: "namespace"}

	err := updateIngress(service, kubeClientset, cfg)

	if err != nil {
		t.Errorf("Error updating ingress: %v", err)
	}
}

func TestDeleteIngress(t *testing.T) {

	K8sObjects := []runtime.Object{
		&netv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-ing",
				Namespace: "namespace",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-ing-auth-expose",
				Namespace: "namespace",
			},
		},
	}

	kubeClientset := testclient.NewSimpleClientset(K8sObjects...)
	cfg := &types.Config{ServicesNamespace: "namespace"}

	err := deleteIngress("service-ing", kubeClientset, cfg)

	if err != nil {
		t.Errorf("Error deleting ingress: %v", err)
	}
}

func TestUpdateSecret(t *testing.T) {

	K8sObjects := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-auth-expose",
				Namespace: "namespace",
			},
		},
	}
	service := types.Service{
		Name: "service",
	}

	kubeClientset := testclient.NewSimpleClientset(K8sObjects...)
	cfg := &types.Config{ServicesNamespace: "namespace"}

	err := updateSecret(service, kubeClientset, cfg)

	if err != nil {
		t.Errorf("Error updating secret: %v", err)
	}
}

func TestDeleteSecret(t *testing.T) {

	K8sObjects := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-auth-expose",
				Namespace: "namespace",
			},
		},
	}

	kubeClientset := testclient.NewSimpleClientset(K8sObjects...)
	cfg := &types.Config{ServicesNamespace: "namespace"}

	err := deleteSecret("service", kubeClientset, cfg)

	if err != nil {
		t.Errorf("Error deleting secret: %v", err)
	}
}

func TestExistsSecret(t *testing.T) {

	K8sObjects := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-auth-expose",
				Namespace: "namespace",
			},
		},
	}

	kubeClientset := testclient.NewSimpleClientset(K8sObjects...)
	cfg := &types.Config{ServicesNamespace: "namespace"}

	exists := existsSecret("service", kubeClientset, cfg)

	if exists != true {
		t.Errorf("Expected secret to exist but got %v", exists)
	}

	notexists := existsSecret("service1", kubeClientset, cfg)

	if notexists != false {
		t.Errorf("Expected secret not to exist but got %v", notexists)
	}
}
