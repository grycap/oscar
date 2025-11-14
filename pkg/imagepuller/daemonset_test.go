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
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/grycap/oscar/v3/pkg/types"
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

	// Patch the watchPods function to return a mock result
	origWatchPods := watchPodsFunc
	watchPodsFunc = func(kubernetes.Interface, *types.Config) {}
	t.Cleanup(func() { watchPodsFunc = origWatchPods })

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

func TestCreateDaemonsetFailsOnNodeListError(t *testing.T) {
	cfg := &types.Config{ServicesNamespace: "default"}
	service := types.Service{Name: "svc"}
	client := fake.NewSimpleClientset()

	client.Fake.PrependReactor("list", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("boom")
	})

	origWatch := watchPodsFunc
	watchPodsFunc = func(kubernetes.Interface, *types.Config) {}
	t.Cleanup(func() { watchPodsFunc = origWatch })

	err := CreateDaemonset(cfg, service, client)
	if err == nil {
		t.Fatalf("expected error when listing nodes fails")
	}
}

func TestSetWorkingNodes(t *testing.T) {
	defer func(prev int) { workingNodes = prev }(workingNodes)

	tests := []struct {
		name          string
		client        kubernetes.Interface
		expectedCount int
		expectError   bool
	}{
		{
			name: "counts worker nodes",
			client: fake.NewSimpleClientset(
				&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
				&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node2"}},
			),
			expectedCount: 2,
		},
		{
			name: "returns error on list failure",
			client: func() kubernetes.Interface {
				c := fake.NewSimpleClientset()
				c.Fake.PrependReactor("list", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("boom")
				})
				return c
			}(),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workingNodes = 0
			err := setWorkingNodes(tt.client)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if workingNodes != 0 {
					t.Fatalf("expected workingNodes to stay at 0, got %d", workingNodes)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if workingNodes != tt.expectedCount {
				t.Fatalf("expected %d working nodes, got %d", tt.expectedCount, workingNodes)
			}
		})
	}
}

func TestHandleUpdatePodEvent(t *testing.T) {
	defer func(prev int, prevPC PodCounter, prevStopper chan struct{}) {
		workingNodes = prev
		pc = prevPC
		stopper = prevStopper
	}(workingNodes, pc, stopper)

	t.Run("running pod stops informer", func(t *testing.T) {
		workingNodes = 1
		stopper = make(chan struct{}, 1)
		pc = PodCounter{}
		pod := &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}
		handleUpdatePodEvent(nil, pod)
		select {
		case <-stopper:
		case <-time.After(time.Second):
			t.Fatalf("expected stopper to receive signal")
		}
		if pc.wnCount != 1 {
			t.Fatalf("expected wnCount to be 1, got %d", pc.wnCount)
		}
	})

	t.Run("non running pod ignored", func(t *testing.T) {
		workingNodes = 1
		stopper = make(chan struct{}, 1)
		pc = PodCounter{}
		pod := &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending}}
		handleUpdatePodEvent(nil, pod)
		select {
		case <-stopper:
			t.Fatalf("did not expect stopper signal")
		default:
		}
		if pc.wnCount != 0 {
			t.Fatalf("expected wnCount to remain 0, got %d", pc.wnCount)
		}
	})
}

func TestGeneratePodGroupName(t *testing.T) {
	name := generatePodGroupName()
	if !strings.HasPrefix(name, "pod-group-") {
		t.Fatalf("unexpected prefix for %s", name)
	}
	suffix := strings.TrimPrefix(name, "pod-group-")
	if len(suffix) != lengthStr {
		t.Fatalf("expected suffix length %d, got %d", lengthStr, len(suffix))
	}
	for _, c := range suffix {
		if c < 'a' || c > 'z' {
			t.Fatalf("suffix contains non lowercase letter: %s", suffix)
		}
	}
}
