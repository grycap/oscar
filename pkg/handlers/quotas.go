package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	kueueclientset "sigs.k8s.io/kueue/client-go/clientset/versioned"
)

var (
	errKueueDisabled    = errors.New("kueue is not enabled")
	errKueueUnavailable = errors.New("kueue API is not available")
)

type quotaResponse struct {
	UserID       string                 `json:"user_id"`
	ClusterQueue string                 `json:"cluster_queue"`
	Resources    map[string]quotaValues `json:"resources"`
}

type quotaValues struct {
	Max  int64 `json:"max"`
	Used int64 `json:"used"`
}

type quotaUpdateRequest struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// MakeGetOwnQuotaHandler handles GET /system/quotas/user for the bearer user.
// @Summary Get own quotas
// @Description Return CPU and memory quotas and current usage for the authenticated user. CPU values are in millicores, memory values in bytes.
// @Tags quotas
// @Produce json
// @Success 200 {object} quotaResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 503 {string} string "Service Unavailable"
// @Failure 500 {string} string "Internal Server Error"
// @Security BearerAuth
// @Router /system/quotas/user [get]
func MakeGetOwnQuotaHandler(cfg *types.Config, kubeConfig *rest.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, err := auth.GetUIDFromContext(c)
		if err != nil {
			c.String(http.StatusUnauthorized, fmt.Sprintf("missing user identificator: %v", err))
			return
		}
		if err := ensureKueueQuotasEnabled(cfg); err != nil {
			writeQuotaError(c, err)
			return
		}
		resp, err := fetchQuota(c.Request.Context(), kubeConfig, uid)
		if err != nil {
			writeQuotaError(c, err)
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
// @Failure 503 {string} string "Service Unavailable"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Router /system/quotas/user/{userId} [get]
func MakeGetUserQuotaHandler(cfg *types.Config, kubeConfig *rest.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.Param("userId")
		authUser := c.GetString(gin.AuthUserKey)
		if authUser != cfg.Username {
			c.String(http.StatusForbidden, "forbidden")
			return
		}
		if err := ensureKueueQuotasEnabled(cfg); err != nil {
			writeQuotaError(c, err)
			return
		}

		resp, err := fetchQuota(c.Request.Context(), kubeConfig, user)
		if err != nil {
			writeQuotaError(c, err)
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
// @Failure 503 {string} string "Service Unavailable"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Router /system/quotas/user/{userId} [put]
func MakeUpdateUserQuotaHandler(cfg *types.Config, kubeConfig *rest.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.Param("userId")
		authUser := c.GetString(gin.AuthUserKey)
		if authUser != cfg.Username {
			c.String(http.StatusForbidden, "forbidden")
			return
		}

		var req quotaUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("invalid payload: %v", err))
			return
		}
		if req.CPU == "" && req.Memory == "" {
			c.String(http.StatusBadRequest, "cpu or memory must be provided")
			return
		}
		if err := ensureKueueQuotasEnabled(cfg); err != nil {
			writeQuotaError(c, err)
			return
		}
		if err := updateQuota(c.Request.Context(), kubeConfig, user, req); err != nil {
			writeQuotaError(c, err)
			return
		}
		resp, err := fetchQuota(c.Request.Context(), kubeConfig, user)
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

func writeQuotaError(c *gin.Context, err error) {
	if errors.Is(err, errKueueDisabled) || errors.Is(err, errKueueUnavailable) {
		c.String(http.StatusServiceUnavailable, err.Error())
		return
	}
	c.String(http.StatusInternalServerError, err.Error())
}

func fetchQuota(ctx context.Context, kubeConfig *rest.Config, user string) (*quotaResponse, error) {
	client, err := kueueclientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("creating kueue client: %w", err)
	}
	cqName := utils.BuildClusterQueueName(user)
	cq, err := client.KueueV1beta2().ClusterQueues().Get(ctx, cqName, metav1.GetOptions{})
	if err != nil {
		if isMissingKueueAPI(err) {
			return nil, fmt.Errorf("%w: install Kueue CRDs before using /system/quotas: %v", errKueueUnavailable, err)
		}
		return nil, fmt.Errorf("getting ClusterQueue %s: %w", cqName, err)
	}

	resp := &quotaResponse{
		UserID:       user,
		ClusterQueue: cqName,
		Resources:    map[string]quotaValues{},
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

	resp.Resources["cpu"] = quotaValues{Max: maxCPU, Used: usedCPU}
	resp.Resources["memory"] = quotaValues{Max: maxMem, Used: usedMem}
	return resp, nil
}

func updateQuota(ctx context.Context, kubeConfig *rest.Config, user string, req quotaUpdateRequest) error {
	client, err := kueueclientset.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("creating kueue client: %w", err)
	}
	cqName := utils.BuildClusterQueueName(user)
	cq, err := client.KueueV1beta2().ClusterQueues().Get(ctx, cqName, metav1.GetOptions{})
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

	_, err = client.KueueV1beta2().ClusterQueues().Update(ctx, cq, metav1.UpdateOptions{})
	if isMissingKueueAPI(err) {
		return fmt.Errorf("%w: install Kueue CRDs before using /system/quotas: %v", errKueueUnavailable, err)
	}
	return err
}

func isMissingKueueAPI(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "the server could not find the requested resource") ||
		strings.Contains(msg, "no matches for kind")
}
