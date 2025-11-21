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
)

func TestMakeFakeBackend(t *testing.T) {
	back := MakeFakeBackend()

	for k, v := range back.errors {
		if len(v) > 0 {
			t.Errorf("invalid list of errors for %s. Expected: empty, got %v", k, v)
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
				back.AddError("ListServices", errFake)
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
				back.AddError("CreateService", errFake)
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
				back.AddError("ReadService", errFake)
			}

			_, err := back.ReadService("", "test")

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
				back.AddError("UpdateService", errFake)
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
	testService := types.Service{
		Name: "test",
	}
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
				back.AddError("DeleteService", errFake)
			}

			err := back.DeleteService(testService)

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

	back.AddError("ListServices", errFake)

	if len(back.errors["ListServices"]) <= 0 {
		t.Error("error setting error value")
	}
}
