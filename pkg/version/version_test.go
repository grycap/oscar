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

package version

import (
	"testing"

	"github.com/grycap/oscar/v3/pkg/backends"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestGetInfo(t *testing.T) {
	fakeClientset := testclient.NewSimpleClientset()
	fakeBackend := backends.MakeFakeBackend()

	// No set version
	info := GetInfo(fakeClientset, fakeBackend)
	if info.Version != "devel" {
		t.Errorf("expecting version: devel, got: %s", info.Version)
	}

	// Set version and git commit
	Version = "test-version"
	GitCommit = "test-gitcommit"
	info = GetInfo(fakeClientset, fakeBackend)
	if info.Version != Version {
		t.Errorf("expecting version: %s, got: %s", Version, info.Version)
	}

	if info.GitCommit != GitCommit {
		t.Errorf("expecting git commit: %s, got: %s", GitCommit, info.GitCommit)
	}
}

func TestGetKubeVersion(t *testing.T) {
	// Valid
	fakeClientset := testclient.NewSimpleClientset()
	version := getKubeVersion(fakeClientset)
	if version == "" {
		t.Error("expected k8s version, got empty string")
	}

	// Invalid
	// Wait for https://github.com/kubernetes/kubernetes/pull/100564
	// reactor := func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
	// 	return true, nil, errors.New("test error")
	// }
	// fakeClientset.Fake.AddReactor("get", "version", reactor)
	// version = getKubeVersion(fakeClientset)
	// if version != "" {
	// 	t.Errorf("expected empty string version, got: %s", version)
	// }

}
