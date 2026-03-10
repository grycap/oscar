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

package types

import (
	"net/http"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// Mock implementation of ServerlessBackend for testing
type mockServerlessBackend struct {
	info       *ServerlessBackendInfo
	services   []*Service
	kubeClient kubernetes.Interface
	proxyFunc  func(req *http.Request)
}

func (m *mockServerlessBackend) GetInfo() *ServerlessBackendInfo {
	return m.info
}

func (m *mockServerlessBackend) ListServices(namespaces ...string) ([]*Service, error) {
	// Simple mock that returns all services regardless of namespace
	return m.services, nil
}

func (m *mockServerlessBackend) CreateService(service Service) error {
	m.services = append(m.services, &service)
	return nil
}

func (m *mockServerlessBackend) ReadService(namespace, name string) (*Service, error) {
	for _, service := range m.services {
		if service.Name == name {
			return service, nil
		}
	}
	return nil, nil
}

func (m *mockServerlessBackend) UpdateService(service Service) error {
	for i, existing := range m.services {
		if existing.Name == service.Name {
			m.services[i] = &service
			return nil
		}
	}
	return nil
}

func (m *mockServerlessBackend) DeleteService(service Service) error {
	for i, existing := range m.services {
		if existing.Name == service.Name {
			m.services = append(m.services[:i], m.services[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockServerlessBackend) GetKubeClientset() kubernetes.Interface {
	return m.kubeClient
}

func newMockServerlessBackend() *mockServerlessBackend {
	return &mockServerlessBackend{
		info: &ServerlessBackendInfo{
			Name:    "MockBackend",
			Version: "1.0.0",
		},
		services:   []*Service{},
		kubeClient: fake.NewSimpleClientset(),
	}
}

// Mock implementation of SyncBackend for testing
type mockSyncBackend struct {
	*mockServerlessBackend
}

func (m *mockSyncBackend) GetProxyDirector(serviceName string) func(req *http.Request) {
	return m.proxyFunc
}

func newMockSyncBackend() *mockSyncBackend {
	return &mockSyncBackend{
		mockServerlessBackend: newMockServerlessBackend(),
	}
}

func TestServerlessBackendInterface(t *testing.T) {
	backend := newMockServerlessBackend()

	// Test GetInfo
	info := backend.GetInfo()
	if info == nil {
		t.Error("Expected GetInfo to return non-nil info")
	}

	if info.Name != "MockBackend" {
		t.Errorf("Expected backend name 'MockBackend', got '%s'", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("Expected backend version '1.0.0', got '%s'", info.Version)
	}

	// Test GetKubeClientset
	clientset := backend.GetKubeClientset()
	if clientset == nil {
		t.Error("Expected GetKubeClientset to return non-nil clientset")
	}
}

func TestServerlessBackendServiceOperations(t *testing.T) {
	backend := newMockServerlessBackend()

	// Test CreateService
	service := Service{
		Name:  "test-service",
		Image: "nginx:latest",
	}

	err := backend.CreateService(service)
	if err != nil {
		t.Errorf("Expected CreateService to succeed, got error: %v", err)
	}

	// Test ListServices
	services, err := backend.ListServices()
	if err != nil {
		t.Errorf("Expected ListServices to succeed, got error: %v", err)
	}

	if len(services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(services))
	}

	if services[0].Name != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", services[0].Name)
	}

	// Test ReadService
	readService, err := backend.ReadService("", "test-service")
	if err != nil {
		t.Errorf("Expected ReadService to succeed, got error: %v", err)
	}

	if readService == nil {
		t.Error("Expected ReadService to return non-nil service")
	} else if readService.Name != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", readService.Name)
	}

	// Test UpdateService
	updatedService := Service{
		Name:  "test-service",
		Image: "nginx:1.21",
	}

	err = backend.UpdateService(updatedService)
	if err != nil {
		t.Errorf("Expected UpdateService to succeed, got error: %v", err)
	}

	// Verify update
	readUpdatedService, err := backend.ReadService("", "test-service")
	if err != nil {
		t.Errorf("Expected ReadService to succeed after update, got error: %v", err)
	}

	if readUpdatedService.Image != "nginx:1.21" {
		t.Errorf("Expected updated image 'nginx:1.21', got '%s'", readUpdatedService.Image)
	}

	// Test DeleteService
	err = backend.DeleteService(service)
	if err != nil {
		t.Errorf("Expected DeleteService to succeed, got error: %v", err)
	}

	// Verify deletion
	deletedService, err := backend.ReadService("", "test-service")
	if err != nil {
		t.Errorf("Expected ReadService to succeed after deletion, got error: %v", err)
	}

	if deletedService != nil {
		t.Error("Expected ReadService to return nil after deletion")
	}
}

func TestServerlessBackendMultipleServices(t *testing.T) {
	backend := newMockServerlessBackend()

	services := []Service{
		{Name: "service-1", Image: "nginx:latest"},
		{Name: "service-2", Image: "redis:latest"},
		{Name: "service-3", Image: "postgres:latest"},
	}

	// Create multiple services
	for _, service := range services {
		err := backend.CreateService(service)
		if err != nil {
			t.Errorf("Expected CreateService to succeed for %s, got error: %v", service.Name, err)
		}
	}

	// List all services
	allServices, err := backend.ListServices()
	if err != nil {
		t.Errorf("Expected ListServices to succeed, got error: %v", err)
	}

	if len(allServices) != len(services) {
		t.Errorf("Expected %d services, got %d", len(services), len(allServices))
	}

	// Verify all services exist
	serviceNames := make(map[string]bool)
	for _, service := range allServices {
		serviceNames[service.Name] = true
	}

	for _, expectedService := range services {
		if !serviceNames[expectedService.Name] {
			t.Errorf("Expected service %s to exist in list", expectedService.Name)
		}
	}
}

func TestServerlessBackendNamespaces(t *testing.T) {
	backend := newMockServerlessBackend()

	service := Service{
		Name:  "test-service",
		Image: "nginx:latest",
	}

	// Create service
	err := backend.CreateService(service)
	if err != nil {
		t.Errorf("Expected CreateService to succeed, got error: %v", err)
	}

	// Test ListServices with different namespace arguments
	tests := []struct {
		name       string
		namespaces []string
	}{
		{"no namespaces", []string{}},
		{"one namespace", []string{"test-ns"}},
		{"multiple namespaces", []string{"ns1", "ns2", "ns3"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, err := backend.ListServices(tt.namespaces...)
			if err != nil {
				t.Errorf("Expected ListServices to succeed with namespaces %v, got error: %v", tt.namespaces, err)
			}

			// Mock implementation returns all services regardless of namespace
			if len(services) != 1 {
				t.Errorf("Expected 1 service, got %d", len(services))
			}
		})
	}
}

func TestSyncBackendInterface(t *testing.T) {
	backend := newMockSyncBackend()

	// Test that SyncBackend inherits ServerlessBackend methods
	info := backend.GetInfo()
	if info == nil {
		t.Error("Expected GetInfo to return non-nil info")
	}

	// Test GetProxyDirector
	var called bool
	proxyFunc := func(req *http.Request) {
		called = true
	}

	backend.proxyFunc = proxyFunc
	director := backend.GetProxyDirector("test-service")

	if director == nil {
		t.Error("Expected GetProxyDirector to return non-nil director")
	}

	// Test the director function
	req := &http.Request{}
	director(req)

	if !called {
		t.Error("Expected director function to be called")
	}
}

func TestSyncBackendMultipleProxyDirectors(t *testing.T) {
	backend := newMockSyncBackend()

	// Create different proxy directors for different services
	calls := make(map[string]bool)

	proxyFunc1 := func(req *http.Request) {
		calls["service1"] = true
	}

	proxyFunc2 := func(req *http.Request) {
		calls["service2"] = true
	}

	// Test director for service1
	backend.proxyFunc = proxyFunc1
	director1 := backend.GetProxyDirector("service1")
	req1 := &http.Request{}
	director1(req1)

	// Test director for service2
	backend.proxyFunc = proxyFunc2
	director2 := backend.GetProxyDirector("service2")
	req2 := &http.Request{}
	director2(req2)

	if !calls["service1"] {
		t.Error("Expected proxy for service1 to be called")
	}

	if !calls["service2"] {
		t.Error("Expected proxy for service2 to be called")
	}
}

func TestServerlessBackendErrorHandling(t *testing.T) {
	backend := newMockServerlessBackend()

	// Test ReadService with non-existent service
	nonExistent, err := backend.ReadService("", "non-existent")
	if err != nil {
		t.Errorf("Expected ReadService to succeed for non-existent service, got error: %v", err)
	}

	if nonExistent != nil {
		t.Error("Expected ReadService to return nil for non-existent service")
	}

	// Test UpdateService with non-existent service
	service := Service{
		Name:  "non-existent",
		Image: "nginx:latest",
	}

	err = backend.UpdateService(service)
	if err != nil {
		t.Errorf("Expected UpdateService to succeed for non-existent service, got error: %v", err)
	}

	// Verify service was not added
	services, err := backend.ListServices()
	if err != nil {
		t.Errorf("Expected ListServices to succeed, got error: %v", err)
	}

	if len(services) != 0 {
		t.Errorf("Expected 0 services after updating non-existent service, got %d", len(services))
	}

	// Test DeleteService with non-existent service
	err = backend.DeleteService(service)
	if err != nil {
		t.Errorf("Expected DeleteService to succeed for non-existent service, got error: %v", err)
	}
}

func TestServerlessBackendEmptyOperations(t *testing.T) {
	backend := newMockServerlessBackend()

	// Test operations on empty backend
	services, err := backend.ListServices()
	if err != nil {
		t.Errorf("Expected ListServices to succeed on empty backend, got error: %v", err)
	}

	if len(services) != 0 {
		t.Errorf("Expected 0 services on empty backend, got %d", len(services))
	}

	// Test create then immediate list
	service := Service{
		Name:  "test-service",
		Image: "nginx:latest",
	}

	err = backend.CreateService(service)
	if err != nil {
		t.Errorf("Expected CreateService to succeed, got error: %v", err)
	}

	services, err = backend.ListServices()
	if err != nil {
		t.Errorf("Expected ListServices to succeed after create, got error: %v", err)
	}

	if len(services) != 1 {
		t.Errorf("Expected 1 service after create, got %d", len(services))
	}
}
