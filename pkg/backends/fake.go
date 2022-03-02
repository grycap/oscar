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
	"errors"
	"fmt"
	"runtime"

	"github.com/grycap/oscar/v2/pkg/types"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
)

var errFake = errors.New("fake error")

// FakeBackend fake struct to mock the beahaviour of the ServerlessBackend interface
type FakeBackend struct {
	returnError map[string]bool
}

// NewFakeBackend returns the pointer of a new FakeBackend struct
func NewFakeBackend() types.ServerlessBackend {
	return &FakeBackend{
		returnError: map[string]bool{
			"GetInfo":          false,
			"ListServices":     false,
			"CreateService":    false,
			"ReadService":      false,
			"UpdateService":    false,
			"DeleteService":    false,
			"GetKubeClientset": false,
		},
	}
}

// GetInfo returns the ServerlessBackendInfo with the name and version (fake)
func (f *FakeBackend) GetInfo() *types.ServerlessBackendInfo {
	return &types.ServerlessBackendInfo{
		Name:    "fake-backend",
		Version: "devel",
	}
}

// ListServices returns a slice with all services registered in the provided namespace (fake)
func (f *FakeBackend) ListServices() ([]*types.Service, error) {
	if f.returnError[getCurrentFuncName()] {
		return nil, errFake
	}

	return []*types.Service{}, nil
}

// CreateService creates a new service as a k8s podTemplate (fake)
func (f *FakeBackend) CreateService(service types.Service) error {
	if f.returnError[getCurrentFuncName()] {
		return errFake
	}

	return nil
}

// ReadService returns a Service (fake)
func (f *FakeBackend) ReadService(name string) (*types.Service, error) {
	if f.returnError[getCurrentFuncName()] {
		return nil, errFake
	}

	return &types.Service{}, nil
}

// UpdateService updates an existent service (fake)
func (f *FakeBackend) UpdateService(service types.Service) error {
	if f.returnError[getCurrentFuncName()] {
		return errFake
	}

	return nil
}

// DeleteService deletes a service (fake)
func (f *FakeBackend) DeleteService(name string) error {
	if f.returnError[getCurrentFuncName()] {
		return errFake
	}

	return nil
}

// GetKubeClientset returns the Kubernetes Clientset (fake)
func (f *FakeBackend) GetKubeClientset() kubernetes.Interface {
	return testclient.NewSimpleClientset()
}

func getCurrentFuncName() string {
	pc, _, _, _ := runtime.Caller(1)
	return fmt.Sprintf("%s", runtime.FuncForPC(pc).Name())
}
