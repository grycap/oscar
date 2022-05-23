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
	"fmt"
	"net/http"
	"testing"

	"github.com/grycap/oscar/v2/pkg/types"
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

	if back.namespace != testConfig.ServicesNamespace {
		t.Errorf("invalid servicesNamespace. Expected: %s, got: %s", testConfig.ServicesNamespace, back.namespace)
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
		}

		if info.Name != "Knative" {
			t.Errorf("invalid name. Expecting: Knative, got: %s", info.Name)
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
			"Error listing knative services",
			[]k8stesting.SimpleReactor{},
			[]k8stesting.SimpleReactor{
				{
					Verb:     "list",
					Resource: "services",
					Reaction: errorReaction,
				}},
			true,
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
}

func TestKnativeReadService(t *testing.T) {
	//TODO
}

func TestKnativeUpdateService(t *testing.T) {
	//TODO
}

func TestKnativeDeleteService(t *testing.T) {
	//TODO
}

func TestKnativeGetProxyDirector(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	back := MakeKnativeBackend(clientset, fakeConfig, testConfig)

	proxyDirector := back.GetProxyDirector("testService")

	testReq, _ := http.NewRequest(http.MethodPost, "testurl", nil)

	proxyDirector(testReq)

	expectedHost := fmt.Sprintf("%s.%s", "testService", back.namespace)

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
