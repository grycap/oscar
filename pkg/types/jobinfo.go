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

// JobInfo details the current status of a service's job
type JobInfo struct {
	Status       string       `json:"status"`
	CreationTime *metav1.Time `json:"creation_time,omitempty"`
	StartTime    *metav1.Time `json:"start_time,omitempty"`
	FinishTime   *metav1.Time `json:"finish_time,omitempty"`
}
