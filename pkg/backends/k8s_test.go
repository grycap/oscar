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

	"github.com/grycap/oscar/v3/pkg/types"
	appsv1 "k8s.io/api/apps/v1"
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

	t.Run("valid list", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return a valid configMap
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("get", "configmaps", validConfigMapReaction)

		// Call
		_, err := back.ListServices()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("getServiceFromFDL throws error getting configMap", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

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

func TestKubeCreateServiceWithVolume(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	back := MakeKubeBackend(clientset, testConfig)
	service := types.Service{
		Name:  "with-volume",
		Owner: "owner",
		Volume: &types.ServiceVolumeConfig{
			Size:      "1Gi",
			MountPath: "/data",
		},
	}

	if err := back.CreateService(service); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := clientset.CoreV1().PersistentVolumeClaims(testConfig.ServicesNamespace).Get(t.Context(), "with-volume", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected created pvc, got error: %v", err)
	}
}

func TestKubeCreateServiceLegacyStorageDoesNotCreateManagedVolume(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	back := MakeKubeBackend(clientset, testConfig)
	service := types.Service{
		Name:   "legacy-storage",
		Owner:  "owner",
		Image:  "img",
		Script: "echo",
		Mount: types.StorageIOConfig{
			Provider: "minio",
			Path:     "legacy/mount",
		},
		Input: []types.StorageIOConfig{
			{Provider: "minio." + types.DefaultProvider, Path: "/input"},
		},
		Output: []types.StorageIOConfig{
			{Provider: "minio." + types.DefaultProvider, Path: "/output"},
		},
	}

	if err := back.CreateService(service); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := clientset.CoreV1().PersistentVolumeClaims(testConfig.ServicesNamespace).Get(t.Context(), service.Name, metav1.GetOptions{}); err == nil {
		t.Fatalf("did not expect managed volume pvc for legacy storage flow")
	}
}

func TestKubeDeleteServiceRespectsVolumeLifecycle(t *testing.T) {
	t.Run("retain keeps pvc", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		back := MakeKubeBackend(clientset, testConfig)
		service := types.Service{
			Name:      "retain-volume",
			Namespace: testConfig.ServicesNamespace,
			Owner:     "owner",
			Volume: &types.ServiceVolumeConfig{
				Name:            "retain-volume",
				Size:            "1Gi",
				MountPath:       "/data",
				LifecyclePolicy: types.VolumeLifecycleRetain,
			},
		}
		_, _ = clientset.CoreV1().PodTemplates(testConfig.ServicesNamespace).Create(t.Context(), &v1.PodTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: service.Name, Namespace: testConfig.ServicesNamespace},
		}, metav1.CreateOptions{})
		_, _ = clientset.CoreV1().PersistentVolumeClaims(testConfig.ServicesNamespace).Create(t.Context(), &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: service.GetVolumePVCName(), Namespace: testConfig.ServicesNamespace},
		}, metav1.CreateOptions{})

		if err := back.DeleteService(service); err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
		if _, err := clientset.CoreV1().PersistentVolumeClaims(testConfig.ServicesNamespace).Get(t.Context(), service.GetVolumePVCName(), metav1.GetOptions{}); err != nil {
			t.Fatalf("expected pvc to remain for retain policy, got error: %v", err)
		}
	})

	t.Run("delete removes pvc", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		back := MakeKubeBackend(clientset, testConfig)
		service := types.Service{
			Name:      "delete-volume",
			Namespace: testConfig.ServicesNamespace,
			Owner:     "owner",
			Volume: &types.ServiceVolumeConfig{
				Name:            "delete-volume",
				Size:            "1Gi",
				MountPath:       "/data",
				LifecyclePolicy: types.VolumeLifecycleDelete,
			},
		}
		_, _ = clientset.CoreV1().PodTemplates(testConfig.ServicesNamespace).Create(t.Context(), &v1.PodTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: service.Name, Namespace: testConfig.ServicesNamespace},
		}, metav1.CreateOptions{})
		_, _ = clientset.CoreV1().PersistentVolumeClaims(testConfig.ServicesNamespace).Create(t.Context(), &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: service.GetVolumePVCName(), Namespace: testConfig.ServicesNamespace},
		}, metav1.CreateOptions{})

		if err := back.DeleteService(service); err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
		if _, err := clientset.CoreV1().PersistentVolumeClaims(testConfig.ServicesNamespace).Get(t.Context(), service.GetVolumePVCName(), metav1.GetOptions{}); err == nil {
			t.Fatalf("expected pvc to be deleted for delete policy")
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
		_, err := back.ReadService(testConfig.ServicesNamespace, "test")
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
		_, err := back.ReadService(testConfig.ServicesNamespace, "test")
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
		_, err := back.ReadService(testConfig.ServicesNamespace, "test")
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
	testService := types.Service{
		Name: "test",
	}
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
		err := back.DeleteService(testService)
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
		err := back.DeleteService(testService)
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
		err := back.DeleteService(testService)
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
		err := back.DeleteService(testService)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("error deleting exposed resources", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()

		back := MakeKubeBackend(clientset, testConfig)

		// Return no error deleting service resources before exposed resources.
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "podtemplates", validDeleteReaction)
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "configmaps", validDeleteReaction)
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete-collection", "jobs", validDeleteReaction)

		// Return error deleting HPA inside exposed resources cleanup.
		back.kubeClientset.(*fake.Clientset).Fake.PrependReactor("delete", "horizontalpodautoscalers", errorReaction)

		exposedService := testService
		exposedService.Expose.APIPort = 8080

		// Call
		err := back.DeleteService(exposedService)
		if err == nil {
			t.Error("expecting error, got: nil")
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

func TestGetExposedServiceDeployment(t *testing.T) {
	client := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc-dlp",
			Namespace: "ns",
		},
	})

	deployment, err := GetExposedServiceDeployment(client, "ns", "svc")
	if err != nil {
		t.Fatalf("get exposed deployment: %v", err)
	}
	if deployment.Name != "svc-dlp" {
		t.Fatalf("expected deployment name svc-dlp, got %s", deployment.Name)
	}
}

func TestListServicePodsHelpers(t *testing.T) {
	client := fake.NewSimpleClientset(
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service-pod",
				Namespace: "ns",
				Labels: map[string]string{
					types.ServiceLabel: "svc",
				},
			},
		},
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "exposed-pod",
				Namespace: "ns",
				Labels: map[string]string{
					"app": "oscar-svc-exp-svc",
				},
			},
		},
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "knative-pod",
				Namespace: "ns",
				Labels: map[string]string{
					"serving.knative.dev/service": "svc",
				},
			},
		},
	)

	servicePods, err := ListServicePods(client, "ns", "svc")
	if err != nil {
		t.Fatalf("list service pods: %v", err)
	}
	if len(servicePods.Items) != 1 || servicePods.Items[0].Name != "service-pod" {
		t.Fatalf("unexpected service pods: %#v", servicePods.Items)
	}

	exposedPods, err := ListExposedServicePods(client, "ns", "svc")
	if err != nil {
		t.Fatalf("list exposed pods: %v", err)
	}
	if len(exposedPods.Items) != 1 || exposedPods.Items[0].Name != "exposed-pod" {
		t.Fatalf("unexpected exposed pods: %#v", exposedPods.Items)
	}

	knativePods, err := ListKnativeServicePods(client, "ns", "svc")
	if err != nil {
		t.Fatalf("list knative service pods: %v", err)
	}
	if len(knativePods.Items) != 1 || knativePods.Items[0].Name != "knative-pod" {
		t.Fatalf("unexpected knative service pods: %#v", knativePods.Items)
	}
}
