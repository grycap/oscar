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

package utils

import (
	"github.com/grycap/oscar/v2/pkg/types"
)

// Validate the input variables of the service
// Change the values if it is necesary
func ValidateService(serv types.Service) types.Service {
	serv = checkExposeInput(serv)
	return serv
}

// GO initialize all values to 0
// It initialize all the values if it gets teh default value.
// value 0 in port means, there is no service expose
func checkExposeInput(serv types.Service) types.Service {
	if serv.Expose.MaxScale == 0 {
		serv.Expose.MaxScale = 10
	}
	if serv.Expose.MinScale == 0 {
		serv.Expose.MinScale = 1
	}
	if serv.Expose.CpuThreshold == 0 {
		serv.Expose.CpuThreshold = 80
	}
	if serv.Expose.Port == 0 {
		serv.Expose.MaxScale = 0
		serv.Expose.MinScale = 0
		serv.Expose.CpuThreshold = 0
	}
	return serv
}
