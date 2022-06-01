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

	"github.com/grycap/oscar/v2/pkg/types"
)

// Custom logger
var resManLogger = log.New(os.Stdout, "[RESOURCE-MANAGER] ", log.Flags())

// ResourceManager interface to define cluster-level resource managers
type ResourceManager interface {
	UpdateResources() error
	IsSchedulable(*types.Service) bool
}

func startResourceManager(rm ResourceManager, cfg *types.Config) {
	// TODO
	for {
		if err := rm.UpdateResources(); err != nil {
			// TODO: check how to add prefix to logs!! e.g. [RESOURCE-MANAGER]
			resManLogger.Println(err.Error())
		}

		// TODO when the 'RESOURCE_MANAGER_INTERVAL' variable has been defined
		//time.Sleep()
	}
}
