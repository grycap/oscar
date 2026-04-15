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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	DeploymentStatePending     = "pending"
	DeploymentStateReady       = "ready"
	DeploymentStateDegraded    = "degraded"
	DeploymentStateFailed      = "failed"
	DeploymentStateUnavailable = "unavailable"

	DeploymentResourceKindExposedService = "exposed_service"
	DeploymentResourceKindKnativeService = "knative_service"
	DeploymentResourceKindUnavailable    = "unavailable"
)

type ServiceDeploymentStatus struct {
	ServiceName        string       `json:"service_name"`
	Namespace          string       `json:"namespace,omitempty"`
	State              string       `json:"state"`
	Reason             string       `json:"reason,omitempty"`
	LastTransitionTime *metav1.Time `json:"last_transition_time,omitempty"`
	ActiveInstances    int          `json:"active_instances"`
	AffectedInstances  int          `json:"affected_instances"`
	ResourceKind       string       `json:"resource_kind"`
}

type ServiceDeploymentSummary struct {
	State              string       `json:"state"`
	Reason             string       `json:"reason,omitempty"`
	LastTransitionTime *metav1.Time `json:"last_transition_time,omitempty"`
	ActiveInstances    int          `json:"active_instances"`
	AffectedInstances  int          `json:"affected_instances"`
	ResourceKind       string       `json:"resource_kind,omitempty"`
}

type DeploymentLogStream struct {
	ServiceName string               `json:"service_name"`
	Available   bool                 `json:"available"`
	Message     string               `json:"message,omitempty"`
	Entries     []DeploymentLogEntry `json:"entries"`
}

type DeploymentLogEntry struct {
	Timestamp string `json:"timestamp,omitempty"`
	Message   string `json:"message"`
}
