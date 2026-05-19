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

// MakeFederationGetHandler godoc
// @Summary Get federation members for a service
// @Description Get federation members and topology for a service.
// @Tags federation
// @Produce json
// @Param serviceName path string true "Service name"
// @Success 200 {object} types.FederationResponse
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/federation/{serviceName} [get]
func MakeFederationGetHandler(back types.ServerlessBackend) gin.HandlerFunc {
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
		var replicas types.ReplicaList
		if service.Federation != nil && len(service.Federation.Members) > 0 {
			replicas = service.Federation.Members
		}
		resp := types.FederationResponse{
			Topology: topology,
			Members:  replicas,
		}
		c.JSON(http.StatusOK, resp)
	}
}

// MakeFederationPostHandler godoc
// @Summary Add federation members to a service
// @Description Add federation members to a service and propagate to the topology.
// @Tags federation
// @Accept json
// @Produce json
// @Param serviceName path string true "Service name"
// @Param payload body types.FederationRequest true "Federation members add payload"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/federation/{serviceName} [post]
func MakeFederationPostHandler(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		updated, err := updateFederationFromRequest(c, back, func(service *types.Service, req *types.FederationRequest) {
			if service.Federation == nil {
				service.Federation = &types.Federation{}
			}
			service.Federation.Members = append(service.Federation.Members, req.Members...)
		})
		if err != nil {
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}

// MakeFederationPutHandler godoc
// @Summary Update federation members in a service
// @Description Update federation members for a service and propagate to the topology.
// @Tags federation
// @Accept json
// @Produce json
// @Param serviceName path string true "Service name"
// @Param payload body types.FederationRequest true "Federation members update payload"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/federation/{serviceName} [put]
func MakeFederationPutHandler(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		updated, err := updateFederationFromRequest(c, back, func(service *types.Service, req *types.FederationRequest) {
			if service.Federation == nil {
				service.Federation = &types.Federation{}
			}
			for _, target := range req.Members {
				for i, replica := range service.Federation.Members {
					if sameReplica(replica, target) && len(req.Update) > 0 {
						service.Federation.Members[i] = req.Update[0]
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

// MakeFederationDeleteHandler godoc
// @Summary Delete federation members from a service
// @Description Remove federation members from a service and propagate to the topology.
// @Tags federation
// @Accept json
// @Produce json
// @Param serviceName path string true "Service name"
// @Param payload body types.FederationRequest true "Federation members delete payload"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/federation/{serviceName} [delete]
func MakeFederationDeleteHandler(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		updated, err := updateFederationFromRequest(c, back, func(service *types.Service, req *types.FederationRequest) {
			if service.Federation == nil {
				service.Federation = &types.Federation{}
			}
			service.Federation.Members = filterReplicas(service.Federation.Members, req.Members)
		})
		if err != nil {
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}

func updateFederationFromRequest(c *gin.Context, back types.ServerlessBackend, mutator func(service *types.Service, req *types.FederationRequest)) (*types.FederationResponse, error) {
	var req types.FederationRequest
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
		if service.Namespace == "" {
			c.String(http.StatusInternalServerError, "error reading refresh-token secret: service namespace is empty")
			return nil, fmt.Errorf("service namespace is empty")
		}
		refreshToken, err := readRefreshTokenSecretValue(service.Name, service.Namespace, back.GetKubeClientset())
		if err != nil {
			c.String(http.StatusInternalServerError, "error reading refresh-token secret: %v", err)
			return nil, err
		}
		if errs := utils.ExpandFederation(service, authHeader, http.MethodPut, refreshToken); len(errs) > 0 {
			c.String(http.StatusOK, fmt.Sprintf("Updated with federation warnings: %v", errs))
			return nil, fmt.Errorf("federation propagation warnings")
		}
	}

	topology := "none"
	if service.Federation != nil && service.Federation.Topology != "" {
		topology = service.Federation.Topology
	}
	var replicas types.ReplicaList
	if service.Federation != nil && len(service.Federation.Members) > 0 {
		replicas = service.Federation.Members
	}
	resp := &types.FederationResponse{
		Topology: topology,
		Members:  replicas,
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
