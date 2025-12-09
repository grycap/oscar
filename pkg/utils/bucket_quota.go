package utils

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/grycap/oscar/v3/pkg/types"
)

// BucketQuotaSpec represents per-user overrides for bucket quotas.
type BucketQuotaSpec struct {
	BucketMaxPerUser     *int   `json:"bucket_max_per_user,omitempty"`
	BucketDefaultMaxSize string `json:"bucket_default_max_size,omitempty"`
}

const BucketQuotaConfigMapName = "oscar-bucket-quotas"

// GetEffectiveBucketQuota returns the effective limits for a user (overrides or defaults).
func GetEffectiveBucketQuota(ctx context.Context, cfg *types.Config, kubeConfig *rest.Config, user string) (int, string, error) {
	max := cfg.BucketMaxPerUser
	size := cfg.BucketDefaultMaxSize

	if kubeConfig == nil {
		return max, size, nil
	}

	spec, _, err := getBucketQuotaOverride(ctx, cfg, kubeConfig, user)
	if err != nil {
		return max, size, err
	}

	if spec.BucketMaxPerUser != nil {
		max = *spec.BucketMaxPerUser
	}
	if spec.BucketDefaultMaxSize != "" {
		size = spec.BucketDefaultMaxSize
	}
	return max, size, nil
}

// SetBucketQuotaOverride creates or updates the per-user override in the ConfigMap.
func SetBucketQuotaOverride(ctx context.Context, cfg *types.Config, kubeConfig *rest.Config, user string, spec BucketQuotaSpec) error {
	if kubeConfig == nil {
		return fmt.Errorf("kubeConfig is required to store bucket quotas")
	}
	client, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	cm, exists, err := getBucketQuotaConfigMap(ctx, cfg, client)
	if err != nil {
		return err
	}

	if cm.Data == nil {
		cm.Data = map[string]string{}
	}

	// Merge with existing override to preserve fields not being updated.
	if existingRaw, ok := cm.Data[user]; ok {
		existing := BucketQuotaSpec{}
		if err := json.Unmarshal([]byte(existingRaw), &existing); err == nil {
			if spec.BucketMaxPerUser == nil {
				spec.BucketMaxPerUser = existing.BucketMaxPerUser
			}
			if spec.BucketDefaultMaxSize == "" {
				spec.BucketDefaultMaxSize = existing.BucketDefaultMaxSize
			}
		}
	}

	payload, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshalling bucket quota override: %w", err)
	}
	cm.Data[user] = string(payload)

	if exists {
		_, err = client.CoreV1().ConfigMaps(cfg.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
		return err
	}
	_, err = client.CoreV1().ConfigMaps(cfg.Namespace).Create(ctx, cm, metav1.CreateOptions{})
	return err
}

func getBucketQuotaOverride(ctx context.Context, cfg *types.Config, kubeConfig *rest.Config, user string) (BucketQuotaSpec, bool, error) {
	res := BucketQuotaSpec{}
	client, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return res, false, err
	}

	cm, _, err := getBucketQuotaConfigMap(ctx, cfg, client)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return res, false, nil
		}
		return res, false, err
	}

	val, ok := cm.Data[user]
	if !ok {
		return res, false, nil
	}
	if err := json.Unmarshal([]byte(val), &res); err != nil {
		return res, false, fmt.Errorf("parsing bucket quota override: %w", err)
	}
	return res, true, nil
}

func getBucketQuotaConfigMap(ctx context.Context, cfg *types.Config, client *kubernetes.Clientset) (*corev1.ConfigMap, bool, error) {
	cm, err := client.CoreV1().ConfigMaps(cfg.Namespace).Get(ctx, BucketQuotaConfigMapName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: BucketQuotaConfigMapName, Namespace: cfg.Namespace}}, false, nil
		}
		return nil, false, err
	}
	return cm, true, nil
}
