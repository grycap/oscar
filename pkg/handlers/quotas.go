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
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	errKueueDisabled    = errors.New("kueue is not enabled")
	errKueueUnavailable = errors.New("kueue API is not available")
)

const (
	minIOQuotaConfigMapName = "oscar-minio-quota"
	minIOQuotaLabelKey      = "oscar.grycap.upv.es/quota"
	minIOQuotaLabelValue    = "minio"
	minIOQuotaBucketsKey    = "buckets"
	minIOQuotaStorageKey    = "storage_per_bucket"

	defaultMinIOBucketMax           int64  = 0
	defaultMinIOStoragePerBucketMax string = "0"
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
		if req.CPU == "" && req.Memory == "" && !hasVolumeQuotaUpdate(req.Volumes) && !hasMinIOQuotaUpdate(req.MinIO) {
			c.String(http.StatusBadRequest, "cpu, memory, volumes or minio must be provided")
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

	if qb.KubeClientset != nil {
		minioQuota, err := getMinIOQuotaInfo(ctx, cfg, qb.KubeClientset, user)
		if err != nil {
			return nil, fmt.Errorf("getting MinIO quotas for user %s: %w", user, err)
		}
		resp.MinIO = minioQuota
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

	if hasMinIOQuotaUpdate(req.MinIO) {
		if qb.KubeClientset == nil {
			return fmt.Errorf("Kubernetes client is not initialized")
		}
		if err := updateMinIOQuota(ctx, user, req.MinIO, cfg, qb.KubeClientset); err != nil {
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

func hasMinIOQuotaUpdate(update *types.MinIOQuotaUpdate) bool {
	return update != nil && (update.Buckets != "" || update.StoragePerBucket != "")
}

func getMinIOQuotaInfo(ctx context.Context, cfg *types.Config, kubeClientset kubernetes.Interface, user string) (*types.MinIOQuotaResponse, error) {
	quota, found, err := GetMinIOQuotaConfig(ctx, cfg, kubeClientset, user)
	if err != nil {
		return nil, err
	}
	resp := &types.MinIOQuotaResponse{
		Buckets: types.MinIOBucketCountQuota{
			Max: defaultMinIOBucketMax,
		},
		StoragePerBucket: types.MinIOStoragePerBucketQuota{
			Max: defaultMinIOStoragePerBucketMax,
		},
		StorageTotal: types.MinIOStorageTotalUsage{
			Used: "0",
		},
	}
	if cfg.MinIOProvider != nil {
		minIOAdminClient, err := utils.MakeMinIOAdminClient(cfg)
		if err != nil {
			return nil, fmt.Errorf("creating MinIO admin client: %w", err)
		}
		ownedBuckets, err := minIOAdminClient.ListBucketsByOwner(cfg.MinIOProvider.GetS3Client(), user)
		if err != nil {
			return nil, err
		}
		resp.Buckets.Used = int64(len(ownedBuckets))
		dataUsage, err := minIOAdminClient.GetDataUsageInfo()
		if err != nil {
			return nil, err
		}
		storageUsage, _, err := utils.AggregateBucketStorageUsage(dataUsage, ownedBuckets)
		if err != nil {
			return nil, err
		}
		resp.StorageTotal = types.MinIOStorageTotalUsage{
			Used: storageUsage.Used,
		}
	}
	if !found {
		return resp, nil
	}
	if quota.Buckets != "" {
		buckets, err := parseMinIOBucketLimit(quota.Buckets)
		if err != nil {
			return nil, err
		}
		resp.Buckets.Max = buckets
	}
	if quota.StoragePerBucket != "" {
		if _, err := utils.ParseStorageBytes(quota.StoragePerBucket); err != nil {
			return nil, fmt.Errorf("invalid minio.storage_per_bucket: %w", err)
		}
		resp.StoragePerBucket = types.MinIOStoragePerBucketQuota{
			Max: quota.StoragePerBucket,
		}
	}
	return resp, nil
}

func updateMinIOQuota(ctx context.Context, user string, update *types.MinIOQuotaUpdate, cfg *types.Config, kubeClientset kubernetes.Interface) error {
	if update.Buckets != "" {
		if _, err := parseMinIOBucketLimit(update.Buckets); err != nil {
			return err
		}
	}
	if update.StoragePerBucket != "" {
		if _, err := utils.ParseStorageBytes(update.StoragePerBucket); err != nil {
			return fmt.Errorf("invalid minio.storage_per_bucket: %w", err)
		}
	}
	current, found, err := GetMinIOQuotaConfig(ctx, cfg, kubeClientset, user)
	if err != nil {
		return err
	}
	if !found {
		current = &types.MinIOQuotaUpdate{}
	}
	if update.Buckets != "" {
		current.Buckets = update.Buckets
	}
	if update.StoragePerBucket != "" {
		current.StoragePerBucket = update.StoragePerBucket
	}
	if err := upsertMinIOQuotaConfig(ctx, cfg, kubeClientset, user, current); err != nil {
		return err
	}
	if update.StoragePerBucket != "" && cfg.MinIOProvider != nil {
		if err := applyMinIOStoragePerBucketQuota(user, update.StoragePerBucket, cfg); err != nil {
			return err
		}
	}
	return nil
}

func applyMinIOStoragePerBucketQuota(owner, storagePerBucket string, cfg *types.Config) error {
	minIOAdminClient, err := utils.MakeMinIOAdminClient(cfg)
	if err != nil {
		return fmt.Errorf("creating MinIO admin client: %w", err)
	}
	ownedBuckets, err := minIOAdminClient.ListBucketsByOwner(cfg.MinIOProvider.GetS3Client(), owner)
	if err != nil {
		return err
	}
	for bucketName := range ownedBuckets {
		if err := minIOAdminClient.SetBucketStorageQuota(bucketName, storagePerBucket); err != nil {
			return err
		}
	}
	return nil
}

func ValidateMinIOBucketCountQuota(cfg *types.Config, minIOAdminClient *utils.MinIOAdminClient, quota *types.MinIOQuotaUpdate, owner string, bucketNames []string) error {
	if quota == nil || quota.Buckets == "" {
		return nil
	}
	limit, err := parseMinIOBucketLimit(quota.Buckets)
	if err != nil {
		return err
	}
	ownedBuckets, err := minIOAdminClient.ListBucketsByOwner(cfg.MinIOProvider.GetS3Client(), owner)
	if err != nil {
		return err
	}
	newBuckets := map[string]struct{}{}
	for _, bucketName := range bucketNames {
		bucketName = strings.TrimSpace(bucketName)
		if bucketName == "" {
			continue
		}
		if _, alreadyOwned := ownedBuckets[bucketName]; alreadyOwned {
			continue
		}
		newBuckets[bucketName] = struct{}{}
	}
	if int64(len(ownedBuckets)+len(newBuckets)) > limit {
		return fmt.Errorf("MinIO bucket quota exceeded for user %s: limit %d, current %d, requested new buckets %d", owner, limit, len(ownedBuckets), len(newBuckets))
	}
	return nil
}

func GetMinIOQuotaConfig(ctx context.Context, cfg *types.Config, kubeClientset kubernetes.Interface, user string) (*types.MinIOQuotaUpdate, bool, error) {
	namespace := utils.BuildUserNamespace(cfg, user)
	cm, err := kubeClientset.CoreV1().ConfigMaps(namespace).Get(ctx, minIOQuotaConfigMapName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("getting MinIO quota ConfigMap %s/%s: %w", namespace, minIOQuotaConfigMapName, err)
	}
	quota := &types.MinIOQuotaUpdate{
		Buckets:          strings.TrimSpace(cm.Data[minIOQuotaBucketsKey]),
		StoragePerBucket: strings.TrimSpace(cm.Data[minIOQuotaStorageKey]),
	}
	return quota, true, nil
}

func upsertMinIOQuotaConfig(ctx context.Context, cfg *types.Config, kubeClientset kubernetes.Interface, user string, quota *types.MinIOQuotaUpdate) error {
	namespace := utils.BuildUserNamespace(cfg, user)
	data := map[string]string{}
	if quota.Buckets != "" {
		data[minIOQuotaBucketsKey] = quota.Buckets
	}
	if quota.StoragePerBucket != "" {
		data[minIOQuotaStorageKey] = quota.StoragePerBucket
	}
	cm, err := kubeClientset.CoreV1().ConfigMaps(namespace).Get(ctx, minIOQuotaConfigMapName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = kubeClientset.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      minIOQuotaConfigMapName,
				Namespace: namespace,
				Labels: map[string]string{
					minIOQuotaLabelKey: minIOQuotaLabelValue,
				},
			},
			Data: data,
		}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("creating MinIO quota ConfigMap %s/%s: %w", namespace, minIOQuotaConfigMapName, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting MinIO quota ConfigMap %s/%s: %w", namespace, minIOQuotaConfigMapName, err)
	}
	if cm.Labels == nil {
		cm.Labels = map[string]string{}
	}
	cm.Labels[minIOQuotaLabelKey] = minIOQuotaLabelValue
	cm.Data = data
	_, err = kubeClientset.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating MinIO quota ConfigMap %s/%s: %w", namespace, minIOQuotaConfigMapName, err)
	}
	return nil
}

func parseMinIOBucketLimit(value string) (int64, error) {
	limit, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid minio.buckets quantity: %w", err)
	}
	if limit < 0 {
		return 0, fmt.Errorf("invalid minio.buckets quantity: must be greater than or equal to zero")
	}
	return limit, nil
}

func isMissingKueueAPI(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "the server could not find the requested resource") ||
		strings.Contains(msg, "no matches for kind")
}
