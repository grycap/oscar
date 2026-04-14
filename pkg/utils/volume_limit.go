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

	"github.com/grycap/oscar/v3/pkg/types"
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

	if errQuotas == nil {
		QuotasLogger.Printf("Creating ResourceQuota %s", name)
		// #nosec
		createResources(name, namespace, kubeClientset, &quota)
	} else {
		QuotasLogger.Printf("ResourceQuota %s already created", name)
	}

	_, errLimits := getLimits(name, namespace, kubeClientset)
	if errLimits == nil {
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
