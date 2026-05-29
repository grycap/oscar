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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestEnsureVolumeLimits(t *testing.T) {
	kubeClientset := fake.NewSimpleClientset()
	cfg := &types.Config{
		VolumeAvailable: "10Gi",
		VolumeMax:       "5",
		VolumeMaxDisk:   "5Gi",
		VolumeMinDisk:   "1Gi",
	}

	EnsureVolumeLimits("test-uid", "test-ns", kubeClientset, cfg)

	rq, err := kubeClientset.CoreV1().ResourceQuotas("test-ns").Get(context.TODO(), "test-uid", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected ResourceQuota to be created: %v", err)
	}
	if rq.Spec.Hard[corev1.ResourceRequestsStorage] != resource.MustParse("10Gi") {
		t.Errorf("expected requests.storage=10Gi, got %v", rq.Spec.Hard[corev1.ResourceRequestsStorage])
	}
	if rq.Spec.Hard[corev1.ResourcePersistentVolumeClaims] != resource.MustParse("5") {
		t.Errorf("expected persistentvolumeclaims=5, got %v", rq.Spec.Hard[corev1.ResourcePersistentVolumeClaims])
	}

	lr, err := kubeClientset.CoreV1().LimitRanges("test-ns").Get(context.TODO(), "test-uid", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected LimitRange to be created: %v", err)
	}
	if len(lr.Spec.Limits) == 0 {
		t.Fatal("expected LimitRange to have limits")
	}
	if lr.Spec.Limits[0].Max[corev1.ResourceStorage] != resource.MustParse("5Gi") {
		t.Errorf("expected max.storage=5Gi, got %v", lr.Spec.Limits[0].Max[corev1.ResourceStorage])
	}
	if lr.Spec.Limits[0].Min[corev1.ResourceStorage] != resource.MustParse("1Gi") {
		t.Errorf("expected min.storage=1Gi, got %v", lr.Spec.Limits[0].Min[corev1.ResourceStorage])
	}
}

func TestUpdateVolumeLimits(t *testing.T) {
	kubeClientset := fake.NewSimpleClientset()
	cfg := &types.Config{
		VolumeAvailable: "10Gi",
		VolumeMax:       "5",
		VolumeMaxDisk:   "5Gi",
		VolumeMinDisk:   "1Gi",
	}

	EnsureVolumeLimits("test-uid", "test-ns", kubeClientset, cfg)

	newLimits := types.VolumeLimits{
		DiskAvailable:    "20Gi",
		MaxVolumes:       "10",
		MaxDiskperVolume: "10Gi",
		MinDiskperVolume: "2Gi",
	}
	err := UpdateVolumeLimits(newLimits, "test-uid", "test-ns", kubeClientset, cfg)
	if err != nil {
		t.Fatalf("unexpected error updating volume limits: %v", err)
	}

	rq, err := kubeClientset.CoreV1().ResourceQuotas("test-ns").Get(context.TODO(), "test-uid", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected ResourceQuota to exist: %v", err)
	}
	if rq.Spec.Hard[corev1.ResourceRequestsStorage] != resource.MustParse("20Gi") {
		t.Errorf("expected requests.storage=20Gi, got %v", rq.Spec.Hard[corev1.ResourceRequestsStorage])
	}
	if rq.Spec.Hard[corev1.ResourcePersistentVolumeClaims] != resource.MustParse("10") {
		t.Errorf("expected persistentvolumeclaims=10, got %v", rq.Spec.Hard[corev1.ResourcePersistentVolumeClaims])
	}

	lr, err := kubeClientset.CoreV1().LimitRanges("test-ns").Get(context.TODO(), "test-uid", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected LimitRange to exist: %v", err)
	}
	if lr.Spec.Limits[0].Max[corev1.ResourceStorage] != resource.MustParse("10Gi") {
		t.Errorf("expected max.storage=10Gi, got %v", lr.Spec.Limits[0].Max[corev1.ResourceStorage])
	}
	if lr.Spec.Limits[0].Min[corev1.ResourceStorage] != resource.MustParse("2Gi") {
		t.Errorf("expected min.storage=2Gi, got %v", lr.Spec.Limits[0].Min[corev1.ResourceStorage])
	}
}

func TestGetVolumeLimitInfo(t *testing.T) {
	kubeClientset := fake.NewSimpleClientset()
	cfg := &types.Config{
		VolumeAvailable: "10Gi",
		VolumeMax:       "5",
		VolumeMaxDisk:   "5Gi",
		VolumeMinDisk:   "1Gi",
	}

	EnsureVolumeLimits("test-uid", "test-ns", kubeClientset, cfg)

	info, err := GetVolumeLimitInfo("test-uid", "test-ns", cfg, kubeClientset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.DiskAvailable != "10Gi" {
		t.Errorf("expected DiskAvailable=10Gi, got %s", info.DiskAvailable)
	}
	if info.MaxVolumes != "5" {
		t.Errorf("expected MaxVolumes=5, got %s", info.MaxVolumes)
	}
	if info.MaxDiskperVolume != "5Gi" {
		t.Errorf("expected MaxDiskperVolume=5Gi, got %s", info.MaxDiskperVolume)
	}
	if info.MinDiskperVolume != "1Gi" {
		t.Errorf("expected MinDiskperVolume=1Gi, got %s", info.MinDiskperVolume)
	}
}

func TestGetVolumeQuotaInfo(t *testing.T) {
	kubeClientset := fake.NewSimpleClientset()
	cfg := &types.Config{
		VolumeAvailable: "10Gi",
		VolumeMax:       "5",
		VolumeMaxDisk:   "5Gi",
		VolumeMinDisk:   "1Gi",
	}

	EnsureVolumeLimits("test-uid", "test-ns", kubeClientset, cfg)

	info, err := GetVolumeQuotaInfo("test-uid", "test-ns", cfg, kubeClientset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Disk.Max != "10Gi" {
		t.Errorf("expected Disk.Max=10Gi, got %s", info.Disk.Max)
	}
	if info.Disk.Used != "0" {
		t.Errorf("expected Disk.Used=0, got %s", info.Disk.Used)
	}
	if info.Volumes.Max != "5" {
		t.Errorf("expected Volumes.Max=5, got %s", info.Volumes.Max)
	}
	if info.Volumes.Used != "0" {
		t.Errorf("expected Volumes.Used=0, got %s", info.Volumes.Used)
	}
	if info.MaxDiskperVolume != "5Gi" {
		t.Errorf("expected MaxDiskperVolume=5Gi, got %s", info.MaxDiskperVolume)
	}
	if info.MinDiskperVolume != "1Gi" {
		t.Errorf("expected MinDiskperVolume=1Gi, got %s", info.MinDiskperVolume)
	}
}

func TestValidateManagedVolumeQuota(t *testing.T) {
	kubeClientset := fake.NewSimpleClientset()
	cfg := &types.Config{
		VolumeAvailable: "10Gi",
		VolumeMax:       "5",
		VolumeMaxDisk:   "5Gi",
		VolumeMinDisk:   "1Gi",
	}

	EnsureVolumeLimits("test-uid", "test-ns", kubeClientset, cfg)

	err := ValidateManagedVolumeQuota("test-uid", "test-ns", "2Gi", cfg, kubeClientset)
	if err != nil {
		t.Fatalf("expected no error for valid quota, got: %v", err)
	}
}

func TestGetNonManagedVolumeUsage(t *testing.T) {
	kubeClientset := fake.NewSimpleClientset()

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unmanaged-pvc",
			Namespace: "test-ns",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("3Gi"),
				},
			},
		},
	}
	_, err := kubeClientset.CoreV1().PersistentVolumeClaims("test-ns").Create(context.TODO(), pvc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create PVC: %v", err)
	}

	count, storage, err := GetNonManagedVolumeUsage("test-ns", kubeClientset)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
	if storage.Cmp(resource.MustParse("3Gi")) != 0 {
		t.Errorf("expected storage=3Gi, got %s", storage.String())
	}
}
