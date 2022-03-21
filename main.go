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

package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v2/pkg/backends"
	"github.com/grycap/oscar/v2/pkg/handlers"
	"github.com/grycap/oscar/v2/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	// Read configuration from the environment
	cfg, err := types.ReadConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Creates the k8s in-cluster config
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Create the k8s clientset
	kubeClientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		log.Fatal(err)
	}

	// Create the ServerlessBackend based on the configuration
	var back types.ServerlessBackend

	switch cfg.ServerlessBackend {
	case "openfaas":
		ofBack := backends.MakeOpenfaasBackend(kubeClientset, kubeConfig, cfg)
		back = ofBack
		// Start OpenFaaS Scaler as a goroutine
		if cfg.OpenfaasScalerEnable {
			go ofBack.StartScaler()
		}
	case "knative":
		back = backends.MakeKnativeBackend(kubeClientset, kubeConfig, cfg)
	default:
		back = backends.MakeKubeBackend(kubeClientset, cfg)
	}

	// Create the router
	r := gin.Default()

	// DEVEL - CORS allow all origins
	r.Use(cors.Default())

	// Define system group with basic auth middleware
	system := r.Group("/system", gin.BasicAuth(gin.Accounts{
		// Use the config's username and password for basic auth
		cfg.Username: cfg.Password,
	}))

	// Config path
	system.GET("/config", handlers.MakeConfigHandler(cfg))

	// CRUD Services
	system.POST("/services", handlers.MakeCreateHandler(cfg, back))
	system.GET("/services", handlers.MakeListHandler(back))
	system.GET("/services/:serviceName", handlers.MakeReadHandler(back))
	system.PUT("/services", handlers.MakeUpdateHandler(cfg, back))
	system.DELETE("/services/:serviceName", handlers.MakeDeleteHandler(cfg, back))

	// Logs paths
	system.GET("/logs/:serviceName", handlers.MakeJobsInfoHandler(kubeClientset, cfg.ServicesNamespace))
	system.DELETE("/logs/:serviceName", handlers.MakeDeleteJobsHandler(kubeClientset, cfg.ServicesNamespace))
	system.GET("/logs/:serviceName/:jobName", handlers.MakeGetLogsHandler(kubeClientset, cfg.ServicesNamespace))
	system.DELETE("/logs/:serviceName/:jobName", handlers.MakeDeleteJobHandler(kubeClientset, cfg.ServicesNamespace))

	// Job path for async invocations
	r.POST("/job/:serviceName", handlers.MakeJobHandler(cfg, kubeClientset, back))

	// Service path for sync invocations (only if ServerlessBackend is enabled)
	syncBack, ok := back.(types.SyncBackend)
	if cfg.ServerlessBackend != "" && ok {
		r.POST("/run/:serviceName", handlers.MakeRunHandler(cfg, syncBack))
	}

	// System info path
	system.GET("/info", handlers.MakeInfoHandler(kubeClientset, back))

	// Serve OSCAR User Interface
	r.Static("/ui", "./assets")
	// Redirect root to /ui
	r.GET("/", func(c *gin.Context) {
		c.Request.URL.Path = "/ui"
		r.HandleContext(c)
	})

	// Health path for k8s health checks
	r.GET("/health", handlers.HealthHandler)

	// Define and start HTTP server
	s := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.ServicePort),
		Handler:      r,
		WriteTimeout: cfg.WriteTimeout,
		ReadTimeout:  cfg.ReadTimeout,
	}

	log.Fatal(s.ListenAndServe())
}
