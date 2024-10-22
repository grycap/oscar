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

package imagepuller

import (
	"testing"

	"bou.ke/monkey"
	"github.com/grycap/oscar/v3/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateDaemonset(t *testing.T) {
	cfg := &types.Config{
		ServicesNamespace: "default",
	}
	service := types.Service{
		Name:             "test-service",
		Image:            "test-image",
		ImagePullSecrets: []string{"test-secret"},
	}
	kubeClientset := fake.NewSimpleClientset()

	// Patch the GetDataFromAPI function to return a mock result
	monkey.Patch(watchPods, func(kubernetes.Interface, *types.Config) {
	})

	err := CreateDaemonset(cfg, service, kubeClientset)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	actions := kubeClientset.Actions()
	if len(actions) != 2 {
		t.Errorf("Expected 2 action but got %d", len(actions))
	}
	if actions[0].GetVerb() != "list" || actions[0].GetResource().Resource != "nodes" {
		t.Errorf("Expected create job action but got %v", actions[0])
	}
	if actions[1].GetVerb() != "create" || actions[1].GetResource().Resource != "daemonsets" {
		t.Errorf("Expected create job action but got %v", actions[1])
	}

	daemonset := getDaemonset(cfg, service)

	if daemonset.Name != "image-puller-test-service" {
		t.Errorf("expected daemonset name to be 'image-puller-test-service', got %s", daemonset.Name)
	}

	if daemonset.Namespace != cfg.ServicesNamespace {
		t.Errorf("expected daemonset namespace to be '%s', got %s", cfg.ServicesNamespace, daemonset.Namespace)
	}

	if daemonset.Spec.Template.Spec.Containers[0].Image != service.Image {
		t.Errorf("expected container image to be '%s', got %s", service.Image, daemonset.Spec.Template.Spec.Containers[0].Image)
	}
}
