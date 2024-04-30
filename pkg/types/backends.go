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
	"net/http"

	"k8s.io/client-go/kubernetes"
)

// ServerlessBackend define an interface for OSCAR's backends
type ServerlessBackend interface {
	GetInfo() *ServerlessBackendInfo
	ListServices() ([]*Service, error)
	CreateService(service Service) error
	ReadService(name string) (*Service, error)
	UpdateService(service Service) error
	DeleteService(service Service) error
	GetKubeClientset() kubernetes.Interface
}

// SyncBackend define an interface for serverless backends that allow sync invocations
type SyncBackend interface {
	ServerlessBackend
	GetProxyDirector(serviceName string) func(req *http.Request)
}
