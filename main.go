//go:generate swag init --parseDependency --parseInternal --generalInfo main.go --output pkg/apidocs

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

// @title OSCAR API
// @version v2.0.0
// @description Secure REST API to manage OSCAR services, storage and executions.
// @contact.name GRyCAP
// @contact.email products@grycap.upv.es
// @BasePath /
// @schemes https http
// @securityDefinitions.basic BasicAuth
// @securityDefinitions.apikey BearerAuth
// @description OIDC Bearer token (e.g. Authorization: Bearer <token>)
// @in header
// @name Authorization

package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/handlers"
	"github.com/grycap/oscar/v3/pkg/handlers/buckets"
	"github.com/grycap/oscar/v3/pkg/resourcemanager"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	versioned "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
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

	//Create the metrics clientset
	metricsClientset := versioned.NewForConfigOrDie(kubeConfig)

	// Check if the cluster has available GPUs
	cfg.CheckAvailableGPUs(kubeClientset)

	cfg.CheckAvailableInterLink(kubeClientset)

	// Create the ServerlessBackend
	back := backends.MakeServerlessBackend(kubeClientset, kubeConfig, cfg)

	// Start OpenFaaS Scaler
	/*if cfg.ServerlessBackend == "openfaas" && cfg.OpenfaasScalerEnable {
		ofBack := back.(*backends.OpenfaasBackend)
		go ofBack.StartScaler()
	}*/

	// Create the ResourceManager and start it if enabled
	resMan := resourcemanager.MakeResourceManager(cfg, kubeClientset)
	if resMan != nil {
		go resourcemanager.StartResourceManager(resMan, cfg.ResourceManagerInterval)
	}

	// Start the ReScheduler if enabled
	if cfg.ReSchedulerEnable {
		go resourcemanager.StartReScheduler(cfg, back, kubeClientset)
	}

	// Create the router
	r := gin.Default()

	// Swagger UI endpoint (disabled in production)
	// r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Define system group with basic auth middleware
	system := r.Group("/system", auth.GetAuthMiddleware(cfg, kubeClientset))

	// Config path
	system.GET("/config", handlers.MakeConfigHandler(cfg))

	// CRUD Services
	system.POST("/services", handlers.MakeCreateHandler(cfg, back))
	system.GET("/services", handlers.MakeListHandler(back))
	system.GET("/services/:serviceName", handlers.MakeReadHandler(back))
	system.PUT("/services", handlers.MakeUpdateHandler(cfg, back))
	system.DELETE("/services/:serviceName", handlers.MakeDeleteHandler(cfg, back))

	// CRUD Buckets
	system.POST("/buckets", buckets.MakeCreateHandler(cfg))
	system.GET("/buckets", buckets.MakeListHandler(cfg))
	system.GET("/buckets/:bucket", buckets.MakeGetHandler(cfg))
	system.PUT("/buckets", buckets.MakeUpdateHandler(cfg))
	system.DELETE("/buckets/:bucket", buckets.MakeDeleteHandler(cfg))
	system.POST("/buckets/:bucket/presign", buckets.MakePresignHandler(cfg))

	// Logs paths
	system.GET("/logs", handlers.MakeGetSystemLogsHandler(kubeClientset, cfg))
	system.GET("/logs/:serviceName", handlers.MakeJobsInfoHandler(back, kubeClientset, cfg))
	system.DELETE("/logs/:serviceName", handlers.MakeDeleteJobsHandler(back, kubeClientset, cfg))
	system.GET("/logs/:serviceName/:jobName", handlers.MakeGetLogsHandler(back, kubeClientset, cfg))
	system.DELETE("/logs/:serviceName/:jobName", handlers.MakeDeleteJobHandler(back, kubeClientset, cfg))

	// Status path for cluster status (Memory and CPU) checks
	system.GET("/status", handlers.MakeStatusHandler(cfg, kubeClientset, metricsClientset))

	// Quotas
	system.GET("/quotas/user", handlers.MakeGetOwnQuotaHandler(cfg, kubeConfig))
	system.GET("/quotas/user/:userId", handlers.MakeGetUserQuotaHandler(cfg, kubeConfig))
	system.PUT("/quotas/user/:userId", handlers.MakeUpdateUserQuotaHandler(cfg, kubeConfig))

	// Job path for async invocations
	r.POST("/job/:serviceName", auth.GetLoggerMiddleware(), handlers.MakeJobHandler(cfg, kubeClientset, back, resMan))

	// Service path for sync invocations (only if ServerlessBackend is enabled)
	syncBack, ok := back.(types.SyncBackend)
	if cfg.ServerlessBackend != "" && ok {
		r.POST("/run/:serviceName", auth.GetLoggerMiddleware(), handlers.MakeRunHandler(cfg, syncBack))
		r.POST("/inference/:serviceName/*path", auth.GetAuthMiddleware(cfg, kubeClientset), handlers.MakeInferenceHandler(cfg, syncBack))
		r.GET("/inference/:serviceName/*path", auth.GetAuthMiddleware(cfg, kubeClientset), handlers.MakeInferenceHandler(cfg, syncBack))
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
