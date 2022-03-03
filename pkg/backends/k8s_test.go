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
	"testing"

	"github.com/grycap/oscar/v2/pkg/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var (
	testConfig *types.Config = &types.Config{
		ServicesNamespace: "testnamespace",
	}

	testFDL string = `name: testname
memory: 1Gi
cpu: "1.0"
total_memory: ""
total_cpu: ""
log_level: ""
image: testimage
alpine: false
token: ""
input: []
output: []
script: testscript
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
`

	errorReactor = func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errFake
	}
)

func TestMakeKubeBackend(t *testing.T) {
	fakeClientset := fake.NewSimpleClientset()

	back := MakeKubeBackend(fakeClientset, testConfig)

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

func TestKubeGetInfo(t *testing.T) {
	fakeClientset := fake.NewSimpleClientset()

	back := MakeKubeBackend(fakeClientset, testConfig)

	if info := back.GetInfo(); info != nil {
		t.Errorf("invalid info. Expecting: nil, got: %v", info)
	}
}

func TestKubeListServices(t *testing.T) {
	validPodTemplateListReactor := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		podTemplateList := &v1.PodTemplateList{
			Items: []v1.PodTemplate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "testnamespace",
					},
					Template: v1.PodTemplateSpec{},
				},
			},
		}
		return true, podTemplateList, nil
	}

	validConfigMapReactor := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		validCM := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testnamespace",
			},
			Data: map[string]string{
				types.ScriptFileName: "testscript",
				types.FDLFileName:    testFDL,
			},
		}
		return true, validCM, nil
	}

	t.Run("valid list", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return a valid PodTemplateList
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("list", "podtemplates", validPodTemplateListReactor)

		// Return a valid configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapReactor)

		// Call
		_, err := back.ListServices()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("listing podTemplates throws an error", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return an error listing  PodTemplates
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("list", "podtemplates", errorReactor)

		// Call
		_, err := back.ListServices()
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("getServiceFromFDL throws an error", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return a valid PodTemplateList
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("list", "podtemplates", validPodTemplateListReactor)

		// Return an error listing  PodTemplates
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", errorReactor)

		// Call
		_, err := back.ListServices()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestKubeCreateService(t *testing.T) {
	testService := types.Service{
		Name: "test",
	}

	errorConfigMapReactor := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errFake
	}

	errorPodTemplateReactor := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errFake
	}

	t.Run("valid", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		err := back.CreateService(testService)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("error creating service's YAML", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error when creating the configMap
		oldYAMLMarshal := types.YAMLMarshal
		types.YAMLMarshal = func(interface{}) ([]byte, error) {
			return nil, errFake
		}
		defer func() {
			types.YAMLMarshal = oldYAMLMarshal
		}()

		err := back.CreateService(testService)

		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error creating configMap", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error when creating the configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("create", "configmaps", errorConfigMapReactor)

		err := back.CreateService(testService)

		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error toPodSpec no error deleting configMap", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error when creating the podSpec (invalid resources)
		invalidService := types.Service{
			Name: "test",
			CPU:  "dfasdf",
		}

		err := back.CreateService(invalidService)

		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error toPodSpec and error deleting configMap", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error when creating the podSpec (invalid resources)
		invalidService := types.Service{
			Name: "test",
			CPU:  "dfasdf",
		}

		// Return error when deleting the configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "configmaps", errorConfigMapReactor)

		err := back.CreateService(invalidService)

		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error creating podTemplate no error deleting", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error when creating the podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("create", "podtemplates", errorPodTemplateReactor)

		err := back.CreateService(testService)

		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error creating podTemplate and error deleting", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error when creating the podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("create", "podtemplates", errorPodTemplateReactor)

		// Return error when deleting the configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "configmaps", errorConfigMapReactor)

		err := back.CreateService(testService)

		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})
}

func TestKubeReadService(t *testing.T) {
	validPodTemplateReactor := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		podTemplate := &v1.PodTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testnamespace",
			},
			Template: v1.PodTemplateSpec{},
		}
		return true, podTemplate, nil
	}

	validConfigMapReactor := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
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
		return true, cm, nil
	}

	t.Run("valid", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return valid podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "podtemplates", validPodTemplateReactor)

		// Return valid ConfigMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapReactor)

		_, err := back.ReadService("test")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid podTemplate", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error getting podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "podtemplates", errorReactor)

		_, err := back.ReadService("test")

		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("valid", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return valid podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "podtemplates", validPodTemplateReactor)

		// Return error getting ConfigMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", errorReactor)

		_, err := back.ReadService("test")

		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

}

func TestKubeUpdateService(t *testing.T) {
}

func TestKubeDeleteService(t *testing.T) {
}

func TestKubeGetKubeClientset(t *testing.T) {
}
