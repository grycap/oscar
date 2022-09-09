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
	"net/http"
	"runtime"
	"strings"

	"github.com/grycap/oscar/v2/pkg/types"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
)

var errFake = errors.New("fake error")

// FakeBackend fake struct to mock the beahaviour of the ServerlessBackend interface
type FakeBackend struct {
	errors map[string][]error
}

// MakeFakeBackend returns the pointer of a new FakeBackend struct
func MakeFakeBackend() *FakeBackend {
	return &FakeBackend{
		errors: map[string][]error{
			"ListServices":  {},
			"CreateService": {},
			"ReadService":   {},
			"UpdateService": {},
			"DeleteService": {},
		},
	}
}

func MakeFakeSyncBackend() *FakeBackend {
	return &FakeBackend{
		errors: map[string][]error{
			"ListServices":     {},
			"CreateService":    {},
			"ReadService":      {},
			"UpdateService":    {},
			"DeleteService":    {},
			"GetProxyDirector": {},
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
	return []*types.Service{}, f.returnError(getCurrentFuncName())
}

// CreateService creates a new service as a k8s podTemplate (fake)
func (f *FakeBackend) CreateService(service types.Service) error {
	return f.returnError(getCurrentFuncName())
}

// ReadService returns a Service (fake)
func (f *FakeBackend) ReadService(name string) (*types.Service, error) {
	return &types.Service{Token: "AbCdEf123456"}, f.returnError(getCurrentFuncName())
}

// UpdateService updates an existent service (fake)
func (f *FakeBackend) UpdateService(service types.Service) error {
	return f.returnError(getCurrentFuncName())
}

// DeleteService deletes a service (fake)
func (f *FakeBackend) DeleteService(name string) error {
	return f.returnError(getCurrentFuncName())
}

// GetKubeClientset returns the Kubernetes Clientset (fake)
func (f *FakeBackend) GetKubeClientset() kubernetes.Interface {
	return testclient.NewSimpleClientset()
}

func (f *FakeBackend) GetProxyDirector(serviceName string) func(req *http.Request) {
	return func(req *http.Request) {
		host := "httpbin.org"
		req.Host = host

		req.URL.Scheme = "https"
		req.URL.Host = host
		req.URL.Path = "/status/200"
	}
}

func getCurrentFuncName() string {
	pc, _, _, _ := runtime.Caller(1)
	str := runtime.FuncForPC(pc).Name()
	slice := strings.Split(str, ".")
	return slice[len(slice)-1]
}

// AddError append an error to the specified function
func (f *FakeBackend) AddError(functionName string, err error) {
	f.errors[functionName] = append(f.errors[functionName], err)
}

func (f *FakeBackend) returnError(functionName string) error {
	if len(f.errors[functionName]) > 0 {
		err := f.errors[functionName][0]
		// Remove the returned error from the slice
		f.errors[functionName] = append(f.errors[functionName][:0], f.errors[functionName][1:]...)

		return err
	}
	return nil
}
