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
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	errKueueDisabled    = errors.New("kueue is not enabled")
	errKueueUnavailable = errors.New("kueue API is not available")
)

// MakeGetOwnQuotaHandler handles GET /system/quotas/user for the bearer user.
// @Summary Get own quotas
// @Description Return CPU, memory, and volume quotas and current usage for the authenticated user. CPU values are in millicores, memory values in bytes, volume values use Kubernetes quantities.
// @Tags quotas
// @Produce json
// @Success 200 {object} types.QuotaResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 503 {string} string "Service Unavailable"
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
		if err := ensureQuotasEnabled(cfg); err != nil {
			writeQuotaError(c, err)
			return
		}
		resp, err := fetchQuota(c.Request.Context(), cfg, qb, uid)
		if err != nil {
			writeQuotaError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

// MakeGetUserQuotaHandler handles GET /system/quotas/user/{userId} for admin (basic auth).
// @Summary Get user quotas
// @Description GET returns CPU, memory, and volume quotas and usage for the specified user (admin only). CPU values are in millicores, memory values in bytes, volume values use Kubernetes quantities.
// @Tags quotas
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} types.QuotaResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 503 {string} string "Service Unavailable"
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
		if err := ensureQuotasEnabled(cfg); err != nil {
			writeQuotaError(c, err)
			return
		}

		resp, err := fetchQuota(c.Request.Context(), cfg, qb, user)
		if err != nil {
			writeQuotaError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

// MakeUpdateUserQuotaHandler handles PUT /system/quotas/user/{userId} for admin (basic auth).
// @Summary Update user quotas
// @Description PUT updates CPU, memory, and volume quotas for the specified user (admin only). At least one of cpu, memory, or volumes must be provided. CPU values are in millicores, memory values in bytes, volume values use Kubernetes quantities.
// @Tags quotas
// @Accept json
// @Produce json
// @Param userId path string true "User ID"
// @Param quotas body types.QuotaUpdateRequest false "Quota update payload (at least one of cpu, memory, or volumes)"
// @Success 200 {object} types.QuotaResponse
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 503 {string} string "Service Unavailable"
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
		if req.CPU == "" && req.Memory == "" && !hasVolumeQuotaUpdate(req.Volumes) {
			c.String(http.StatusBadRequest, "cpu, memory or volumes must be provided")
			return
		}
		if err := ensureQuotasEnabled(cfg); err != nil {
			writeQuotaError(c, err)
			return
		}
		if err := updateQuota(c.Request.Context(), cfg, qb, user, req); err != nil {
			writeQuotaError(c, err)
			return
		}
		resp, err := fetchQuota(c.Request.Context(), cfg, qb, user)
		if err != nil {
			writeQuotaError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func ensureKueueQuotasEnabled(cfg *types.Config) error {
	if cfg == nil || !cfg.KueueEnable {
		return fmt.Errorf("%w: /system/quotas requires KUEUE_ENABLE=true", errKueueDisabled)
	}
	return nil
}

func ensureQuotasEnabled(cfg *types.Config) error {
	if cfg == nil || (!cfg.KueueEnable && !cfg.VolumeEnable) {
		return fmt.Errorf("%w: /system/quotas requires KUEUE_ENABLE=true or VOLUME_ENABLE=true", errKueueDisabled)
	}
	return nil
}

func writeQuotaError(c *gin.Context, err error) {
	if errors.Is(err, errKueueDisabled) || errors.Is(err, errKueueUnavailable) {
		c.String(http.StatusServiceUnavailable, err.Error())
		return
	}
	c.String(http.StatusInternalServerError, err.Error())
}

func fetchQuota(ctx context.Context, cfg *types.Config, qb types.QuotaBackend, user string) (*types.QuotaResponse, error) {
	resp := &types.QuotaResponse{
		UserID: user,
	}

	if cfg.KueueEnable {
		if qb.Kueueclient == nil {
			return nil, fmt.Errorf("%w: Kueue client is not initialized", errKueueUnavailable)
		}

		cqName := utils.BuildClusterQueueName(user)
		cq, err := qb.Kueueclient.KueueV1beta2().ClusterQueues().Get(ctx, cqName, metav1.GetOptions{})
		if err != nil {
			if isMissingKueueAPI(err) {
				return nil, fmt.Errorf("%w: install Kueue CRDs before using /system/quotas: %v", errKueueUnavailable, err)
			}
			return nil, fmt.Errorf("getting ClusterQueue %s: %w", cqName, err)
		}

		resp.ClusterQueue = cqName
		resp.Resources = map[string]types.QuotaValues{}

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
	}

	if cfg.VolumeEnable {
		if qb.KubeClientset == nil {
			return nil, fmt.Errorf("Kubernetes client is not initialized")
		}
		volumes, err := utils.GetVolumeQuotaInfo(auth.FormatUID(user), utils.BuildUserNamespace(cfg, user), cfg, qb.KubeClientset)
		if err != nil {
			return nil, fmt.Errorf("getting volume quotas for user %s: %w", user, err)
		}
		resp.Volumes = volumes
	}

	return resp, nil
}

func updateQuota(ctx context.Context, cfg *types.Config, qb types.QuotaBackend, user string, req types.QuotaUpdateRequest) error {
	if req.CPU != "" || req.Memory != "" {
		if err := ensureKueueQuotasEnabled(cfg); err != nil {
			return err
		}
		if err := updateKueueQuota(ctx, qb, user, req); err != nil {
			return err
		}
	}

	if hasVolumeQuotaUpdate(req.Volumes) {
		if !cfg.VolumeEnable {
			return fmt.Errorf("volume quotas require VOLUME_ENABLE=true")
		}
		if qb.KubeClientset == nil {
			return fmt.Errorf("Kubernetes client is not initialized")
		}
		if err := updateVolumeQuota(user, req.Volumes, cfg, qb); err != nil {
			return err
		}
	}

	return nil
}

func updateKueueQuota(ctx context.Context, qb types.QuotaBackend, user string, req types.QuotaUpdateRequest) error {
	if qb.Kueueclient == nil {
		return fmt.Errorf("%w: Kueue client is not initialized", errKueueUnavailable)
	}

	cqName := utils.BuildClusterQueueName(user)
	cq, err := qb.Kueueclient.KueueV1beta2().ClusterQueues().Get(ctx, cqName, metav1.GetOptions{})
	if err != nil {
		if isMissingKueueAPI(err) {
			return fmt.Errorf("%w: install Kueue CRDs before using /system/quotas: %v", errKueueUnavailable, err)
		}
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
	if isMissingKueueAPI(err) {
		return fmt.Errorf("%w: install Kueue CRDs before using /system/quotas: %v", errKueueUnavailable, err)
	}
	return err
}

func updateVolumeQuota(user string, update *types.VolumeQuotaUpdate, cfg *types.Config, qb types.QuotaBackend) error {
	quotaName := auth.FormatUID(user)
	namespace := utils.BuildUserNamespace(cfg, user)
	current, err := utils.GetVolumeLimitInfo(quotaName, namespace, cfg, qb.KubeClientset)
	if err != nil {
		utils.EnsureVolumeLimits(quotaName, namespace, qb.KubeClientset, cfg)
		current, err = utils.GetVolumeLimitInfo(quotaName, namespace, cfg, qb.KubeClientset)
		if err != nil {
			return fmt.Errorf("getting current volume quota: %w", err)
		}
	}

	var nonManagedVolumes int
	var nonManagedStorage resource.Quantity
	if update.Disk != "" || update.Volumes != "" {
		nonManagedVolumes, nonManagedStorage, err = utils.GetNonManagedVolumeUsage(namespace, qb.KubeClientset)
		if err != nil {
			return fmt.Errorf("getting non-managed volume usage: %w", err)
		}
	}

	merged := *current
	if update.Disk != "" {
		visibleDisk, err := resource.ParseQuantity(update.Disk)
		if err != nil {
			return fmt.Errorf("invalid volumes.disk quantity: %w", err)
		}
		visibleDisk.Add(nonManagedStorage)
		merged.DiskAvailable = visibleDisk.String()
	}
	if update.Volumes != "" {
		visibleVolumes, err := resource.ParseQuantity(update.Volumes)
		if err != nil {
			return fmt.Errorf("invalid volumes.volumes quantity: %w", err)
		}
		merged.MaxVolumes = fmt.Sprintf("%d", visibleVolumes.Value()+int64(nonManagedVolumes))
	}
	if update.MaxDiskperVolume != "" {
		merged.MaxDiskperVolume = update.MaxDiskperVolume
	}
	if update.MinDiskperVolume != "" {
		merged.MinDiskperVolume = update.MinDiskperVolume
	}

	if _, err := resource.ParseQuantity(merged.DiskAvailable); err != nil {
		return fmt.Errorf("invalid volumes.disk quantity: %w", err)
	}
	if _, err := resource.ParseQuantity(merged.MaxVolumes); err != nil {
		return fmt.Errorf("invalid volumes.volumes quantity: %w", err)
	}
	if _, err := resource.ParseQuantity(merged.MaxDiskperVolume); err != nil {
		return fmt.Errorf("invalid volumes.max_disk_per_volume quantity: %w", err)
	}
	if _, err := resource.ParseQuantity(merged.MinDiskperVolume); err != nil {
		return fmt.Errorf("invalid volumes.min_disk_per_volume quantity: %w", err)
	}

	return utils.UpdateVolumeLimits(merged, quotaName, namespace, qb.KubeClientset, cfg)
}

func hasVolumeQuotaUpdate(update *types.VolumeQuotaUpdate) bool {
	return update != nil && (update.Disk != "" ||
		update.Volumes != "" ||
		update.MaxDiskperVolume != "" ||
		update.MinDiskperVolume != "")
}

func isMissingKueueAPI(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "the server could not find the requested resource") ||
		strings.Contains(msg, "no matches for kind")
}
