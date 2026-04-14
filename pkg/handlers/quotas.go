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
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MakeGetOwnQuotaHandler handles GET /system/quotas/user for the bearer user.
// @Summary Get own quotas
// @Description Return CPU and memory quotas and current usage for the authenticated user. CPU values are in millicores, memory values in bytes.
// @Tags quotas
// @Produce json
// @Success 200 {object} quotaResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 500 {string} string "Internal Server Error"
// @Security BearerAuth
// @Router /system/quotas/user [get]
func MakeGetOwnQuotaHandler(qb types.QuotaBackend, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, err := auth.GetUIDFromContext(c)
		if err != nil {
			c.String(http.StatusUnauthorized, fmt.Sprintf("missing user identificator: %v", err))
			return
		}
		resp, err := fetchQuota(c.Request.Context(), cfg, qb, uid)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

// MakeGetUserQuotaHandler handles GET /system/quotas/user/{userId} for admin (basic auth).
// @Summary Get user quotas
// @Description GET returns CPU/memory quotas and usage for the specified user (admin only). CPU values are in millicores, memory values in bytes.
// @Tags quotas
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} quotaResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Router /system/quotas/user/{userId} [get]
func MakeGetUserQuotaHandler(qb types.QuotaBackend, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.Param("userId")
		authUser := c.GetString(gin.AuthUserKey)
		if authUser != cfg.Username {
			c.String(http.StatusForbidden, "forbidden")
			return
		}

		resp, err := fetchQuota(c.Request.Context(), cfg, qb, user)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

// MakeUpdateUserQuotaHandler handles PUT /system/quotas/user/{userId} for admin (basic auth).
// @Summary Update user quotas
// @Description PUT updates CPU/memory nominal quotas for the specified user (admin only). At least one of cpu or memory must be provided. CPU values are in millicores, memory values in bytes.
// @Tags quotas
// @Accept json
// @Produce json
// @Param userId path string true "User ID"
// @Param quotas body quotaUpdateRequest false "Quota update payload (at least one of cpu or memory)"
// @Success 200 {object} quotaResponse
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Router /system/quotas/user/{userId} [put]
func MakeUpdateUserQuotaHandler(qb types.QuotaBackend, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.Param("userId")
		authUser := c.GetString(gin.AuthUserKey)
		if authUser != cfg.Username {
			c.String(http.StatusForbidden, "forbidden")
			return
		}

		var req types.QuotaUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("invalid payload: %v", err))
			return
		}
		if req.CPU == "" && req.Memory == "" {
			c.String(http.StatusBadRequest, "cpu or memory must be provided")
			return
		}
		if err := updateQuota(c.Request.Context(), cfg, qb, user, req); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		resp, err := fetchQuota(c.Request.Context(), cfg, qb, user)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func fetchQuota(ctx context.Context, cfg *types.Config, qb types.QuotaBackend, user string) (*types.QuotaResponse, error) {

	cqName := utils.BuildClusterQueueName(user)
	cq, err := qb.Kueueclient.KueueV1beta2().ClusterQueues().Get(ctx, cqName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting ClusterQueue %s: %w", cqName, err)
	}

	resp := &types.QuotaResponse{
		UserID:       user,
		ClusterQueue: cqName,
		Resources:    map[string]types.QuotaValues{},
	}

	var maxCPU int64
	var maxMem int64
	if len(cq.Spec.ResourceGroups) > 0 && len(cq.Spec.ResourceGroups[0].Flavors) > 0 {
		for _, res := range cq.Spec.ResourceGroups[0].Flavors[0].Resources {
			switch res.Name {
			case corev1.ResourceCPU:
				maxCPU = res.NominalQuota.MilliValue()
			case corev1.ResourceMemory:
				maxMem = res.NominalQuota.Value()
			}
		}
	}

	var usedCPU int64
	var usedMem int64
	if len(cq.Status.FlavorsUsage) > 0 {
		for _, res := range cq.Status.FlavorsUsage[0].Resources {
			switch res.Name {
			case corev1.ResourceCPU:
				usedCPU = res.Total.MilliValue()
			case corev1.ResourceMemory:
				usedMem = res.Total.Value()
			}
		}
	}

	resp.Resources["cpu"] = types.QuotaValues{Max: maxCPU, Used: usedCPU}
	resp.Resources["memory"] = types.QuotaValues{Max: maxMem, Used: usedMem}

	return resp, nil
}

func updateQuota(ctx context.Context, cfg *types.Config, qb types.QuotaBackend, user string, req types.QuotaUpdateRequest) error {
	cqName := utils.BuildClusterQueueName(user)
	cq, err := qb.Kueueclient.KueueV1beta2().ClusterQueues().Get(ctx, cqName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting ClusterQueue %s: %w", cqName, err)
	}

	// Update quotas on the first flavor/resource group.
	if len(cq.Spec.ResourceGroups) == 0 || len(cq.Spec.ResourceGroups[0].Flavors) == 0 {
		return fmt.Errorf("ClusterQueue %s has no resource groups/flavors to update", cqName)
	}

	flavor := &cq.Spec.ResourceGroups[0].Flavors[0]
	for i, res := range flavor.Resources {
		switch res.Name {
		case corev1.ResourceCPU:
			if req.CPU != "" {
				q, err := resource.ParseQuantity(req.CPU)
				if err != nil {
					return fmt.Errorf("invalid cpu quantity: %w", err)
				}
				flavor.Resources[i].NominalQuota = q
			}
		case corev1.ResourceMemory:
			if req.Memory != "" {
				q, err := resource.ParseQuantity(req.Memory)
				if err != nil {
					return fmt.Errorf("invalid memory quantity: %w", err)
				}
				flavor.Resources[i].NominalQuota = q
			}
		}
	}

	_, err = qb.Kueueclient.KueueV1beta2().ClusterQueues().Update(ctx, cq, metav1.UpdateOptions{})
	return err
}
