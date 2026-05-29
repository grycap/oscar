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
	"testing"

	"github.com/grycap/oscar/v4/pkg/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateMinIOQuotaConfigMapIfDontExist_Disabled(t *testing.T) {
	cfg := &types.Config{
		MinIOQuotaEnabled: false,
	}
	cm, err := CreateMinIOQuotaConfigMapIfDontExist(context.Background(), cfg, nil, "oscar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cm != nil {
		t.Fatal("expected nil ConfigMap when MinIOQuotaEnabled is false")
	}
}

func TestCreateMinIOQuotaConfigMapIfDontExist_AlreadyExists(t *testing.T) {
	cfg := &types.Config{
		MinIOQuotaEnabled: true,
		MinIOQuotaBuckets: "5",
		MinIOQuotaStorage: "10Gi",
	}
	kubeClientset := fake.NewSimpleClientset()
	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MinIOQuotaConfigMapName,
			Namespace: "oscar",
		},
		Data: map[string]string{
			"existing": "data",
		},
	}
	kubeClientset.CoreV1().ConfigMaps("oscar").Create(context.Background(), existingCM, metav1.CreateOptions{})

	cm, err := CreateMinIOQuotaConfigMapIfDontExist(context.Background(), cfg, kubeClientset, "oscar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cm == nil {
		t.Fatal("expected non-nil ConfigMap")
	}
	if cm.Data["existing"] != "data" {
		t.Fatal("expected existing ConfigMap to be returned unchanged")
	}
}

func TestCreateMinIOQuotaConfigMapIfDontExist_Creates(t *testing.T) {
	cfg := &types.Config{
		MinIOQuotaEnabled: true,
		MinIOQuotaBuckets: "5",
		MinIOQuotaStorage: "10Gi",
	}
	kubeClientset := fake.NewSimpleClientset()

	cm, err := CreateMinIOQuotaConfigMapIfDontExist(context.Background(), cfg, kubeClientset, "oscar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cm == nil {
		t.Fatal("expected non-nil created ConfigMap")
	}
	if cm.Name != MinIOQuotaConfigMapName {
		t.Fatalf("expected ConfigMap name %q, got %q", MinIOQuotaConfigMapName, cm.Name)
	}
	if cm.Namespace != "oscar" {
		t.Fatalf("expected namespace 'oscar', got %q", cm.Namespace)
	}
	if cm.Labels[MinIOQuotaLabelKey] != MinIOQuotaLabelValue {
		t.Fatalf("expected label %s=%s", MinIOQuotaLabelKey, MinIOQuotaLabelValue)
	}
	if cm.Data[MinIOQuotaBucketsKey] != "5" {
		t.Fatalf("expected buckets key %q", "5")
	}
	if cm.Data[MinIOQuotaStorageKey] != "10Gi" {
		t.Fatalf("expected storage key %q", "10Gi")
	}
}
