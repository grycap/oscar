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
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
)

// MakeReplicasGetHandler godoc
// @Summary Get replicas for a service
// @Description Get replicas and topology for a service federation.
// @Tags replicas
// @Produce json
// @Param serviceName path string true "Service name"
// @Success 200 {object} types.ReplicasResponse
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/replicas/{serviceName} [get]
func MakeReplicasGetHandler(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		service, err := back.ReadService("", c.Param("serviceName"))
		if err != nil {
			if errors.IsNotFound(err) || errors.IsGone(err) {
				c.Status(http.StatusNotFound)
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			return
		}

		topology := "none"
		if service.Federation != nil && service.Federation.Topology != "" {
			topology = service.Federation.Topology
		}
		resp := types.ReplicasResponse{
			Topology: topology,
			Replicas: service.Replicas,
		}
		c.JSON(http.StatusOK, resp)
	}
}

// MakeReplicasPostHandler godoc
// @Summary Add replicas to a federation
// @Description Add replicas to a service federation and propagate to the topology.
// @Tags replicas
// @Accept json
// @Produce json
// @Param serviceName path string true "Service name"
// @Param payload body types.ReplicasRequest true "Replicas add payload"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/replicas/{serviceName} [post]
func MakeReplicasPostHandler(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		updated, err := updateReplicasFromRequest(c, back, func(service *types.Service, req *types.ReplicasRequest) {
			service.Replicas = append(service.Replicas, req.Replicas...)
			if service.Federation != nil {
				service.Federation.Members = append(service.Federation.Members, req.Replicas...)
			}
		})
		if err != nil {
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}

// MakeReplicasPutHandler godoc
// @Summary Update replicas in a federation
// @Description Update replicas for a service federation and propagate to the topology.
// @Tags replicas
// @Accept json
// @Produce json
// @Param serviceName path string true "Service name"
// @Param payload body types.ReplicasRequest true "Replicas update payload"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/replicas/{serviceName} [put]
func MakeReplicasPutHandler(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		updated, err := updateReplicasFromRequest(c, back, func(service *types.Service, req *types.ReplicasRequest) {
			for _, target := range req.Replicas {
				for i, replica := range service.Replicas {
					if sameReplica(replica, target) && len(req.Update) > 0 {
						service.Replicas[i] = req.Update[0]
					}
				}
			}
			if service.Federation != nil {
				for _, target := range req.Replicas {
					for i, replica := range service.Federation.Members {
						if sameReplica(replica, target) && len(req.Update) > 0 {
							service.Federation.Members[i] = req.Update[0]
						}
					}
				}
			}
		})
		if err != nil {
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}

// MakeReplicasDeleteHandler godoc
// @Summary Delete replicas from a federation
// @Description Remove replicas from a service federation and propagate to the topology.
// @Tags replicas
// @Accept json
// @Produce json
// @Param serviceName path string true "Service name"
// @Param payload body types.ReplicasRequest true "Replicas delete payload"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/replicas/{serviceName} [delete]
func MakeReplicasDeleteHandler(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		updated, err := updateReplicasFromRequest(c, back, func(service *types.Service, req *types.ReplicasRequest) {
			service.Replicas = filterReplicas(service.Replicas, req.Replicas)
			if service.Federation != nil {
				service.Federation.Members = filterReplicas(service.Federation.Members, req.Replicas)
			}
		})
		if err != nil {
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}

func updateReplicasFromRequest(c *gin.Context, back types.ServerlessBackend, mutator func(service *types.Service, req *types.ReplicasRequest)) (*types.ReplicasResponse, error) {
	var req types.ReplicasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("Invalid payload: %v", err))
		return nil, err
	}

	service, err := back.ReadService("", c.Param("serviceName"))
	if err != nil {
		if errors.IsNotFound(err) || errors.IsGone(err) {
			c.Status(http.StatusNotFound)
		} else {
			c.String(http.StatusInternalServerError, err.Error())
		}
		return nil, err
	}

	if req.Clusters != nil {
		if service.Clusters == nil {
			service.Clusters = map[string]types.Cluster{}
		}
		for k, v := range req.Clusters {
			service.Clusters[k] = v
		}
	}
	if req.StorageProviders != nil {
		service.StorageProviders = req.StorageProviders
	}

	mutator(service, &req)

	if err := back.UpdateService(*service); err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error updating service: %v", err))
		return nil, err
	}

	if service.HasFederationMembers() {
		authHeader := c.GetHeader("Authorization")
		if errs := utils.ExpandFederation(service, authHeader, http.MethodPut); len(errs) > 0 {
			c.String(http.StatusOK, fmt.Sprintf("Updated with federation warnings: %v", errs))
			return nil, fmt.Errorf("federation propagation warnings")
		}
	}

	topology := "none"
	if service.Federation != nil && service.Federation.Topology != "" {
		topology = service.Federation.Topology
	}
	resp := &types.ReplicasResponse{
		Topology: topology,
		Replicas: service.Replicas,
	}
	return resp, nil
}

func filterReplicas(current types.ReplicaList, remove types.ReplicaList) types.ReplicaList {
	var filtered types.ReplicaList
	for _, replica := range current {
		if !containsReplica(remove, replica) {
			filtered = append(filtered, replica)
		}
	}
	return filtered
}

func containsReplica(list types.ReplicaList, target types.Replica) bool {
	for _, replica := range list {
		if sameReplica(replica, target) {
			return true
		}
	}
	return false
}

func sameReplica(a, b types.Replica) bool {
	return strings.EqualFold(a.Type, b.Type) &&
		a.ClusterID == b.ClusterID &&
		a.ServiceName == b.ServiceName
}
