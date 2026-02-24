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

package backends

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/grycap/oscar/v3/pkg/types"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
	knv1 "knative.dev/serving/pkg/apis/serving/v1"
	knFake "knative.dev/serving/pkg/client/clientset/versioned/fake"
)

var (
	fakeConfig = &rest.Config{}

	knServiceListReactor = k8stesting.SimpleReactor{
		Verb:     "list",
		Resource: "services",
		Reaction: func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			knSvcList := &knv1.ServiceList{
				Items: []knv1.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test",
							Namespace: "testnamespace",
						},
					},
				},
			}
			return true, knSvcList, nil
		},
	}

	knGetSvcReactor = k8stesting.SimpleReactor{
		Verb:     "get",
		Resource: "services",
		Reaction: func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			knSvc := &knv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Namespace:   "testnamespace",
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
			}
			return true, knSvc, nil
		},
	}

	knUpdateSvcReactor = k8stesting.SimpleReactor{
		Verb:     "update",
		Resource: "services",
		Reaction: func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			knSvc := &knv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testnamespace",
				},
			}
			return true, knSvc, nil
		},
	}
)

type knativeBackendTestScenario struct {
	name        string
	k8sReactors []k8stesting.SimpleReactor
	knReactors  []k8stesting.SimpleReactor
	returnError bool
}

func TestMakeKnativeBackend(t *testing.T) {
	fakeClientset := fake.NewSimpleClientset()

	back := MakeKnativeBackend(fakeClientset, fakeConfig, testConfig)

	if back.kubeClientset != fakeClientset {
		t.Error("error setting the kubernetes clientset")
	}

	if back.config != testConfig {
		t.Error("error setting the config")
	}
}

func TestKnativeGetInfo(t *testing.T) {
	fakeClientset := fake.NewSimpleClientset()

	back := MakeKnativeBackend(fakeClientset, fakeConfig, testConfig)

	t.Run("valid version", func(t *testing.T) {
		// Set fake clientset and invalidDiscoveryReactor
		back.knClientset = knFake.NewSimpleClientset()

		info := back.GetInfo()

		if info == nil {
			t.Error("invalid info. Got: nil")
		} else {
			if info.Name != "Knative" {
				t.Errorf("invalid name. Expecting: Knative, got: %s", info.Name)
			}
		}
	})
}

func TestKnativeListServices(t *testing.T) {
	scenarios := []knativeBackendTestScenario{
		{
			"Valid",
			[]k8stesting.SimpleReactor{
				{
					Verb:     "get",
					Resource: "configmaps",
					Reaction: validConfigMapReaction,
				},
			},
			[]k8stesting.SimpleReactor{knServiceListReactor},
			false,
		},
		{
			"Error getting the configMap",
			[]k8stesting.SimpleReactor{
				{
					Verb:     "get",
					Resource: "configmaps",
					Reaction: errorReaction,
				},
			},
			[]k8stesting.SimpleReactor{knServiceListReactor},
			false,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset()

			back := MakeKnativeBackend(fakeClientset, fakeConfig, testConfig)

			back.knClientset = knFake.NewSimpleClientset()

			for _, r := range s.k8sReactors {
				back.kubeClientset.(*fake.Clientset).Fake.PrependReactor(r.Verb, r.Resource, r.Reaction)
			}

			for _, r := range s.knReactors {
				back.knClientset.(*knFake.Clientset).Fake.PrependReactor(r.Verb, r.Resource, r.Reaction)
			}

			svcList, err := back.ListServices()

			if s.returnError {
				if err == nil {
					t.Errorf("expected error, got: %v", svcList)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestKnativeCreateService(t *testing.T) {
	scenarios := []knativeBackendTestScenario{
		{
			"Valid",
			[]k8stesting.SimpleReactor{},
			[]k8stesting.SimpleReactor{},
			false,
		},
		{
			"Error creating configMap",
			[]k8stesting.SimpleReactor{
				{
					Verb:     "create",
					Resource: "configmaps",
					Reaction: errorReaction,
				},
			},
			[]k8stesting.SimpleReactor{},
			true,
		},
		{
			"Error creating knative service",
			[]k8stesting.SimpleReactor{},
			[]k8stesting.SimpleReactor{
				{
					Verb:     "create",
					Resource: "services",
					Reaction: errorReaction,
				},
			},
			true,
		},
		{
			"Error creating knative service and deleting configMap",
			[]k8stesting.SimpleReactor{
				{
					Verb:     "delete",
					Resource: "configmaps",
					Reaction: errorReaction,
				},
			},
			[]k8stesting.SimpleReactor{
				{
					Verb:     "create",
					Resource: "services",
					Reaction: errorReaction,
				},
			},
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset()

			back := MakeKnativeBackend(fakeClientset, fakeConfig, testConfig)

			back.knClientset = knFake.NewSimpleClientset()

			testService := types.Service{
				Name:   "test",
				Labels: map[string]string{},
			}

			for _, r := range s.k8sReactors {
				back.kubeClientset.(*fake.Clientset).Fake.PrependReactor(r.Verb, r.Resource, r.Reaction)
			}

			for _, r := range s.knReactors {
				back.knClientset.(*knFake.Clientset).Fake.PrependReactor(r.Verb, r.Resource, r.Reaction)
			}

			err := back.CreateService(testService)

			if s.returnError {
				if err == nil {
					t.Error("expected error, got: nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}

	t.Run("error creating the podSpec", func(t *testing.T) {
		fakeClientset := fake.NewSimpleClientset()

		back := MakeKnativeBackend(fakeClientset, fakeConfig, testConfig)

		back.knClientset = knFake.NewSimpleClientset()

		// Return error creating the podSpec (invalid resources)
		invalidService := types.Service{
			Name:   "test",
			CPU:    "dfasdf",
			Labels: map[string]string{},
		}

		// Error deleting configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "configmaps", errorReaction)

		if err := back.CreateService(invalidService); err == nil {
			t.Error("expected error, got: nil")
		}
	})

	t.Run("exposed service skips knative service creation", func(t *testing.T) {
		fakeClientset := fake.NewSimpleClientset()

		back := MakeKnativeBackend(fakeClientset, fakeConfig, testConfig)
		back.knClientset = knFake.NewSimpleClientset()

		// Fail if Knative Service create is called.
		back.knClientset.(*knFake.Clientset).Fake.PrependReactor("create", "services", errorReaction)

		exposedService := types.Service{
			Name:   "test-exposed",
			Image:  "nginx",
			Script: "echo test",
			Labels: map[string]string{},
			Expose: types.Expose{
				APIPort: 80,
			},
		}

		if err := back.CreateService(exposedService); err != nil {
			t.Fatalf("unexpected error creating exposed service: %v", err)
		}

		deployments, err := back.kubeClientset.AppsV1().Deployments(testConfig.ServicesNamespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("error listing exposed deployments: %v", err)
		}
		if len(deployments.Items) != 1 {
			t.Fatalf("expected 1 exposed deployment, got %d", len(deployments.Items))
		}
	})
}

func TestKnativeReadService(t *testing.T) {
	scenarios := []knativeBackendTestScenario{
		{
			"Error getting configmap",
			[]k8stesting.SimpleReactor{
				{
					Verb:     "get",
					Resource: "configmaps",
					Reaction: errorReaction,
				},
			},
			[]k8stesting.SimpleReactor{knGetSvcReactor},
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			t.Logf("%v", s.name)
			fakeClientset := fake.NewSimpleClientset()
			back := MakeKnativeBackend(fakeClientset, fakeConfig, testConfig)
			back.knClientset = knFake.NewSimpleClientset()

			for _, r := range s.k8sReactors {
				back.kubeClientset.(*fake.Clientset).Fake.PrependReactor(r.Verb, r.Resource, r.Reaction)
			}

			for _, r := range s.knReactors {
				back.knClientset.(*knFake.Clientset).Fake.PrependReactor(r.Verb, r.Resource, r.Reaction)
			}

			// Read service
			svc, err := back.ReadService(testConfig.ServicesNamespace, "test")
			if s.returnError {
				if err == nil {
					t.Errorf("expected error, got: %v", svc)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

		})
	}
	t.Run("Valid read service scenario", func(t *testing.T) {
		fakeClientset := fake.NewSimpleClientset()
		back := MakeKnativeBackend(fakeClientset, fakeConfig, testConfig)
		back.knClientset = knFake.NewSimpleClientset()

		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapReaction)

		back.knClientset.(*knFake.Clientset).Fake.PrependReactor("get", "services", knGetSvcReactor.React)

		// Read service
		_, err := back.ReadService(testConfig.ServicesNamespace, "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

	})

}

func TestKnativeUpdateService(t *testing.T) {
	newService := types.Service{
		Name:   "test",
		CPU:    "2.0",
		Labels: map[string]string{},
		Annotations: map[string]string{
			"testannotationkey": "testannotationvalue",
		},
	}

	newInvalidService := types.Service{
		Name:        "test",
		CPU:         "asdfg",
		Labels:      map[string]string{},
		Annotations: map[string]string{},
	}

	scenarios := []knativeBackendTestScenario{
		{
			"Valid",
			[]k8stesting.SimpleReactor{
				{
					Verb:     "get",
					Resource: "configmaps",
					Reaction: validConfigMapReaction,
				},
				{
					Verb:     "update",
					Resource: "configmaps",
					Reaction: validConfigMapReaction,
				},
			},
			[]k8stesting.SimpleReactor{knGetSvcReactor, knUpdateSvcReactor},
			false,
		},
		{
			"Error getting old knative service",
			[]k8stesting.SimpleReactor{},
			[]k8stesting.SimpleReactor{
				{
					Verb:     "get",
					Resource: "services",
					Reaction: errorReaction,
				},
			},
			true,
		},
		{
			"Error getting configmap",
			[]k8stesting.SimpleReactor{
				{
					Verb:     "get",
					Resource: "configmaps",
					Reaction: errorReaction,
				},
			},
			[]k8stesting.SimpleReactor{knGetSvcReactor},
			true,
		},
		{
			"Error updating configmap",
			[]k8stesting.SimpleReactor{
				{
					Verb:     "get",
					Resource: "configmaps",
					Reaction: validConfigMapReaction,
				},
				{
					Verb:     "update",
					Resource: "configmaps",
					Reaction: errorReaction,
				},
			},
			[]k8stesting.SimpleReactor{knGetSvcReactor},
			true,
		},
		{
			"Error updating service",
			[]k8stesting.SimpleReactor{
				{
					Verb:     "get",
					Resource: "configmaps",
					Reaction: validConfigMapReaction,
				},
				{
					Verb:     "update",
					Resource: "configmaps",
					Reaction: validConfigMapReaction,
				},
			},
			[]k8stesting.SimpleReactor{
				knGetSvcReactor,
				{
					Verb:     "update",
					Resource: "services",
					Reaction: errorReaction,
				},
			},
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			fakeClientset := fake.NewSimpleClientset()
			back := MakeKnativeBackend(fakeClientset, fakeConfig, testConfig)
			back.knClientset = knFake.NewSimpleClientset()

			for _, r := range s.k8sReactors {
				back.kubeClientset.(*fake.Clientset).Fake.PrependReactor(r.Verb, r.Resource, r.Reaction)
			}

			for _, r := range s.knReactors {
				back.knClientset.(*knFake.Clientset).Fake.PrependReactor(r.Verb, r.Resource, r.Reaction)
			}

			// Update with valid service
			err := back.UpdateService(newService)
			if s.returnError {
				if err == nil {
					t.Error("expected error, got: nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}

	// Error creating service definition and restoring old configMap
	t.Run("Error creating service definition and restoring old configMap", func(t *testing.T) {
		fakeClientset := fake.NewSimpleClientset()

		back := MakeKnativeBackend(fakeClientset, fakeConfig, testConfig)
		back.knClientset = knFake.NewSimpleClientset()

		var configMapReactorCounter = new(int)
		customConfigMapReactor := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			cm := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testnamespace",
				},
				Data: map[string]string{
					types.ScriptFileName: "testscript",
					types.FDLFileName:    testFDL,
				},
			}

			if *configMapReactorCounter == 0 {
				*configMapReactorCounter++
				return true, cm, nil
			}

			*configMapReactorCounter++
			return true, nil, errFake
		}

		// Return valid configmap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapReaction)

		// Return valid knative service
		back.knClientset.(*knFake.Clientset).Fake.PrependReactor("get", "services", knGetSvcReactor.Reaction)

		// Return error updating knative service
		back.knClientset.(*knFake.Clientset).Fake.PrependReactor("update", "services", errorReaction)

		// Custom reactor for configmap update
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("update", "configmaps", customConfigMapReactor)

		// Call
		err := back.UpdateService(newInvalidService)
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	// Error updating service and restoring old configMap
	t.Run("Error updating service and restoring old configMap", func(t *testing.T) {
		fakeClientset := fake.NewSimpleClientset()

		back := MakeKnativeBackend(fakeClientset, fakeConfig, testConfig)
		back.knClientset = knFake.NewSimpleClientset()

		var configMapReactorCounter = new(int)
		customConfigMapReactor := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			cm := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testnamespace",
				},
				Data: map[string]string{
					types.ScriptFileName: "testscript",
					types.FDLFileName:    testFDL,
				},
			}

			if *configMapReactorCounter == 0 {
				*configMapReactorCounter++
				return true, cm, nil
			}

			*configMapReactorCounter++
			return true, nil, errFake
		}

		// Return valid configmap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapReaction)

		// Return valid knative service
		back.knClientset.(*knFake.Clientset).Fake.PrependReactor("get", "services", knGetSvcReactor.Reaction)

		// Return error updating knative service
		back.knClientset.(*knFake.Clientset).Fake.PrependReactor("update", "services", errorReaction)

		// Custom reactor for configmap update
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("update", "configmaps", customConfigMapReactor)

		// Call
		err := back.UpdateService(newService)
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})
}

func TestKnativeDeleteService(t *testing.T) {
	testService := types.Service{
		Name: "test",
	}
	scenarios := []knativeBackendTestScenario{
		{
			"Error deleting knative service",
			[]k8stesting.SimpleReactor{},
			[]k8stesting.SimpleReactor{
				{
					Verb:     "delete",
					Resource: "services",
					Reaction: errorReaction,
				}},
			true,
		},
		{
			"Knative service not found should not fail delete",
			[]k8stesting.SimpleReactor{},
			[]k8stesting.SimpleReactor{
				{
					Verb:     "delete",
					Resource: "services",
					Reaction: func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewNotFound(v1.Resource("services"), "test")
					},
				},
			},
			false,
		},
		{
			"Error deleting configmap",
			[]k8stesting.SimpleReactor{
				{
					Verb:     "delete",
					Resource: "configmaps",
					Reaction: errorReaction,
				},
			},
			[]k8stesting.SimpleReactor{
				{
					Verb:     "delete",
					Resource: "services",
					Reaction: validDeleteReaction,
				},
			},
			false,
		},
		{
			"Error deleting jobs",
			[]k8stesting.SimpleReactor{
				{
					Verb:     "delete-collection",
					Resource: "jobs",
					Reaction: errorReaction,
				},
			},
			[]k8stesting.SimpleReactor{
				{
					Verb:     "delete",
					Resource: "services",
					Reaction: validDeleteReaction,
				},
			},
			false,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			t.Logf("%v", s.name)
			fakeClientset := fake.NewSimpleClientset()
			back := MakeKnativeBackend(fakeClientset, fakeConfig, testConfig)
			back.knClientset = knFake.NewSimpleClientset()

			for _, r := range s.k8sReactors {
				back.kubeClientset.(*fake.Clientset).Fake.PrependReactor(r.Verb, r.Resource, r.Reaction)
			}

			for _, r := range s.knReactors {
				back.knClientset.(*knFake.Clientset).Fake.PrependReactor(r.Verb, r.Resource, r.Reaction)
			}

			// Delete service
			err := back.DeleteService(testService)
			if s.returnError {
				if err == nil {
					t.Error("expected error, got: nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

		})
	}
}

func TestKnativeGetProxyDirector(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	back := MakeKnativeBackend(clientset, fakeConfig, testConfig)

	proxyDirector := back.GetProxyDirector("testService")

	testReq, _ := http.NewRequest(http.MethodPost, "testurl", nil)

	proxyDirector(testReq)

	expectedHost := fmt.Sprintf("%s.%s", "testService", testConfig.ServicesNamespace)

	if testReq.Host != expectedHost && testReq.URL.Host != expectedHost {
		t.Errorf("invalid host. Expected: %s, got: %s / (URL) %s", expectedHost, testReq.Host, testReq.URL.Host)
	}
}

func TestKnativeKubeGetKubeClientset(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	back := MakeKnativeBackend(clientset, fakeConfig, testConfig)

	if clientset != back.GetKubeClientset() {
		t.Error("the clientset obtained is not the same")
	}
}
