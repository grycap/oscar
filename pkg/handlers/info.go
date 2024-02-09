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

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/version"
	"k8s.io/client-go/kubernetes"
)

// MakeInfoHandler makes a handler to retrieve system info
func MakeInfoHandler(kubeClientset kubernetes.Interface, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		info := version.GetInfo(kubeClientset, back)

		c.JSON(http.StatusOK, info)
	}
}
