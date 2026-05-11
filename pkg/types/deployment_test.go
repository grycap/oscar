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
	"encoding/json"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestServiceDeploymentStatusJSON(t *testing.T) {
	now := metav1.NewTime(time.Now().UTC())
	status := ServiceDeploymentStatus{
		ServiceName:        "svc",
		Namespace:          "ns",
		State:              DeploymentStateDegraded,
		Reason:             "1 of 2 instances is not ready",
		LastTransitionTime: &now,
		ActiveInstances:    2,
		AffectedInstances:  1,
		ResourceKind:       DeploymentResourceKindExposedService,
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}

	var decoded ServiceDeploymentStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}

	if decoded.ServiceName != status.ServiceName {
		t.Fatalf("expected service %q, got %q", status.ServiceName, decoded.ServiceName)
	}
	if decoded.State != status.State {
		t.Fatalf("expected state %q, got %q", status.State, decoded.State)
	}
	if decoded.AffectedInstances != 1 {
		t.Fatalf("expected affected instances 1, got %d", decoded.AffectedInstances)
	}
	if decoded.ResourceKind != DeploymentResourceKindExposedService {
		t.Fatalf("expected resource kind %q, got %q", DeploymentResourceKindExposedService, decoded.ResourceKind)
	}
}

func TestDeploymentLogStreamJSON(t *testing.T) {
	stream := DeploymentLogStream{
		ServiceName: "svc",
		Available:   true,
		Message:     "Returning recent deployment logs.",
		Entries: []DeploymentLogEntry{
			{
				Timestamp: "2026-04-04T12:00:00Z",
				Message:   "startup complete",
			},
		},
	}

	data, err := json.Marshal(stream)
	if err != nil {
		t.Fatalf("marshal stream: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	if raw["service_name"] != "svc" {
		t.Fatalf("expected service_name field, got %#v", raw["service_name"])
	}
	if raw["available"] != true {
		t.Fatalf("expected available=true, got %#v", raw["available"])
	}
}
