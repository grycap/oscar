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
package utils

import (
	"context"
	"fmt"

	"github.com/grycap/oscar/v3/pkg/types"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	MinIOQuotaConfigMapName = "oscar-minio-quota"
	MinIOQuotaLabelKey      = "oscar.grycap.upv.es/quota"
	MinIOQuotaLabelValue    = "minio"
	MinIOQuotaBucketsKey    = "buckets"
	MinIOQuotaStorageKey    = "storage_per_bucket"
)

func GetDefaultMinIOQuotaConfigMapName() string {
	return MinIOQuotaConfigMapName
}

func CreateMinIOQuotaConfigMapIfDontExist(ctx context.Context, cfg *types.Config, kubeClientset kubernetes.Interface, namespace string) (*corev1.ConfigMap, error) {
	if !cfg.MinIOQuotaEnabled {
		return nil, nil
	}

	cm, err := kubeClientset.CoreV1().ConfigMaps(namespace).Get(ctx, MinIOQuotaConfigMapName, metav1.GetOptions{})
	if err == nil {
		return cm, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("getting MinIO quota ConfigMap %s/%s: %w", namespace, MinIOQuotaConfigMapName, err)
	}

	cm = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MinIOQuotaConfigMapName,
			Namespace: namespace,
			Labels: map[string]string{
				MinIOQuotaLabelKey: MinIOQuotaLabelValue,
			},
		},
		Data: map[string]string{
			MinIOQuotaBucketsKey: cfg.MinIOQuotaBuckets,
			MinIOQuotaStorageKey: cfg.MinIOQuotaStorage,
		},
	}
	cm, err = kubeClientset.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating MinIO quota ConfigMap %s/%s: %w", namespace, MinIOQuotaConfigMapName, err)
	}
	return cm, nil
}
