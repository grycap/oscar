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
	"log"
	"os"

	"github.com/grycap/oscar/v4/pkg/types"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	PVC = "PersistentVolumeClaim"
)

// Custom logger
var QuotasLogger = log.New(os.Stdout, "[QUOTAS] ", log.Flags())
var LimitsLogger = log.New(os.Stdout, "[LIMITS] ", log.Flags())

// Main functions
func EnsureVolumeLimits(name string, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) {
	_, errQuotas := getResouceQuotas(name, namespace, kubeClientset)
	quota := fromConftoVolumeLimits(cfg)

	if errQuotas != nil {
		QuotasLogger.Printf("Creating ResourceQuota %s", name)
		// #nosec
		createResources(name, namespace, kubeClientset, &quota)
	} else {
		QuotasLogger.Printf("ResourceQuota %s already created", name)
	}

	_, errLimits := getLimits(name, namespace, kubeClientset)
	if errLimits != nil {
		LimitsLogger.Printf("Creating Limit %s", name)
		// #nosec
		createLimits(name, namespace, kubeClientset, &quota)
	} else {
		LimitsLogger.Printf("Limit %s already created", name)
	}
}

func UpdateVolumeLimits(vl types.VolumeLimits, name string, namespace string, kubeClientset kubernetes.Interface, cfg *types.Config) error {
	EnsureVolumeLimits(name, namespace, kubeClientset, cfg)
	resource := getResourceDef(name, namespace, vl)
	QuotasLogger.Printf("Updating ResourceQuota %s", name)
	if _, err := kubeClientset.CoreV1().ResourceQuotas(namespace).Update(context.TODO(), resource, metav1.UpdateOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	limit := getLimitsDef(name, namespace, vl)
	LimitsLogger.Printf("Updating Limit %s", name)
	if _, err := kubeClientset.CoreV1().LimitRanges(namespace).Update(context.TODO(), limit, metav1.UpdateOptions{}); err != nil && !apierrors.IsNotFound(err) {
		fmt.Println(err)
		return err
	}
	return nil
}

func GetVolumeLimitInfo(uid string, namespace string, cfg *types.Config, kubeClientset kubernetes.Interface) (*types.VolumeLimits, error) {
	resources, err := getResouceQuotas(uid, namespace, kubeClientset)
	if err != nil {
		return nil, err
	}
	limits, err := getLimits(uid, namespace, kubeClientset)
	if err != nil {
		return nil, err
	}

	VolumeLimits := types.VolumeLimits{
		DiskAvailable:    getResourceQuantity(resources.Spec.Hard, v1.ResourceRequestsStorage),
		MaxVolumes:       getResourceQuantity(resources.Spec.Hard, v1.ResourcePersistentVolumeClaims),
		MaxDiskperVolume: getResourceQuantity(limits.Spec.Limits[0].Max, v1.ResourceStorage),
		MinDiskperVolume: getResourceQuantity(limits.Spec.Limits[0].Min, v1.ResourceStorage),
	}
	return &VolumeLimits, nil
}

func GetVolumeQuotaInfo(uid string, namespace string, cfg *types.Config, kubeClientset kubernetes.Interface) (*types.VolumeQuotaResponse, error) {
	EnsureVolumeLimits(uid, namespace, kubeClientset, cfg)

	resources, err := getResouceQuotas(uid, namespace, kubeClientset)
	if err != nil {
		return nil, err
	}
	limits, err := getLimits(uid, namespace, kubeClientset)
	if err != nil {
		return nil, err
	}
	if len(limits.Spec.Limits) == 0 {
		return nil, fmt.Errorf("LimitRange %s/%s has no limits", namespace, uid)
	}
	managedUsage, nonManagedUsage, err := getVolumeUsage(namespace, kubeClientset)
	if err != nil {
		return nil, err
	}
	effectiveDiskMax := getEffectiveStorageQuota(resources.Spec.Hard, nonManagedUsage.Storage)
	effectiveVolumeMax := getEffectivePVCQuota(resources.Spec.Hard, nonManagedUsage.Count)

	volumeQuota := types.VolumeQuotaResponse{
		Disk: types.VolumeQuotaValues{
			Max:  effectiveDiskMax.String(),
			Used: managedUsage.Storage.String(),
		},
		Volumes: types.VolumeQuotaValues{
			Max:  fmt.Sprintf("%d", effectiveVolumeMax),
			Used: fmt.Sprintf("%d", managedUsage.Count),
		},
		MaxDiskperVolume: getResourceQuantity(limits.Spec.Limits[0].Max, v1.ResourceStorage),
		MinDiskperVolume: getResourceQuantity(limits.Spec.Limits[0].Min, v1.ResourceStorage),
	}
	return &volumeQuota, nil
}

func ValidateManagedVolumeQuota(uid string, namespace string, size string, cfg *types.Config, kubeClientset kubernetes.Interface) error {
	quota, err := GetVolumeQuotaInfo(uid, namespace, cfg, kubeClientset)
	if err != nil {
		return err
	}
	requested, err := resource.ParseQuantity(size)
	if err != nil {
		return fmt.Errorf("invalid volume size: %w", err)
	}
	maxDisk, err := resource.ParseQuantity(quota.Disk.Max)
	if err != nil {
		return fmt.Errorf("invalid volume disk quota: %w", err)
	}
	usedDisk, err := resource.ParseQuantity(quota.Disk.Used)
	if err != nil {
		return fmt.Errorf("invalid used volume disk quota: %w", err)
	}
	availableDisk := maxDisk.DeepCopy()
	availableDisk.Sub(usedDisk)
	if availableDisk.Sign() < 0 {
		availableDisk = resource.Quantity{}
	}
	if requested.Cmp(availableDisk) > 0 {
		return fmt.Errorf("not enough volume disk quota: requested %s, available %s", requested.String(), availableDisk.String())
	}
	maxVolumes, err := resource.ParseQuantity(quota.Volumes.Max)
	if err != nil {
		return fmt.Errorf("invalid volume count quota: %w", err)
	}
	usedVolumes, err := resource.ParseQuantity(quota.Volumes.Used)
	if err != nil {
		return fmt.Errorf("invalid used volume count quota: %w", err)
	}
	availableVolumes := maxVolumes.Value() - usedVolumes.Value()
	if availableVolumes < 0 {
		availableVolumes = 0
	}
	if availableVolumes < 1 {
		return fmt.Errorf("not enough volume count quota: requested 1, available %d", availableVolumes)
	}
	return nil
}

func GetNonManagedVolumeUsage(namespace string, kubeClientset kubernetes.Interface) (int, resource.Quantity, error) {
	_, nonManagedUsage, err := getVolumeUsage(namespace, kubeClientset)
	if err != nil {
		return 0, resource.Quantity{}, err
	}
	return nonManagedUsage.Count, nonManagedUsage.Storage, nil
}

type volumeUsage struct {
	Count   int
	Storage resource.Quantity
}

func getVolumeUsage(namespace string, kubeClientset kubernetes.Interface) (volumeUsage, volumeUsage, error) {
	list, err := kubeClientset.CoreV1().PersistentVolumeClaims(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return volumeUsage{}, volumeUsage{}, err
	}

	managed := volumeUsage{}
	nonManaged := volumeUsage{}
	for i := range list.Items {
		usage := &nonManaged
		if list.Items[i].Labels[types.ManagedVolumeLabel] == "true" {
			usage = &managed
		}
		usage.Count++
		if qty, ok := list.Items[i].Spec.Resources.Requests[v1.ResourceStorage]; ok {
			usage.Storage.Add(qty)
		}
	}
	return managed, nonManaged, nil
}

func getEffectiveStorageQuota(list v1.ResourceList, overhead resource.Quantity) resource.Quantity {
	max := resource.Quantity{}
	if qty, ok := list[v1.ResourceRequestsStorage]; ok {
		max = qty.DeepCopy()
	}
	max.Sub(overhead)
	if max.Sign() < 0 {
		return resource.Quantity{}
	}
	return max
}

func getEffectivePVCQuota(list v1.ResourceList, overhead int) int64 {
	var max int64
	if qty, ok := list[v1.ResourcePersistentVolumeClaims]; ok {
		max = qty.Value()
	}
	effective := max - int64(overhead)
	if effective < 0 {
		return 0
	}
	return effective
}

// Create resources
func createResources(name string, namespace string, kubeClientset kubernetes.Interface, vl *types.VolumeLimits) error {
	resource := getResourceDef(name, namespace, *vl)
	if _, err := kubeClientset.CoreV1().ResourceQuotas(namespace).Create(context.TODO(), resource, metav1.CreateOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func createLimits(name string, namespace string, kubeClientset kubernetes.Interface, vl *types.VolumeLimits) error {
	limit := getLimitsDef(name, namespace, *vl)
	if _, err := kubeClientset.CoreV1().LimitRanges(namespace).Create(context.TODO(), limit, metav1.CreateOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// Read resources
func getResouceQuotas(name string, namespace string, kubeClientset kubernetes.Interface) (*v1.ResourceQuota, error) {
	resourcequota, err := kubeClientset.CoreV1().ResourceQuotas(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return resourcequota, nil
}

func getLimits(name string, namespace string, kubeClientset kubernetes.Interface) (*v1.LimitRange, error) {
	limit, err := kubeClientset.CoreV1().LimitRanges(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return limit, nil

}

// get definitions of resources
func getResourceDef(name string, namespace string, vl types.VolumeLimits) *v1.ResourceQuota {
	resourceQuota := v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.ResourceQuotaSpec{
			Hard: v1.ResourceList{
				v1.ResourceRequestsStorage:        resource.MustParse(vl.DiskAvailable),
				v1.ResourcePersistentVolumeClaims: resource.MustParse(vl.MaxVolumes),
			},
		},
	}
	return &resourceQuota

}

func getLimitsDef(name string, namespace string, vl types.VolumeLimits) *v1.LimitRange {
	limits := v1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.LimitRangeSpec{
			Limits: []v1.LimitRangeItem{
				{
					Type: PVC,
					Max: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse(vl.MaxDiskperVolume),
					},
					Min: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse(vl.MinDiskperVolume),
					},
				},
			},
		},
	}
	return &limits
}

// Auxiliary functions
func getResourceQuantity(list v1.ResourceList, key v1.ResourceName) string {
	resourceRequestStorage, exits := list[key]
	if exits {
		return resourceRequestStorage.String()
	} else {
		return "NotFound"
	}
}

func fromConftoVolumeLimits(cfg *types.Config) types.VolumeLimits {
	return types.VolumeLimits{
		DiskAvailable:    cfg.VolumeAvailable,
		MaxVolumes:       cfg.VolumeMax,
		MaxDiskperVolume: cfg.VolumeMaxDisk,
		MinDiskperVolume: cfg.VolumeMinDisk,
	}
}
