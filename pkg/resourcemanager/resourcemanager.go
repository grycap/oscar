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
	"log"
	"os"
	"time"

	"github.com/grycap/oscar/v2/pkg/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// Custom logger
var ResourceManagerLogger = log.New(os.Stdout, "[RESOURCE-MANAGER] ", log.Flags())

// ResourceManager interface to define cluster-level resource managers
type ResourceManager interface {
	UpdateResources() error
	IsSchedulable(v1.ResourceRequirements) bool
}

// MakeResourceManager returns a new ResourceManager if it is enabled in the config
// Apache's YuniKorn scheduler not supported yet
func MakeResourceManager(cfg *types.Config, kubeClientset kubernetes.Interface) ResourceManager {
	if cfg.ResourceManagerEnable {
		if !cfg.YunikornEnable {
			return &KubeResourceManager{
				kubeClientset: kubeClientset,
			}
		}
	}

	return nil
}

// StartResourceManager starts the ResourceManager loop to check cluster resources every cfg.ResourceManagerInterval
func StartResourceManager(rm ResourceManager, interval int) {
	for {
		if err := rm.UpdateResources(); err != nil {
			ResourceManagerLogger.Println(err.Error())
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}
}
