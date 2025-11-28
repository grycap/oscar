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

package resourcemanager

import (
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/grycap/oscar/v3/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func TestMakeResourceManager(t *testing.T) {
	cfg := types.Config{}
	enableCfg := types.Config{ResourceManagerEnable: true}

	rmEnable := MakeResourceManager(&enableCfg, fake.NewSimpleClientset())
	rm := MakeResourceManager(&cfg, fake.NewSimpleClientset())

	if rm != nil {
		t.Errorf("expecting nil, got %d", rm)
	}
	if rmEnable == nil {
		t.Errorf("expecting resource manager instance, got %d", rmEnable)
	}
}

type stubResourceManager struct {
	calls int
}

func (s *stubResourceManager) UpdateResources() error {
	s.calls++
	if s.calls >= 1 {
		panic("stop")
	}
	return nil
}
func (s *stubResourceManager) IsSchedulable(v1.ResourceRequirements) bool { return true }

func TestStartResourceManager(t *testing.T) {
	rm := &stubResourceManager{}

	done := make(chan struct{})
	go func() {
		defer func() {
			recover()
			close(done)
		}()
		StartResourceManager(rm, 0)
	}()

	<-done
	if rm.calls == 0 {
		t.Fatalf("expected UpdateResources to be invoked at least once")
	}
}
