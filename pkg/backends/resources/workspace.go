package resources

import (
	"context"

	"github.com/grycap/oscar/v3/pkg/types"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const workspaceStorageClassName = "nfs"

// EnsureWorkspacePVC creates the workspace PVC when the service enables workspace support.
func EnsureWorkspacePVC(ctx context.Context, kubeClientset kubernetes.Interface, service types.Service, namespace string) error {
	if service.Workspace == nil {
		return nil
	}
	if service.Workspace.ReuseFromService != "" {
		_, err := kubeClientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, service.GetWorkspacePVCName(), metav1.GetOptions{})
		return err
	}

	requests := v1.ResourceList{
		v1.ResourceStorage: resource.MustParse(service.Workspace.Size),
	}
	storageClass := workspaceStorageClassName
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.GetWorkspacePVCName(),
			Namespace: namespace,
			Labels: map[string]string{
				types.ServiceLabel: service.Name,
			},
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
			StorageClassName: &storageClass,
			Resources: v1.VolumeResourceRequirements{
				Requests: requests,
			},
		},
	}

	if _, err := kubeClientset.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// DeleteWorkspacePVC removes the workspace PVC associated with the service.
func DeleteWorkspacePVC(ctx context.Context, kubeClientset kubernetes.Interface, service types.Service, namespace string) error {
	if service.Workspace == nil {
		return nil
	}
	if service.Workspace.ReuseFromService != "" {
		return nil
	}
	if err := kubeClientset.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, service.GetWorkspacePVCName(), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
