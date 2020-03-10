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

package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/pkg/handlers"
)

func main() {
	r := gin.Default()

	// Define system group with basic auth middleware
	system := r.Group("/system", gin.BasicAuth(gin.Accounts{
		"admin": "admin",
	}))

	//system.POST("/service", handlers.MakeCreateHandler())

	// Test Basic auth...
	system.GET("/info", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "valid user"})
	})

	r.GET("/health", handlers.HealthHandler)

	r.Run(":8080")
}

// use something similar on main
// MakeServerlessBackend creates a new ServerlessBackend based on the configuration
// func MakeServerlessBackend(c *Config, kubeClientset *kubernetes.Clientset) ServerlessBackend {
// 	// TODO
// 	if c.EnableServerlessBackend {
// 		switch c.ServerlessBackend {
// 		case "openfaas":
// 			//return backends.MakeOpenfaasBackend()
// 		case "knative":
// 			//return backends.MakeKnativeBackend()
// 		}
// 	}

// 	// KubeBackend is the default ServerlessBackend
// 	return backends.MakeKubeBackend(kubeClientset)
// }
