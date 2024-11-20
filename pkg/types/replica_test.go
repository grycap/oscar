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
	"sort"
	"testing"
)

func TestReplicaList_Len(t *testing.T) {
	replicas := ReplicaList{
		{Type: "oscar", Priority: 1},
		{Type: "endpoint", Priority: 2},
	}
	expected := 2
	if replicas.Len() != expected {
		t.Errorf("expected %d, got %d", expected, len(replicas))
	}
}

func TestReplicaList_Swap(t *testing.T) {
	replicas := ReplicaList{
		{Type: "oscar", Priority: 1},
		{Type: "endpoint", Priority: 2},
	}
	replicas.Swap(0, 1)
	if replicas[0].Priority != 2 || replicas[1].Priority != 1 {
		t.Errorf("Swap did not work as expected")
	}
}

func TestReplicaList_Less(t *testing.T) {
	replicas := ReplicaList{
		{Type: "oscar", Priority: 1},
		{Type: "endpoint", Priority: 2},
	}
	if !replicas.Less(0, 1) {
		t.Errorf("expected replicas[0] to be less than replicas[1]")
	}
	if replicas.Less(1, 0) {
		t.Errorf("expected replicas[1] to not be less than replicas[0]")
	}
}

func TestReplicaList_Sort(t *testing.T) {
	replicas := ReplicaList{
		{Type: "endpoint", Priority: 2},
		{Type: "oscar", Priority: 1},
		{Type: "oscar", Priority: 0},
	}
	sort.Sort(replicas)
	if replicas[0].Priority != 0 || replicas[1].Priority != 1 || replicas[2].Priority != 2 {
		t.Errorf("Sort did not work as expected")
	}
}
