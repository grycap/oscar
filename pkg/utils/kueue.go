package utils

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/grycap/oscar/v3/pkg/types"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/rest"
	kueuev1 "sigs.k8s.io/kueue/apis/kueue/v1beta2"
	kueueclientset "sigs.k8s.io/kueue/client-go/clientset/versioned"
)

const (
	defaultKueueQueuePrefix      = "oscar-cq"
	defaultKueueLocalQueuePrefix = "oscar-lq"
)

// EnsureKueueUserQueues makes sure the user ClusterQueue and the service LocalQueue exist with default quotas.
// It is idempotent and will no-op if Kueue is disabled.
func EnsureKueueUserQueues(ctx context.Context, cfg *types.Config, serviceNamespace, owner, serviceName string) error {
	if !cfg.KueueEnable {
		return nil
	}

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("unable to build in-cluster config for kueue: %w", err)
	}

	kueueClient, err := kueueclientset.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("unable to create kueue client: %w", err)
	}

	flavorName := sanitizeKueueName(cfg.KueueDefaultFlavor)
	if err := ensureResourceFlavor(ctx, kueueClient, flavorName); err != nil {
		return fmt.Errorf("ensuring kueue ResourceFlavor: %w", err)
	}

	clusterQueueName := buildClusterQueueName(owner)
	if err := ensureClusterQueue(ctx, kueueClient, cfg, clusterQueueName, flavorName, owner); err != nil {
		return fmt.Errorf("ensuring kueue ClusterQueue: %w", err)
	}

	if err := ensureLocalQueue(ctx, kueueClient, serviceNamespace, serviceName, clusterQueueName, owner); err != nil {
		return fmt.Errorf("ensuring kueue LocalQueue: %w", err)
	}

	return nil
}

func ensureResourceFlavor(ctx context.Context, kueueClient *kueueclientset.Clientset, flavorName string) error {
	_, err := kueueClient.KueueV1beta2().ResourceFlavors().Get(ctx, flavorName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	flavor := &kueuev1.ResourceFlavor{
		ObjectMeta: metav1.ObjectMeta{
			Name: flavorName,
			Labels: map[string]string{
				types.KueueOwnerLabel: defaultKueueQueuePrefix,
			},
		},
		Spec: kueuev1.ResourceFlavorSpec{
			NodeLabels: map[string]string{},
		},
	}

	_, err = kueueClient.KueueV1beta2().ResourceFlavors().Create(ctx, flavor, metav1.CreateOptions{})
	return err
}

func ensureClusterQueue(ctx context.Context, kueueClient *kueueclientset.Clientset, cfg *types.Config, cqName, flavorName, owner string) error {
	cpuQuota, err := resource.ParseQuantity(cfg.KueueDefaultCPU)
	if err != nil {
		return fmt.Errorf("invalid Kueue default CPU quota %q: %w", cfg.KueueDefaultCPU, err)
	}
	memoryQuota, err := resource.ParseQuantity(cfg.KueueDefaultMemory)
	if err != nil {
		return fmt.Errorf("invalid Kueue default memory quota %q: %w", cfg.KueueDefaultMemory, err)
	}

	cq := &kueuev1.ClusterQueue{
		ObjectMeta: metav1.ObjectMeta{
			Name: cqName,
			Labels: map[string]string{
				types.KueueOwnerLabel: sanitizeKueueName(owner),
			},
		},
		Spec: kueuev1.ClusterQueueSpec{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					namespaceOwnerHashLabel: ownerHash(owner),
				},
			},
			ResourceGroups: []kueuev1.ResourceGroup{
				{
					CoveredResources: []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory},
					Flavors: []kueuev1.FlavorQuotas{
						{
							Name: kueuev1.ResourceFlavorReference(flavorName),
							Resources: []kueuev1.ResourceQuota{
								{
									Name:         corev1.ResourceCPU,
									NominalQuota: cpuQuota,
								},
								{
									Name:         corev1.ResourceMemory,
									NominalQuota: memoryQuota,
								},
							},
						},
					},
				},
			},
		},
	}

	current, err := kueueClient.KueueV1beta2().ClusterQueues().Get(ctx, cqName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = kueueClient.KueueV1beta2().ClusterQueues().Create(ctx, cq, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}

	// Reconcile quotas if missing/zero so queues are usable without manual patching.
	if current.Spec.NamespaceSelector == nil ||
		len(current.Spec.ResourceGroups) == 0 ||
		len(current.Spec.ResourceGroups[0].Flavors) == 0 ||
		len(current.Spec.ResourceGroups[0].Flavors[0].Resources) == 0 ||
		!reflect.DeepEqual(current.Spec.ResourceGroups, cq.Spec.ResourceGroups) ||
		!reflect.DeepEqual(current.Spec.NamespaceSelector, cq.Spec.NamespaceSelector) {
		current.Spec.ResourceGroups = cq.Spec.ResourceGroups
		current.Spec.NamespaceSelector = cq.Spec.NamespaceSelector
		_, err = kueueClient.KueueV1beta2().ClusterQueues().Update(ctx, current, metav1.UpdateOptions{})
	}
	return err
}

func ensureLocalQueue(ctx context.Context, kueueClient *kueueclientset.Clientset, namespace, serviceName, clusterQueueName, owner string) error {
	lqName := BuildLocalQueueName(serviceName)
	lq, err := kueueClient.KueueV1beta2().LocalQueues(namespace).Get(ctx, lqName, metav1.GetOptions{})
	if err == nil {
		if string(lq.Spec.ClusterQueue) != clusterQueueName {
			lq.Spec.ClusterQueue = kueuev1.ClusterQueueReference(clusterQueueName)
			if lq.Labels == nil {
				lq.Labels = map[string]string{}
			}
			lq.Labels[types.KueueOwnerLabel] = sanitizeKueueName(owner)
			_, err = kueueClient.KueueV1beta2().LocalQueues(namespace).Update(ctx, lq, metav1.UpdateOptions{})
		}
		return err
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	localQueue := &kueuev1.LocalQueue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lqName,
			Namespace: namespace,
			Labels: map[string]string{
				types.KueueOwnerLabel: sanitizeKueueName(owner),
			},
		},
		Spec: kueuev1.LocalQueueSpec{
			ClusterQueue: kueuev1.ClusterQueueReference(clusterQueueName),
		},
	}
	_, err = kueueClient.KueueV1beta2().LocalQueues(namespace).Create(ctx, localQueue, metav1.CreateOptions{})
	return err
}

// DeleteKueueLocalQueue deletes the LocalQueue associated with a service. It does not delete the ClusterQueue.
func DeleteKueueLocalQueue(ctx context.Context, cfg *types.Config, namespace, serviceName string) error {
	if !cfg.KueueEnable {
		return nil
	}

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("unable to build in-cluster config for kueue: %w", err)
	}

	kueueClient, err := kueueclientset.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("unable to create kueue client: %w", err)
	}

	err = kueueClient.KueueV1beta2().LocalQueues(namespace).Delete(ctx, BuildLocalQueueName(serviceName), metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func buildClusterQueueName(owner string) string {
	return sanitizeKueueName(fmt.Sprintf("%s-%s", defaultKueueQueuePrefix, owner))
}

// BuildClusterQueueName returns the canonical ClusterQueue name for a given owner.
func BuildClusterQueueName(owner string) string {
	return buildClusterQueueName(owner)
}

// SanitizeKueueName exposes the internal sanitizer for consumers that must
// reference Kueue objects (e.g., when annotating Jobs with a LocalQueue name).
func SanitizeKueueName(value string) string {
	return sanitizeKueueName(value)
}

// BuildLocalQueueName builds the canonical LocalQueue name for a service.
func BuildLocalQueueName(serviceName string) string {
	return sanitizeKueueName(fmt.Sprintf("%s-%s", defaultKueueLocalQueuePrefix, serviceName))
}

func sanitizeKueueName(value string) string {
	value = strings.ToLower(value)

	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}

	clean := strings.Trim(b.String(), "-")
	if len(clean) > validation.DNS1123LabelMaxLength {
		clean = clean[:validation.DNS1123LabelMaxLength]
		clean = strings.TrimRight(clean, "-")
	}
	if clean == "" {
		return defaultKueueQueuePrefix
	}
	return clean
}
