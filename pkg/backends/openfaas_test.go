package backends

import (
	"testing"
	"time"

	"github.com/grycap/oscar/v3/pkg/types"
	ofv1 "github.com/openfaas/faas-netes/pkg/apis/openfaas/v1"
	ofclientset "github.com/openfaas/faas-netes/pkg/client/clientset/versioned/fake"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

func TestMakeOpenfaasBackend(t *testing.T) {
	kubeClientset := fake.NewSimpleClientset()
	kubeConfig := &rest.Config{}
	cfg := &types.Config{
		ServicesNamespace: "default",
		OpenfaasNamespace: "openfaas",
		OpenfaasPort:      8080,
	}

	ofBackend := MakeOpenfaasBackend(kubeClientset, kubeConfig, cfg)

	if ofBackend.namespace != "default" {
		t.Errorf("Expected namespace to be 'default', got '%s'", ofBackend.namespace)
	}
	if ofBackend.gatewayEndpoint != "gateway.openfaas:8080" {
		t.Errorf("Expected gatewayEndpoint to be 'gateway.openfaas:8080', got '%s'", ofBackend.gatewayEndpoint)
	}
}

func TestGetInfo(t *testing.T) {
	kubeClientset := fake.NewSimpleClientset()
	cfg := &types.Config{
		ServicesNamespace: "default",
		OpenfaasNamespace: "openfaas",
		OpenfaasPort:      8080,
	}

	ofClientset := ofclientset.NewSimpleClientset()
	ofBackend := &OpenfaasBackend{
		kubeClientset: kubeClientset,
		ofClientset:   ofClientset,
		namespace:     cfg.ServicesNamespace,
		config:        cfg,
	}

	info := ofBackend.GetInfo()
	if info.Name != "OpenFaaS" {
		t.Errorf("Expected Name to be 'OpenFaaS', got '%s'", info.Name)
	}
}

func TestCreateService(t *testing.T) {
	ofClientset := ofclientset.NewSimpleClientset()
	cfg := &types.Config{
		ServicesNamespace: "default",
		OpenfaasNamespace: "openfaas",
		OpenfaasPort:      8080,
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: cfg.ServicesNamespace,
		},
	}
	kubeClientset := fake.NewSimpleClientset(deployment)

	ofBackend := &OpenfaasBackend{
		kubeClientset: kubeClientset,
		ofClientset:   ofClientset,
		namespace:     cfg.ServicesNamespace,
		config:        cfg,
	}

	service := types.Service{
		Name:  "test-service",
		Image: "test-image",
		Labels: map[string]string{
			"test": "label",
		},
	}

	// Create a fake watcher
	fakeWatcher := watch.NewFake()

	// Set up a reactor to intercept the Watch action and return the fake watcher
	kubeClientset.PrependWatchReactor("deployments", func(action k8stesting.Action) (handled bool, ret watch.Interface, err error) {
		return true, fakeWatcher, nil
	})

	// Run watcher in a goroutine
	go func() {
		// Simulate the creation of the deployment by triggering an event on the fake watcher
		time.Sleep(1 * time.Second) // Ensure the CreateService method is waiting on the watcher
		fakeWatcher.Add(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      service.Name,
				Namespace: cfg.ServicesNamespace,
			},
		})

		// Allow some time for the CreateService method to process the event
		time.Sleep(1 * time.Second)
	}()

	err := ofBackend.CreateService(service)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	actions := ofClientset.Actions()
	if len(actions) != 1 {
		t.Errorf("Expected 1 action, got %d", len(actions))
	}
	if actions[0].GetResource().Resource != "functions" || actions[0].GetVerb() != "create" {
		t.Errorf("Expected action to be 'create functions', got '%s %s'", actions[0].GetVerb(), actions[0].GetResource().Resource)
	}
}

func TestReadService(t *testing.T) {
	ofClientset := ofclientset.NewSimpleClientset()
	cfg := &types.Config{
		ServicesNamespace: "default",
		OpenfaasNamespace: "openfaas",
		OpenfaasPort:      8080,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: cfg.ServicesNamespace,
		},
		Data: map[string]string{
			types.FDLFileName:    `{"name": "test-service"}`,
			types.ScriptFileName: "script.sh",
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: cfg.ServicesNamespace,
		},
	}
	kubeClientset := fake.NewSimpleClientset(cm, deployment)

	ofBackend := &OpenfaasBackend{
		kubeClientset: kubeClientset,
		ofClientset:   ofClientset,
		namespace:     cfg.ServicesNamespace,
		config:        cfg,
	}

	service, err := ofBackend.ReadService("test-service")
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}
	if service.Name != "test-service" {
		t.Errorf("Expected service name to be 'test-service', got '%s'", service.Name)
	}
}

func TestDeleteService(t *testing.T) {

	kubeClientset := fake.NewSimpleClientset()
	offunction := &ofv1.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: ofv1.FunctionSpec{
			Image: "test-image",
		},
	}
	ofClientset := ofclientset.NewSimpleClientset(offunction)
	cfg := &types.Config{
		ServicesNamespace: "default",
		OpenfaasNamespace: "openfaas",
		OpenfaasPort:      8080,
	}

	ofBackend := &OpenfaasBackend{
		kubeClientset: kubeClientset,
		ofClientset:   ofClientset,
		namespace:     cfg.ServicesNamespace,
		config:        cfg,
	}

	service := types.Service{
		Name:  "test-service",
		Image: "test-image",
	}

	// Delete the service
	err := ofBackend.DeleteService(service)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	actions := ofClientset.Actions()
	if len(actions) != 1 {
		t.Errorf("Expected 1 action, got %d", len(actions))
	}
	if actions[0].GetResource().Resource != "functions" || actions[0].GetVerb() != "delete" {
		t.Errorf("Expected action to be 'delete functions', got '%s %s'", actions[0].GetVerb(), actions[0].GetResource().Resource)
	}
}

func TestUpdateService(t *testing.T) {
	ofClientset := ofclientset.NewSimpleClientset()
	cfg := &types.Config{
		ServicesNamespace: "default",
		OpenfaasNamespace: "openfaas",
		OpenfaasPort:      8080,
	}

	oldCm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: cfg.ServicesNamespace,
		},
		Data: map[string]string{
			types.FDLFileName:    `{"name": "test-service"}`,
			types.ScriptFileName: "script.sh",
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: cfg.ServicesNamespace,
		},
	}
	kubeClientset := fake.NewSimpleClientset(oldCm, deployment)

	ofBackend := &OpenfaasBackend{
		kubeClientset: kubeClientset,
		ofClientset:   ofClientset,
		namespace:     cfg.ServicesNamespace,
		config:        cfg,
	}

	service := types.Service{
		Name:  "test-service",
		Image: "test-image",
		Labels: map[string]string{
			"test": "label",
		},
	}

	err := ofBackend.UpdateService(service)
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}

	actions := kubeClientset.Actions()
	if len(actions) != 4 {
		t.Errorf("Expected 4 actions, got %d", len(actions))
	}
	if actions[0].GetResource().Resource != "configmaps" || actions[0].GetVerb() != "get" {
		t.Errorf("Expected action to be 'get configmaps', got '%s %s'", actions[0].GetVerb(), actions[0].GetResource().Resource)
	}
	if actions[1].GetResource().Resource != "configmaps" || actions[1].GetVerb() != "update" {
		t.Errorf("Expected action to be 'update configmaps', got '%s %s'", actions[2].GetVerb(), actions[2].GetResource().Resource)
	}
	if actions[2].GetResource().Resource != "deployments" || actions[2].GetVerb() != "get" {
		t.Errorf("Expected action to be 'get deployments', got '%s %s'", actions[2].GetVerb(), actions[2].GetResource().Resource)
	}
	if actions[3].GetResource().Resource != "deployments" || actions[3].GetVerb() != "update" {
		t.Errorf("Expected action to be 'update deployments', got '%s %s'", actions[3].GetVerb(), actions[3].GetResource().Resource)
	}
}

func TestListServices(t *testing.T) {
	cfg := &types.Config{
		ServicesNamespace: "default",
		OpenfaasNamespace: "openfaas",
		OpenfaasPort:      8080,
	}

	ofClientset := ofclientset.NewSimpleClientset()

	cml := &v1.ConfigMapList{
		Items: []v1.ConfigMap{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: cfg.ServicesNamespace,
					Labels: map[string]string{
						"oscar_service": "true",
					},
				},
				Data: map[string]string{
					types.FDLFileName:    `{"name": "test-service"}`,
					types.ScriptFileName: "script.sh",
				},
			},
		},
	}
	kubeClientset := fake.NewSimpleClientset(cml)

	ofBackend := &OpenfaasBackend{
		kubeClientset: kubeClientset,
		ofClientset:   ofClientset,
		namespace:     cfg.ServicesNamespace,
		config:        cfg,
	}

	services, err := ofBackend.ListServices()
	if err != nil {
		t.Errorf("Expected no error, got '%v'", err)
	}
	if len(services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(services))
	}
	if services[0].Name != "test-service" {
		t.Errorf("Expected service name to be 'test-service', got '%s'", services[0].Name)
	}
}
