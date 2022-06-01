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

import "github.com/grycap/oscar/v2/pkg/types"

// TODO: implement!!

// TODO: add apropriate variables to store cpu and memory usage
// KubeResourceManager
type KubeResourceManager struct {
}

func (krm *KubeResourceManager) UpdateResources() error {
	// Count only schedulable (working) nodes
	return nil
}

func (krm *KubeResourceManager) IsSchedulable(*types.Service) bool {
	return true
}
