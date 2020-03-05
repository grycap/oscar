// Copyright (C) GRyCAP - I3M - UPV
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const containerName = "oscar-container"

// Service represents an OSCAR service following the SCAR Function Definition Language
type Service struct {
	// The name of the service
	Name string `json:"name"`

	// Memory limit for the service following the kubernetes format
	// https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-memory
	Memory string `json:"memory"`

	// CPU limit for the service following the kubernetes format
	// https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu
	CPU string `json:"cpu"`

	// Docker image for the service
	Image string `json:"image"`

	// StorageIOConfig slices with the input and ouput service configuration
	Input  []StorageIOConfig `json:"input"`
	Output []StorageIOConfig `json:"output"`

	// The user script to execute when the service is invoked
	Script string `json:"script"`

	// The user-defined environment variables assigned to the service
	Environment struct {
		Vars map[string]string `json:"Variables"`
	} `json:"environment"`

	// Configuration for the storage providers used by the service
	StorageProviders *StorageProviders `json:"storage_providers"`
}

// ToPodSpec returns a k8s podSpec from the Service
// TODO
func (service *Service) ToPodSpec() (*v1.PodSpec, error) {
	resources, err := createResources(service)
	if err != nil {
		return nil, err
	}

	podSpec := &v1.PodSpec{
		Containers: []v1.Container{
			v1.Container{
				Name:      containerName,
				Image:     service.Image,
				Env:       convertEnvVars(service.Environment.Vars),
				Resources: resources,
			},
		},
	}

	return podSpec, nil
}

func convertEnvVars(vars map[string]string) []v1.EnvVar {
	envVars := []v1.EnvVar{}
	for k, v := range vars {
		envVars = append(envVars, v1.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	return envVars
}

func createResources(service *Service) (v1.ResourceRequirements, error) {
	resources := v1.ResourceRequirements{
		Limits: v1.ResourceList{},
	}

	if len(service.CPU) > 0 {
		cpu, err := resource.ParseQuantity(service.CPU)
		if err != nil {
			return resources, err
		}
		resources.Limits[v1.ResourceCPU] = cpu
	}

	if len(service.Memory) > 0 {
		memory, err := resource.ParseQuantity(service.Memory)
		if err != nil {
			return resources, err
		}
		resources.Limits[v1.ResourceMemory] = memory
	}

	return resources, nil
}
