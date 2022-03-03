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
	"testing"

	"github.com/grycap/oscar/v2/pkg/types"
)

func TestMakeFakeBackend(t *testing.T) {
	back := MakeFakeBackend()

	for k, v := range back.returnError {
		if v != nil {
			t.Errorf("invalid returnError value for %s. Expected: false, got %v", k, v)
		}
	}
}

func TestFakeGetInfo(t *testing.T) {
	back := MakeFakeBackend()

	info := back.GetInfo()

	if info.Name != "fake-backend" || info.Version != "devel" {
		t.Error("invalid values")
	}
}

func TestFakeListServices(t *testing.T) {
	scenarios := []struct {
		name        string
		returnError bool
	}{
		{
			"test with no errors",
			false,
		},
		{
			"test with errors",
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			back := MakeFakeBackend()

			if s.returnError {
				back.ReturnError("ListServices", errors.New("fake error"))
			}

			_, err := back.ListServices()

			if s.returnError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expecting no errors, got %s", err.Error())
				}
			}
		})
	}
}

func TestFakeCreateService(t *testing.T) {
	scenarios := []struct {
		name        string
		returnError bool
	}{
		{
			"test with no errors",
			false,
		},
		{
			"test with errors",
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			back := MakeFakeBackend()

			if s.returnError {
				back.ReturnError("CreateService", errFake)
			}

			err := back.CreateService(types.Service{})

			if s.returnError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expecting no errors, got %s", err.Error())
				}
			}
		})
	}
}

func TestFakeReadService(t *testing.T) {
	scenarios := []struct {
		name        string
		returnError bool
	}{
		{
			"test with no errors",
			false,
		},
		{
			"test with errors",
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			back := MakeFakeBackend()

			if s.returnError {
				back.ReturnError("ReadService", errFake)
			}

			_, err := back.ReadService("test")

			if s.returnError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expecting no errors, got %s", err.Error())
				}
			}
		})
	}
}

func TestFakeUpdateService(t *testing.T) {
	scenarios := []struct {
		name        string
		returnError bool
	}{
		{
			"test with no errors",
			false,
		},
		{
			"test with errors",
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			back := MakeFakeBackend()

			if s.returnError {
				back.ReturnError("UpdateService", errFake)
			}

			err := back.UpdateService(types.Service{})

			if s.returnError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expecting no errors, got %s", err.Error())
				}
			}
		})
	}
}

func TestFakeDeleteService(t *testing.T) {
	scenarios := []struct {
		name        string
		returnError bool
	}{
		{
			"test with no errors",
			false,
		},
		{
			"test with errors",
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			back := MakeFakeBackend()

			if s.returnError {
				back.ReturnError("DeleteService", errFake)
			}

			err := back.DeleteService("test")

			if s.returnError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expecting no errors, got %s", err.Error())
				}
			}
		})
	}
}

func TestFakeGetKubeClientset(t *testing.T) {
	back := MakeFakeBackend()

	kubeClientset := back.GetKubeClientset()
	if kubeClientset == nil {
		t.Error("expecting kubernetes clientset interface, got nil")
	}
}

func TestGetCurrentFuncName(t *testing.T) {
	str := getCurrentFuncName()

	if str != "TestGetCurrentFuncName" {
		t.Errorf("expecting func name: TestGetCurrentFuncName, got: %s", str)
	}
}

func TestReturnError(t *testing.T) {
	back := MakeFakeBackend()

	back.ReturnError("ListServices", errFake)

	if back.returnError["ListServices"] == nil {
		t.Error("error setting returnError value")
	}
}

func TestReturnNoError(t *testing.T) {
	back := FakeBackend{
		returnError: map[string]error{
			"test": errFake,
		},
	}

	back.ReturnNoError("test")

	if back.returnError["test"] != nil {
		t.Error("error unsetting returnError value")
	}
}
