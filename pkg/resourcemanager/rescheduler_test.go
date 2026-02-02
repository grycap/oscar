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
	"bytes"
	"log"
	"testing"
	"time"

	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/types"
	jobv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetReSchedulablePods(t *testing.T) {
	// Define test namespace
	namespace := "test-namespace"

	// Create test pods
	pods := &v1.PodList{
		Items: []v1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: namespace,
					Labels: map[string]string{
						types.ServiceLabel:        "service1",
						types.ReSchedulerLabelKey: "10",
					},
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-15 * time.Second)},
				},
				Status: v1.PodStatus{
					Phase: v1.PodPending,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod2",
					Namespace: namespace,
					Labels: map[string]string{
						types.ServiceLabel:        "service2",
						types.ReSchedulerLabelKey: "20",
					},
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-5 * time.Second)},
				},
				Status: v1.PodStatus{
					Phase: v1.PodPending,
				},
			},
		},
	}

	// Create a fake Kubernetes client
	kubeClientset := fake.NewSimpleClientset(pods)

	// Call the function to test
	reSchedulablePods, err := getReSchedulablePods(kubeClientset, namespace)
	if err != nil {
		t.Fatalf("error getting reschedulable pods: %v", err)
	}

	// Check the results
	if len(reSchedulablePods) != 1 {
		t.Errorf("expected 1 reschedulable pod, got %d", len(reSchedulablePods))
	}

	if reSchedulablePods[0].Name != "pod1" {
		t.Errorf("expected pod1 to be reschedulable, got %s", reSchedulablePods[0].Name)
	}
}

func TestGetReScheduleInfos(t *testing.T) {
	// Define test namespace
	namespace := "test-namespace"

	// Create test pods
	pods := []v1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod1",
				Namespace: namespace,
				Labels: map[string]string{
					types.ServiceLabel:        "service1",
					types.ReSchedulerLabelKey: "10",
				},
				CreationTimestamp: metav1.Time{Time: time.Now().Add(-15 * time.Second)},
			},
			Status: v1.PodStatus{
				Phase: v1.PodPending,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod2",
				Namespace: namespace,
				Labels: map[string]string{
					types.ServiceLabel:        "service2",
					types.ReSchedulerLabelKey: "20",
				},
				CreationTimestamp: metav1.Time{Time: time.Now().Add(-5 * time.Second)},
			},
			Status: v1.PodStatus{
				Phase: v1.PodPending,
			},
		},
	}

	back := backends.MakeFakeBackend()
	// Call the function to test
	reScheduleInfos := getReScheduleInfos(pods, back)
	if reScheduleInfos == nil {
		t.Fatalf("error getting reschedule infos")
	}

}

func TestStartReScheduler(t *testing.T) {
	// Define test namespace
	namespace := "test-namespace"

	// Create test pods
	pods := &v1.PodList{
		Items: []v1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: namespace,
					Labels: map[string]string{
						types.ServiceLabel:        "service1",
						types.ReSchedulerLabelKey: "10",
						"job-name":                "job1",
					},
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-15 * time.Second)},
				},
				Status: v1.PodStatus{
					Phase: v1.PodPending,
				},
			},
		},
	}
	jobs := &jobv1.JobList{
		Items: []jobv1.Job{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "job1",
					Namespace: namespace,
				},
			},
		},
	}

	// Create a fake Kubernetes client
	kubeClientset := fake.NewSimpleClientset(pods, jobs)
	back := backends.MakeFakeBackend()
	cfg := &types.Config{
		ReSchedulerInterval: 5,
		ServicesNamespace:   namespace,
	}

	// Mock the Delegate function using test hook variable
	origDelegate := delegateJobFunc
	delegateJobFunc = func(*types.Service, string, string, *log.Logger, *types.Config, kubernetes.Interface) error {
		return nil
	}
	t.Cleanup(func() { delegateJobFunc = origDelegate })
	var buf bytes.Buffer
	reSchedulerLogger = log.New(&buf, "[RE-SCHEDULER] ", log.Flags())
	// Call the function to test
	go StartReScheduler(cfg, back, kubeClientset)
	time.Sleep(2 * time.Second)

	if buf.String() != "" {
		t.Fatalf("error starting rescheduler: %v", buf.String())
	}
}

func TestGetEventEnvVar(t *testing.T) {
	podSpec := v1.PodSpec{
		Containers: []v1.Container{
			{
				Name: types.ContainerName,
				Env: []v1.EnvVar{
					{Name: "other", Value: "x"},
					{Name: types.EventVariable, Value: "payload"},
				},
			},
		},
	}

	if ev := getEvent(podSpec); ev != "payload" {
		t.Fatalf("expected payload event, got %s", ev)
	}

	emptySpec := v1.PodSpec{Containers: []v1.Container{{Name: "faas-container"}}}
	if ev := getEvent(emptySpec); ev != "" {
		t.Fatalf("expected empty event, got %s", ev)
	}
}
