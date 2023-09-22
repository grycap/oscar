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

	testFDL = `name: testname
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

	errorReaction = func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errFake
	}

	validConfigMapReaction = func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
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

	validDeleteReaction = func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, nil
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

	t.Run("valid list", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return a valid PodTemplateList
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("list", "podtemplates", validPodTemplateListReactor)

		// Return a valid configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapReaction)

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
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("list", "podtemplates", errorReaction)

		// Call
		_, err := back.ListServices()
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("getServiceFromFDL throws error getting configMap", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return a valid PodTemplateList
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("list", "podtemplates", validPodTemplateListReactor)

		// Return an error getting the configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", errorReaction)

		// Call
		_, err := back.ListServices()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("getServiceFromFDL throws error unmarshaling FDL", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		validConfigMapWithInvalidFDLReactor := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			validCM := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testnamespace",
				},
				Data: map[string]string{
					types.ScriptFileName: "testscript",
					types.FDLFileName:    "asDF::Asdf:asd;",
				},
			}
			return true, validCM, nil
		}

		// Return a valid PodTemplateList
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("list", "podtemplates", validPodTemplateListReactor)

		// Return a valid configMap with invalid FDL
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapWithInvalidFDLReactor)

		// Call
		_, err := back.ListServices()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// Test temporarily disabled to be able to use the image cache feature

func TestKubeCreateService(t *testing.T) {
	testService := types.Service{
		Name: "test",
	}

	t.Run("valid", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Call
		err := back.CreateService(testService)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("error creating service's YAML", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error creating the configMap
		oldYAMLMarshal := types.YAMLMarshal
		types.YAMLMarshal = func(interface{}) ([]byte, error) {
			return nil, errFake
		}
		defer func() {
			types.YAMLMarshal = oldYAMLMarshal
		}()

		// Call
		err := back.CreateService(testService)
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error creating configMap", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error creating the configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("create", "configmaps", errorReaction)

		// Call
		err := back.CreateService(testService)
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error toPodSpec no error deleting configMap", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error creating the podSpec (invalid resources)
		invalidService := types.Service{
			Name: "test",
			CPU:  "dfasdf",
		}

		// Call
		err := back.CreateService(invalidService)
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error toPodSpec and error deleting configMap", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error creating the podSpec (invalid resources)
		invalidService := types.Service{
			Name: "test",
			CPU:  "dfasdf",
		}

		// Return error deleting the configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "configmaps", errorReaction)

		// Call
		err := back.CreateService(invalidService)
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error creating podTemplate no error deleting", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error creating the podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("create", "podtemplates", errorReaction)

		// Call
		err := back.CreateService(testService)
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error creating podTemplate and error deleting", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error creating the podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("create", "podtemplates", errorReaction)

		// Return error deleting the configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "configmaps", errorReaction)

		// Call
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

		// Call
		_, err := back.ReadService("test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid podTemplate", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error getting podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "podtemplates", errorReaction)

		// Call
		_, err := back.ReadService("test")
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error updating configMap", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return valid podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "podtemplates", validPodTemplateReactor)

		// Return error creating ConfigMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("create", "configmaps", errorReaction)

		// Call
		_, err := back.ReadService("test")
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

}

func TestKubeUpdateService(t *testing.T) {
	testService := types.Service{
		Name: "test",
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

	t.Run("valid", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return valid ConfigMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapReactor)

		// Return no errors updating ConfigMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("update", "configmaps", validConfigMapReactor)

		// Return no errors updating podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("update", "podtemplates", validPodTemplateReactor)

		// Call
		err := back.UpdateService(testService)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("error getting old configMap", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error getting the old configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", errorReaction)

		// Call
		err := back.UpdateService(testService)
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error creating FDL YAML", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return valid configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapReactor)

		// Return error creating the configMap YAML
		oldYAMLMarshal := types.YAMLMarshal
		types.YAMLMarshal = func(interface{}) ([]byte, error) {
			return nil, errFake
		}
		defer func() {
			types.YAMLMarshal = oldYAMLMarshal
		}()

		// Call
		err := back.UpdateService(testService)
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error updating configMap", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return valid old configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapReactor)

		// Return error updating the configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("update", "configmaps", errorReaction)

		// Call
		err := back.UpdateService(testService)
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error creating podSpec", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return valid ConfigMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapReactor)

		// Return no errors updating ConfigMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("update", "configmaps", validConfigMapReactor)

		// Return error creating the podSpec (invalid resources)
		invalidService := types.Service{
			Name: "test",
			CPU:  "dfasdf",
		}

		// Call
		err := back.UpdateService(invalidService)
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error creating podSpec and restoring old configMap", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

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

		// Return valid ConfigMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapReactor)

		// Return no errors updating ConfigMap the first time is called
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("update", "configmaps", customConfigMapReactor)

		// Return error creating the podSpec (invalid resources)
		invalidService := types.Service{
			Name: "test",
			CPU:  "dfasdf",
		}

		// Call
		err := back.UpdateService(invalidService)
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error updating podTemplate", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return valid ConfigMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapReactor)

		// Return no errors updating ConfigMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("update", "configmaps", validConfigMapReactor)

		// Return error updating podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("update", "podtemplates", errorReaction)

		// Call
		err := back.UpdateService(testService)
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error updating podTemplate and restoring old configMap", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

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

		// Return valid ConfigMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapReactor)

		// Return no errors updating ConfigMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("update", "configmaps", customConfigMapReactor)

		// Return error updating podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("update", "podtemplates", errorReaction)

		// Call
		err := back.UpdateService(testService)
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

}

func TestKubeDeleteService(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return no error deleting podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "podtemplates", validDeleteReaction)

		// Return no error deleting podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "configmaps", validDeleteReaction)

		// Return no error deleting jobs
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete-collection", "jobs", validDeleteReaction)

		// Call
		err := back.DeleteService("test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("error deleting podTemplate", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return error deleting podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "podtemplates", errorReaction)

		// Call
		err := back.DeleteService("test")
		if err == nil {
			t.Error("expecting error, got: nil")
		}
	})

	t.Run("error deleting configMap", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return no error deleting podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "podtemplates", validDeleteReaction)

		// Return error deleting podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "configmaps", errorReaction)

		// Call
		err := back.DeleteService("test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("error deleting jobs", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return no error deleting podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "podtemplates", validDeleteReaction)

		// Return no error deleting podTemplate
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "configmaps", validDeleteReaction)

		// Return no error deleting jobs
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete-collection", "jobs", errorReaction)

		// Call
		err := back.DeleteService("test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestKubeGetKubeClientset(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	back := MakeKubeBackend(clientset, testConfig)

	if clientset != back.GetKubeClientset() {
		t.Error("the clientset obtained is not the same")
	}
}
