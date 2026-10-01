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

package utils

import (
	"reflect"
	"testing"
)

func TestServicesFromBucketTags(t *testing.T) {
	scenarios := []struct {
		name     string
		metadata map[string]string
		expected []string
	}{
		{"nil tags", nil, nil},
		{"empty tags", map[string]string{}, nil},
		{"missing tag", map[string]string{"owner": "alice"}, nil},
		{"empty value", map[string]string{"from_service": "  "}, nil},
		{"single service", map[string]string{"from_service": "cowsay"}, []string{"cowsay"}},
		{"multiple services", map[string]string{"from_service": "cowsay faas-vision"}, []string{"cowsay", "faas-vision"}},
		{"comma separated", map[string]string{"from_service": "cowsay,faas-vision"}, []string{"cowsay", "faas-vision"}},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			got := ServicesFromBucketTags(s.metadata)
			if !reflect.DeepEqual(got, s.expected) {
				t.Fatalf("expected %v, got %v", s.expected, got)
			}
		})
	}
}

func TestAddServiceToBucketTag(t *testing.T) {
	scenarios := []struct {
		name     string
		tags     map[string]string
		service  string
		expected string
	}{
		{"empty tags", map[string]string{}, "cowsay", "cowsay"},
		{"nil tags", nil, "cowsay", "cowsay"},
		{"add second service", map[string]string{"from_service": "cowsay"}, "faas-vision", "cowsay faas-vision"},
		{"deduplicate", map[string]string{"from_service": "cowsay"}, "cowsay", "cowsay"},
		{"preserve other tags", map[string]string{"owner": "alice", "from_service": "cowsay"}, "faas-vision", "cowsay faas-vision"},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			got := AddServiceToBucketTag(s.tags, s.service)
			if got[bucketServicesTag] != s.expected {
				t.Fatalf("expected from_service %q, got %q", s.expected, got[bucketServicesTag])
			}
		})
	}
}

func TestRemoveServiceFromBucketTag(t *testing.T) {
	scenarios := []struct {
		name            string
		tags            map[string]string
		service         string
		expectedService string
		expectedPresent bool
	}{
		{"remove last service", map[string]string{"from_service": "cowsay"}, "cowsay", "", false},
		{"remove one of many", map[string]string{"from_service": "cowsay faas-vision"}, "cowsay", "faas-vision", true},
		{"service not present", map[string]string{"from_service": "cowsay"}, "faas-vision", "cowsay", true},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			got := RemoveServiceFromBucketTag(s.tags, s.service)
			_, present := got[bucketServicesTag]
			if present != s.expectedPresent {
				t.Fatalf("expected from_service present=%v, got %v", s.expectedPresent, present)
			}
			if present && got[bucketServicesTag] != s.expectedService {
				t.Fatalf("expected from_service %q, got %q", s.expectedService, got[bucketServicesTag])
			}
		})
	}
}
