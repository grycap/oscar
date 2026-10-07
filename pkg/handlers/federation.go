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
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
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
		delegation := "static"
		if service.Federation != nil && service.Federation.Topology != "" {
			topology = service.Federation.Topology
		}
		if service.Federation != nil && service.Federation.Delegation != "" {
			delegation = service.Federation.Delegation
		}
		var replicas types.ReplicaList
		if service.Federation != nil && len(service.Federation.Members) > 0 {
			replicas = service.Federation.Members
		}
		resp := types.FederationResponse{
			Topology:   topology,
			Delegation: delegation,
			Members:    replicas,
		}
		c.JSON(http.StatusOK, resp)
	}
}

// MakeFederationPostHandler godoc
// @Summary Add federation members to a service
// @Description Add federation members to a service, creating missing remote services and updating existing ones.
// @Tags federation
// @Accept json
// @Produce json
// @Param serviceName path string true "Service name"
// @Param payload body types.FederationRequest true "Federation members add payload"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Failure 502 {string} string "Federation propagation failed"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/federation/{serviceName} [post]
func MakeFederationPostHandler(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		updated, err := updateFederationFromRequest(c, back, true, func(service *types.Service, req *types.FederationRequest) {
			if service.Federation == nil {
				service.Federation = &types.Federation{
					GroupID:    service.Name,
					Topology:   "star",
					Delegation: "static",
				}
			} else if !service.HasFederationMembers() {
				if service.Federation.Topology == "" || service.Federation.Topology == "none" {
					service.Federation.Topology = "star"
				}
				if service.Federation.Delegation == "" {
					service.Federation.Delegation = "static"
				}
				if service.Federation.GroupID == "" {
					service.Federation.GroupID = service.Name
				}
			}
			for _, member := range req.Members {
				if !containsReplica(service.Federation.Members, member) {
					service.Federation.Members = append(service.Federation.Members, member)
				}
			}
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
// @Failure 502 {string} string "Federation propagation failed"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/federation/{serviceName} [put]
func MakeFederationPutHandler(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		updated, err := updateFederationFromRequest(c, back, false, func(service *types.Service, req *types.FederationRequest) {
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
// @Failure 502 {string} string "Federation propagation failed"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/federation/{serviceName} [delete]
func MakeFederationDeleteHandler(back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		updated, err := updateFederationFromRequest(c, back, false, func(service *types.Service, req *types.FederationRequest) {
			if service.Federation == nil {
				service.Federation = &types.Federation{}
			}
			//Possibility of deleting service in a federation
			if req.Delete {
				// We safely iterate through each member sent in the JSON
				for d := 0; d < len(req.Members); d++ {
					targetServiceName := req.Members[d].ServiceName

					if targetServiceName != "" {
						// We create a temporary struct of type types.Service
						serviceToDelete := types.Service{
							Name:      targetServiceName,
							Namespace: service.Namespace,
						}

						// We pass the complete object to DeleteService
						errDelete := back.DeleteService(serviceToDelete)
						if errDelete != nil {
							fmt.Printf("Error removing the federated service %s: %v\n", targetServiceName, errDelete)
						} else {
							fmt.Printf(" Federated service removed: %s - %v\n", targetServiceName, errDelete)
						}
					}
				}
			}
			service.Federation.Members = filterReplicas(service.Federation.Members, req.Members)
		})
		if err != nil {
			return
		}
		c.JSON(http.StatusOK, updated)
	}
}

func updateFederationFromRequest(c *gin.Context, back types.ServerlessBackend, upsert bool, mutator func(service *types.Service, req *types.FederationRequest)) (*types.FederationResponse, error) {
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
	if isBearerRequest(c) {
		uid, uidErr := auth.GetUIDFromContext(c)
		if uidErr != nil {
			c.String(http.StatusUnauthorized, uidErr.Error())
			return nil, uidErr
		}
		if !isServiceOwnedByUser(service, uid) {
			c.Status(http.StatusForbidden)
			return nil, fmt.Errorf("user is not allowed to modify this service")
		}
	}

	if req.Clusters != nil {
		if service.Clusters == nil {
			service.Clusters = map[string]types.Cluster{}
		}
		for k, v := range req.Clusters {
			service.Clusters[k] = v
		}
	}
	if strings.TrimSpace(req.ClusterID) != "" {
		service.ClusterID = strings.TrimSpace(req.ClusterID)
	}
	if req.StorageProviders != nil {
		service.StorageProviders = req.StorageProviders
	}

	mutator(service, &req)
	if req.Topology != "" {
		topology := strings.ToLower(strings.TrimSpace(req.Topology))
		if topology != "none" && topology != "star" && topology != "mesh" {
			err := fmt.Errorf("federation topology must be none, star or mesh")
			c.String(http.StatusBadRequest, err.Error())
			return nil, err
		}
		if topology == "mesh" {
			if strings.TrimSpace(service.ClusterID) == "" {
				err := fmt.Errorf("service cluster_id is required for mesh federation")
				c.String(http.StatusBadRequest, err.Error())
				return nil, err
			}
			if _, ok := service.Clusters[service.ClusterID]; !ok {
				err := fmt.Errorf("coordinator cluster %q must be defined for mesh federation", service.ClusterID)
				c.String(http.StatusBadRequest, err.Error())
				return nil, err
			}
		}
		if service.Federation == nil {
			service.Federation = &types.Federation{Delegation: "static"}
		}
		if service.Federation.GroupID == "" {
			service.Federation.GroupID = service.Name
		}
		service.Federation.Topology = topology
	}
	if req.Delegation != "" {
		delegation := strings.ToLower(strings.TrimSpace(req.Delegation))
		if delegation != "static" && delegation != "random" && delegation != "load-based" {
			err := fmt.Errorf("federation delegation must be static, random or load-based")
			c.String(http.StatusBadRequest, err.Error())
			return nil, err
		}
		if service.Federation == nil {
			service.Federation = &types.Federation{Topology: "none"}
		}
		if service.Federation.Topology == "" {
			service.Federation.Topology = "none"
		}
		if service.Federation.GroupID == "" {
			service.Federation.GroupID = service.Name
		}
		service.Federation.Delegation = delegation
	}

	refreshToken := strings.TrimSpace(req.RefreshToken)
	if service.HasFederationMembers() {
		if service.Namespace == "" {
			c.String(http.StatusInternalServerError, "error storing federation credentials: service namespace is empty")
			return nil, fmt.Errorf("service namespace is empty")
		}
		if refreshToken != "" {
			if err := upsertRefreshTokenSecret(service, service.Namespace, refreshToken, back.GetKubeClientset()); err != nil {
				c.String(http.StatusInternalServerError, "error storing refresh-token secret: %v", err)
				return nil, err
			}
		} else if service.HasActiveFederationMembers() {
			var err error
			refreshToken, err = readRefreshTokenSecretValue(service.Name, service.Namespace, back.GetKubeClientset())
			if err != nil {
				c.String(http.StatusBadRequest, "refresh_token is required for active federation: %v", err)
				return nil, err
			}
		}
	}

	if err := back.UpdateService(*service); err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("Error updating service: %v", err))
		return nil, err
	}

	if service.HasFederationMembers() {
		authHeader := c.GetHeader("Authorization")
		var errs []error
		if upsert {
			errs = utils.UpsertFederation(service, authHeader, refreshToken)
		} else {
			errs = utils.ExpandFederation(service, authHeader, http.MethodPut, refreshToken)
		}
		if len(errs) > 0 {
			c.String(http.StatusBadGateway, fmt.Sprintf("federation propagation failed: %v", errs))
			return nil, fmt.Errorf("federation propagation failed")
		}
	}

	topology := "none"
	delegation := "static"
	if service.Federation != nil && service.Federation.Topology != "" {
		topology = service.Federation.Topology
	}
	if service.Federation != nil && service.Federation.Delegation != "" {
		delegation = service.Federation.Delegation
	}
	var replicas types.ReplicaList
	if service.Federation != nil && len(service.Federation.Members) > 0 {
		replicas = service.Federation.Members
	}
	resp := &types.FederationResponse{
		Topology:   topology,
		Delegation: delegation,
		Members:    replicas,
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
