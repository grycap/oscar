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
)

// Custom logger
var reSchLogger = log.New(os.Stdout, "[RE-SCHEDULER] ", log.Flags())

// StartReScheduler starts the ReScheduler loop to check if there are pending pods exceeding the xx every cfg.ResourceManagerInterval
func StartReScheduler(rm ResourceManager, cfg *types.Config) {
	// TODO
	for {

		time.Sleep(time.Duration(cfg.ReSchedulerInterval) * time.Second)
	}
}
