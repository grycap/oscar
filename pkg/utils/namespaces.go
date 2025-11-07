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
	"crypto/sha1" // #nosec G505 -- used for deterministic short hash, not for security
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/grycap/oscar/v3/pkg/types"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
)

const (
	maxNamespaceLength          = 63
	controllerRoleName          = "oscar-controller"
	controllerRoleBindingName   = "oscar-controller-binding"
	namespaceManagedByLabel     = "app.kubernetes.io/managed-by"
	namespaceManagedByValue     = "oscar"
	namespaceOwnerLabel         = "oscar.grycap.upv.es/owner"
	namespaceOwnerHashLabel     = "oscar.grycap.upv.es/owner-hash"
	namespaceLifecycleLabel     = "oscar.grycap.upv.es/lifecycle"
	namespaceLifecycleActive    = "active"
	namespaceSanitizePattern    = "[^a-z0-9-]+"
	namespaceHashPaddingDivider = "-"
)

var sanitizeRegexp = regexp.MustCompile(namespaceSanitizePattern)

// BuildUserNamespace returns the namespace name that should be used to deploy
// services owned by the provided user. When no owner is provided (i.e. cluster admin)
// the configured ServicesNamespace is returned.
func BuildUserNamespace(cfg *types.Config, owner string) string {
	if cfg == nil {
		return ""
	}

	if owner == "" || owner == types.DefaultOwner {
		return cfg.ServicesNamespace
	}

	prefix := cfg.ServicesNamespace
	if prefix == "" {
		prefix = "oscar-svc"
	}

	hash := ownerHash(owner)
	sanitized := sanitizeOwner(owner)

	maxSuffixLen := maxNamespaceLength - len(prefix) - 1
	if maxSuffixLen <= 0 {
		// Prefix already uses all the available length; fall back to hash only
		return truncateLabel(fmt.Sprintf("%s-%s", prefix, hash), maxNamespaceLength)
	}

	if len(sanitized) == 0 {
		return fmt.Sprintf("%s-%s", prefix, truncateLabel(hash, maxSuffixLen))
	}

	if len(sanitized) > maxSuffixLen {
		sanitized = truncateLabel(sanitized, maxSuffixLen)
	}

	if len(sanitized) == maxSuffixLen {
		return fmt.Sprintf("%s-%s", prefix, sanitized)
	}

	remaining := maxSuffixLen - len(sanitized) - len(namespaceHashPaddingDivider)
	if remaining <= 0 {
		return fmt.Sprintf("%s-%s", prefix, sanitized)
	}

	return fmt.Sprintf("%s-%s%s%s", prefix, sanitized, namespaceHashPaddingDivider, truncateLabel(hash, remaining))
}

// EnsureUserNamespace makes sure the namespace and associated RBAC resources exist.
// It returns the namespace name that should be used for the current owner.
func EnsureUserNamespace(ctx context.Context, kubeClientset kubernetes.Interface, cfg *types.Config, owner string) (string, error) {
	if kubeClientset == nil {
		return "", fmt.Errorf("kubernetes clientset cannot be nil")
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	nsName := BuildUserNamespace(cfg, owner)
	if nsName == "" {
		return "", fmt.Errorf("unable to resolve namespace for owner %q", owner)
	}

	nsClient := kubeClientset.CoreV1().Namespaces()
	ns, err := nsClient.Get(ctx, nsName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}

		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: nsName,
				Labels: map[string]string{
					namespaceManagedByLabel: namespaceManagedByValue,
					namespaceLifecycleLabel: namespaceLifecycleActive,
					namespaceOwnerHashLabel: ownerHash(owner),
				},
				Annotations: map[string]string{
					namespaceOwnerLabel: owner,
				},
			},
		}
		if owner == "" || owner == types.DefaultOwner {
			// do not leak owner information for the shared namespace
			delete(ns.Annotations, namespaceOwnerLabel)
			delete(ns.Labels, namespaceOwnerHashLabel)
		}

		if _, err = nsClient.Create(ctx, ns, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return "", err
		}
	} else {
		// Ensure the namespace has the expected labels and annotations
		updated := false
		if ns.Labels == nil {
			ns.Labels = map[string]string{}
		}
		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}
		if ns.Labels[namespaceManagedByLabel] != namespaceManagedByValue {
			ns.Labels[namespaceManagedByLabel] = namespaceManagedByValue
			updated = true
		}
		if owner != "" && owner != types.DefaultOwner {
			if ns.Labels[namespaceOwnerHashLabel] != ownerHash(owner) {
				ns.Labels[namespaceOwnerHashLabel] = ownerHash(owner)
				updated = true
			}
			if ns.Annotations[namespaceOwnerLabel] != owner {
				ns.Annotations[namespaceOwnerLabel] = owner
				updated = true
			}
		}
		if updated {
			if _, err := nsClient.Update(ctx, ns, metav1.UpdateOptions{}); err != nil {
				return "", err
			}
		}
	}

	// Shared namespace does not require per-user RBAC bindings
	if owner == "" || owner == types.DefaultOwner {
		return nsName, nil
	}

	if err := ensureControllerRole(ctx, kubeClientset, nsName); err != nil {
		return "", err
	}

	if err := ensureControllerRoleBinding(ctx, kubeClientset, cfg, nsName); err != nil {
		return "", err
	}

	if err := ensureSharedRuntimePVC(ctx, kubeClientset, cfg, nsName); err != nil {
		return "", err
	}

	return nsName, nil
}

func ensureControllerRole(ctx context.Context, kubeClientset kubernetes.Interface, namespace string) error {
	roleClient := kubeClientset.RbacV1().Roles(namespace)
	if _, err := roleClient.Get(ctx, controllerRoleName, metav1.GetOptions{}); err == nil {
		return nil
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerRoleName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "pods/log", "podtemplates", "configmaps", "secrets", "services", "persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "watch", "create", "delete", "update"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"daemonsets", "deployments"},
				Verbs:     []string{"get", "list", "watch", "create", "delete", "update"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs"},
				Verbs:     []string{"get", "list", "watch", "create", "delete", "deletecollection"},
			},
			{
				APIGroups: []string{"autoscaling"},
				Resources: []string{"horizontalpodautoscalers"},
				Verbs:     []string{"get", "list", "watch", "create", "delete", "update"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"ingresses"},
				Verbs:     []string{"get", "list", "watch", "create", "delete", "update"},
			},
			{
				APIGroups: []string{"serving.knative.dev"},
				Resources: []string{"services"},
				Verbs:     []string{"get", "list", "watch", "create", "delete", "update"},
			},
		},
	}

	_, err := roleClient.Create(ctx, role, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func ensureControllerRoleBinding(ctx context.Context, kubeClientset kubernetes.Interface, cfg *types.Config, namespace string) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	rbClient := kubeClientset.RbacV1().RoleBindings(namespace)
	if _, err := rbClient.Get(ctx, controllerRoleBindingName, metav1.GetOptions{}); err == nil {
		return nil
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	serviceAccount := cfg.ControllerServiceAccount
	if serviceAccount == "" {
		serviceAccount = "oscar-sa"
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerRoleBindingName,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     controllerRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount,
				Namespace: cfg.Namespace,
			},
		},
	}

	_, err := rbClient.Create(ctx, rb, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func ensureSharedRuntimePVC(ctx context.Context, kubeClientset kubernetes.Interface, cfg *types.Config, namespace string) error {
	baseNamespace := cfg.ServicesNamespace
	if baseNamespace == "" {
		baseNamespace = "oscar-svc"
	}

	pvcClient := kubeClientset.CoreV1().PersistentVolumeClaims(namespace)
	if _, err := pvcClient.Get(ctx, types.PVCName, metav1.GetOptions{}); err == nil {
		return nil
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	basePVC, err := kubeClientset.CoreV1().PersistentVolumeClaims(baseNamespace).Get(ctx, types.PVCName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error retrieving base pvc %s/%s: %w", baseNamespace, types.PVCName, err)
	}

	if basePVC.Status.Phase != corev1.ClaimBound {
		return fmt.Errorf("base pvc %s/%s must be bound before creating shared pvc", baseNamespace, types.PVCName)
	}

	if basePVC.Spec.VolumeName == "" {
		return fmt.Errorf("base pvc %s/%s is not bound to a persistent volume", baseNamespace, types.PVCName)
	}

	basePV, err := kubeClientset.CoreV1().PersistentVolumes().Get(ctx, basePVC.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error retrieving base persistent volume %s: %w", basePVC.Spec.VolumeName, err)
	}

	pvName := buildSharedPVName(basePV.Name, namespace)
	if _, err := kubeClientset.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		newPV := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvName,
				Labels: map[string]string{
					namespaceManagedByLabel: namespaceManagedByValue,
					namespaceOwnerHashLabel: ownerHash(namespace),
				},
			},
			Spec: buildSharedPVSpec(basePV.Spec),
		}

		if _, err := kubeClientset.CoreV1().PersistentVolumes().Create(ctx, newPV, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("error creating shared persistent volume %s: %w", pvName, err)
		}
	}

	storageClassName := ""
	pvcSpec := basePVC.Spec.DeepCopy()
	pvcSpec.VolumeName = pvName
	pvcSpec.StorageClassName = &storageClassName
	pvcSpec.DataSource = nil
	pvcSpec.DataSourceRef = nil
	pvcSpec.Selector = nil

	if pvcSpec.VolumeMode == nil {
		pvcSpec.VolumeMode = basePVC.Spec.VolumeMode
	}

	userPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      types.PVCName,
			Namespace: namespace,
			Labels: map[string]string{
				namespaceManagedByLabel: namespaceManagedByValue,
				namespaceOwnerHashLabel: ownerHash(namespace),
			},
		},
		Spec: *pvcSpec,
	}

	if _, err := pvcClient.Create(ctx, userPVC, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating shared pvc %s/%s: %w", namespace, types.PVCName, err)
	}

	return nil
}

func buildSharedPVSpec(base corev1.PersistentVolumeSpec) corev1.PersistentVolumeSpec {
	capacity := base.Capacity.DeepCopy()
	mountOptions := append([]string{}, base.MountOptions...)
	if !containsString(mountOptions, "ro") {
		mountOptions = append(mountOptions, "ro")
	}

	var nodeAffinity *corev1.VolumeNodeAffinity
	if base.NodeAffinity != nil {
		nodeAffinity = base.NodeAffinity.DeepCopy()
	}

	spec := corev1.PersistentVolumeSpec{
		Capacity:                      capacity,
		AccessModes:                   append([]corev1.PersistentVolumeAccessMode{}, base.AccessModes...),
		PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
		PersistentVolumeSource:        base.PersistentVolumeSource,
		MountOptions:                  mountOptions,
		StorageClassName:              "",
		NodeAffinity:                  nodeAffinity,
		VolumeMode:                    base.VolumeMode,
	}

	spec.ClaimRef = nil

	return spec
}

func buildSharedPVName(baseName, namespace string) string {
	hash := ownerHash(namespace)
	if len(hash) > 10 {
		hash = hash[:10]
	}

	maxBaseLen := maxNamespaceLength - len(hash) - 1
	base := truncateLabel(baseName, maxBaseLen)
	if base == "" {
		base = "oscar-pv"
	}

	return fmt.Sprintf("%s-%s", base, hash)
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func sanitizeOwner(owner string) string {
	if owner == "" {
		return ""
	}

	sanitized := strings.ToLower(owner)
	sanitized = sanitizeRegexp.ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-")
	for len(validation.IsDNS1123Label(sanitized)) > 0 && len(sanitized) > 0 {
		sanitized = strings.Trim(sanitized, "-")
		if len(sanitized) > maxNamespaceLength {
			sanitized = sanitized[:maxNamespaceLength]
		}
		if len(validation.IsDNS1123Label(sanitized)) == 0 {
			break
		}
		// if still invalid, fall back to empty so hash is used
		return ""
	}

	return sanitized
}

func truncateLabel(label string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(label) <= max {
		return strings.Trim(label, "-")
	}
	truncated := label[:max]
	truncated = strings.Trim(truncated, "-")
	return truncated
}

func ownerHash(owner string) string {
	if owner == "" {
		return ""
	}
	hash := sha1.Sum([]byte(owner))
	return hex.EncodeToString(hash[:])
}
