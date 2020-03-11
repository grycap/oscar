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
	"flag"
	"os"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/pkg/backends"
	"github.com/grycap/oscar/pkg/handlers"
	"github.com/grycap/oscar/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// Read configuration from the environment
	cfg, err := types.ReadConfig()
	if err != nil {
		panic(err.Error())
	}

	// Create the k8s clientset
	// Creates the k8s in-cluster config
	// kubeConfig, err := rest.InClusterConfig()
	// if err != nil {
	// 	panic(err.Error())
	// }

	// Read kubeconfig file in $HOME FOR TESTING
	homeDir, _ := os.UserHomeDir()
	kubeconfigPath := flag.String("kubeconfig", path.Join(homeDir, "kubeconfig"), "absolute path")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfigPath)
	if err != nil {
		panic(err.Error())
	}
	kubeClientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		panic(err.Error())
	}

	// Create the ServerlessBackend based on the configuration
	var back types.ServerlessBackend
	if cfg.EnableServerlessBackend {
		switch cfg.ServerlessBackend {
		// TODO: Uncomment when backends are implemented
		// case "openfaas":
		// 	back = backends.MakeOpenfaasBackend()
		// case "knative":
		// 	back = backends.MakeKnativeBackend()
		default:
			back = backends.MakeKubeBackend(kubeClientset, cfg)
		}
	} else {
		back = backends.MakeKubeBackend(kubeClientset, cfg)
	}

	// Create the router
	r := gin.Default()

	// Define system group with basic auth middleware
	system := r.Group("/system", gin.BasicAuth(gin.Accounts{
		// Use the config's username and password for basic auth
		cfg.Username: cfg.Password,
	}))

	// CRUD Services
	system.POST("/services", handlers.MakeCreateHandler(cfg, kubeClientset, back))
	//system.GET("/services", handlers.MakeListHandler(cfg, kubeClientset, back))
	//system.GET("/services/:serviceName", handlers.MakeReadHandler(cfg, kubeClientset, back))
	//system.PUT("/services", handlers.MakeUpdateHandler(cfg, kubeClientset, back))
	//system.DELETE("/services/:serviceName", handlers.MakeDeleteHandler(cfg, kubeClientset, back))

	// Job path for async invocations
	//r.POST("/job/:serviceName", handlers.MakeJobHandler(cfg, kubeClientset, back))

	// Service path for sync invocations (only if ServerlessBackend is enabled)
	// if cfg.EnableServerlessBackend {
	// 	r.GET("/service/:serviceName", handlers.MakeServiceHandler(cfg, kubeClientset, back))
	// }

	// System info path
	//system.GET("/info", handlers.MakeInfoHandler(...))

	// Health path for k8s health checks
	r.GET("/health", handlers.HealthHandler)

	r.Run(":8080")
}
