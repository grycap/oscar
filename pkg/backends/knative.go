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

// TODO

// TODO: add annotation "serving.knative.dev/visibility=cluster-local"
// to make all services only cluster-local, the Kn serving component can be configured to use the default domain "svc.cluster.local"
// https://knative.dev/docs/serving/cluster-local-route/

// KnativeBackend struct to represent a Knative client
type KnativeBackend struct{}
