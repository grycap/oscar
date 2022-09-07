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

package resourcemanager

import (
	"errors"
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var errFake = errors.New("fake error")

func TestUpdateResources(t *testing.T) {
	krm := KubeResourceManager{kubeClientset: fake.NewSimpleClientset()}

	validNodeReactorUnschedulable := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		nodeList := &v1.NodeList{
			Items: []v1.Node{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: v1.NodeSpec{
						Unschedulable: true,
					},
					Status: v1.NodeStatus{},
				},
			},
		}
		return true, nodeList, nil
	}

	validNodeReactorSchedulable := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		nodeList := &v1.NodeList{
			Items: []v1.Node{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: v1.NodeSpec{
						Unschedulable: false,
					},
					Status: v1.NodeStatus{
						Conditions: []v1.NodeCondition{
							{Type: v1.NodeReady,
								Status: v1.ConditionTrue},
						},
					},
				},
			},
		}
		return true, nodeList, nil
	}

	errReactor := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errFake
	}

	scenariosk8s := []struct {
		name        string
		returnError bool
	}{
		{"valid", false},
		{"error getting node list", true},
		{"error getting pod list", true},
	}

	// Tests UpdateResources() and isNodeReady() call
	t.Run("valid schedulable nodes", func(t *testing.T) {
		krm.kubeClientset.(*fake.Clientset).Fake.PrependReactor("list", "nodes", validNodeReactorSchedulable)
		err := krm.UpdateResources()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	//Tests k8s calls to list nodes and list pods
	for _, s := range scenariosk8s {
		t.Run(s.name, func(t *testing.T) {
			if !s.returnError {
				krm.kubeClientset.(*fake.Clientset).Fake.PrependReactor("list", "nodes", validNodeReactorUnschedulable)
				err := krm.UpdateResources()
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if s.name == "error getting pod list" {
					krm.kubeClientset.(*fake.Clientset).Fake.PrependReactor("list", "nodes", validNodeReactorUnschedulable)
					krm.kubeClientset.(*fake.Clientset).Fake.PrependReactor("list", "pods", errReactor)
					err := krm.UpdateResources()
					if err == nil {
						t.Errorf("expecting error got nil")
					}
				}
				if s.name == "error getting node list" {
					krm.kubeClientset.(*fake.Clientset).Fake.PrependReactor("list", "nodes", errReactor)
					err := krm.UpdateResources()
					if err == nil {
						t.Errorf("expecting error got nil")
					}
				}
			}
		})
	}
}

func TestGetNodeAvailableResources(t *testing.T) {
	memorySize := resource.NewQuantity(2*1024*1024*1024, resource.BinarySI)
	cpuSize := resource.NewQuantity(1*1024*1024*1024, resource.DecimalSI)

	validNodeReactorSchedulable := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		nodeList := &v1.NodeList{
			Items: []v1.Node{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec: v1.NodeSpec{
						Unschedulable: false,
					},
					Status: v1.NodeStatus{
						Conditions: []v1.NodeCondition{
							{Type: v1.NodeReady,
								Status: v1.ConditionTrue},
						},
						Allocatable: v1.ResourceList{
							"memory": *memorySize,
							"cpu":    *cpuSize,
						},
					},
				},
			},
		}
		return true, nodeList, nil
	}

	validPodReactor := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		podList := &v1.PodList{
			TypeMeta: metav1.TypeMeta{},
			ListMeta: metav1.ListMeta{},
			Items: []v1.Pod{
				{Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								"memory": *memorySize,
								"cpu":    *cpuSize,
							},
						},
						},
					},
				},
				},
			},
		}
		return true, podList, nil
	}

	krm := KubeResourceManager{
		resources: []nodeResources{
			{
				memory: 2,
				cpu:    1,
			},
		},
		kubeClientset: fake.NewSimpleClientset(),
	}

	krm.kubeClientset.(*fake.Clientset).Fake.PrependReactor("list", "nodes", validNodeReactorSchedulable)
	krm.kubeClientset.(*fake.Clientset).Fake.PrependReactor("list", "pods", validPodReactor)
	err := krm.UpdateResources()
	if err != nil {
		t.Errorf("expected error, got nil")
	}

}

func TestIsSchedulable(t *testing.T) {
	validMemorySize := resource.NewQuantity(2*1024*1024*1024, resource.BinarySI)
	fmt.Printf("memorySize = %v\n", validMemorySize)
	notValidMemorySize := resource.NewQuantity(4*1024*1024*1024, resource.BinarySI)
	fmt.Printf("memorySize = %v\n", notValidMemorySize)
	cpuSize := resource.NewQuantity(1*1024, resource.DecimalSI)
	krm := KubeResourceManager{
		resources: []nodeResources{
			{
				memory: 3000000000,
				cpu:    2000000,
			},
		},
	}
	t.Run("Valid size", func(t *testing.T) {
		validResources := v1.ResourceRequirements{
			Limits: v1.ResourceList{
				"memory": *validMemorySize,
				"cpu":    *cpuSize,
			},
		}
		res := krm.IsSchedulable(validResources)
		if !res {
			t.Errorf("expected true, got false")
		}
	})
	// TODO fix
	t.Run("Not valid size", func(t *testing.T) {
		badResources := v1.ResourceRequirements{
			Limits: v1.ResourceList{
				"memory": *notValidMemorySize,
				"cpu":    *cpuSize,
			},
		}
		res := krm.IsSchedulable(badResources)
		if res {
			t.Errorf("expected false, got true")
		}
	})
}
