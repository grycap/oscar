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

// Service represents an OSCAR service following the SCAR Function Definition Language
type Service struct {
	Name        string            `json:"name"`
	Memory      int               `json:"memory"`
	CPU         int               `json:"cpu"`
	Image       string            `json:"image"`
	Input       []StorageIOConfig `json:"input"`
	Output      []StorageIOConfig `json:"output"`
	Script      string            `json:"script"`
	Environment struct {
		Vars map[string]string `json:"Variables"`
	} `json:"environment"`
	StorageProviders *StorageProviders `json:"storage_providers"`
}
