package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/grycap/oscar/v3/pkg/types"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const volumeStorageClassName = "nfs"

var ErrManagedVolumeAttached = errors.New("managed volume is still attached to one or more services")

func EnsureServiceVolume(ctx context.Context, kubeClientset kubernetes.Interface, service types.Service, namespace string) error {
	if service.Volume == nil {
		return nil
	}

	if service.CreatesManagedVolume() {
		return CreateManagedVolume(
			ctx,
			kubeClientset,
			namespace,
			service.Owner,
			service.GetVolumeName(),
			service.Volume.Size,
			types.VolumeCreationModeService,
			service.Name,
			getLifecyclePolicy(service.Volume),
		)
	}

	_, err := kubeClientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, service.GetVolumePVCName(), metav1.GetOptions{})
	return err
}

func DeleteServiceVolume(ctx context.Context, kubeClientset kubernetes.Interface, service types.Service, namespace string) error {
	if service.Volume == nil || !service.CreatesManagedVolume() {
		return nil
	}
	if getLifecyclePolicy(service.Volume) == types.VolumeLifecycleRetain {
		return nil
	}
	return DeleteManagedVolume(ctx, kubeClientset, namespace, service.GetVolumeName(), true)
}

func CreateManagedVolume(ctx context.Context, kubeClientset kubernetes.Interface, namespace, owner, name, size, creationMode, createdByService, lifecyclePolicy string) error {
	requests := v1.ResourceList{
		v1.ResourceStorage: resource.MustParse(size),
	}
	storageClass := volumeStorageClassName

	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				types.ManagedVolumeLabel:                 "true",
				types.ManagedVolumeNameLabel:             name,
				types.ManagedVolumeCreationModeLabel:     creationMode,
				types.ManagedVolumeCreatedByServiceLabel: createdByService,
				types.ManagedVolumeLifecyclePolicyLabel:  lifecyclePolicy,
			},
			Annotations: map[string]string{
				"oscar.grycap/owner-user": owner,
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

	if _, err := kubeClientset.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return err
		}
		return err
	}
	return nil
}

func GetManagedVolume(ctx context.Context, kubeClientset kubernetes.Interface, namespace, name string) (*types.ManagedVolume, error) {
	pvc, err := kubeClientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return managedVolumeFromPVC(ctx, kubeClientset, pvc)
}

func ListManagedVolumes(ctx context.Context, kubeClientset kubernetes.Interface, namespace string) ([]types.ManagedVolume, error) {
	list, err := kubeClientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=true", types.ManagedVolumeLabel),
	})
	if err != nil {
		return nil, err
	}

	volumes := make([]types.ManagedVolume, 0, len(list.Items))
	for i := range list.Items {
		volume, err := managedVolumeFromPVC(ctx, kubeClientset, &list.Items[i])
		if err != nil {
			return nil, err
		}
		volumes = append(volumes, *volume)
	}
	return volumes, nil
}

func DeleteManagedVolume(ctx context.Context, kubeClientset kubernetes.Interface, namespace, name string, force bool) error {
	if !force {
		attachments, err := CountVolumeAttachments(ctx, kubeClientset, namespace, name)
		if err != nil {
			return err
		}
		if attachments > 0 {
			return ErrManagedVolumeAttached
		}
	}

	if err := kubeClientset.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func CountVolumeAttachments(ctx context.Context, kubeClientset kubernetes.Interface, namespace, volumeName string) (int, error) {
	services, err := ListServicesUsingVolume(ctx, kubeClientset, namespace, volumeName)
	if err != nil {
		return 0, err
	}
	return len(services), nil
}

func ListServicesUsingVolume(ctx context.Context, kubeClientset kubernetes.Interface, namespace, volumeName string) ([]string, error) {
	configMaps, err := kubeClientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	services := []string{}
	for _, cm := range configMaps.Items {
		rawFDL, ok := cm.Data[types.FDLFileName]
		if !ok || rawFDL == "" {
			continue
		}

		service := &types.Service{}
		if err := yaml.Unmarshal([]byte(rawFDL), service); err != nil {
			continue
		}
		if service.Volume != nil && service.GetVolumeName() == volumeName {
			services = append(services, service.Name)
		}
	}

	return services, nil
}

func managedVolumeFromPVC(ctx context.Context, kubeClientset kubernetes.Interface, pvc *v1.PersistentVolumeClaim) (*types.ManagedVolume, error) {
	if pvc == nil {
		return nil, fmt.Errorf("nil pvc")
	}

	name := pvc.Labels[types.ManagedVolumeNameLabel]
	if name == "" {
		name = pvc.Name
	}
	attachmentCount, err := CountVolumeAttachments(ctx, kubeClientset, pvc.Namespace, name)
	if err != nil {
		return nil, err
	}

	phase := types.VolumePhaseReady
	if pvc.DeletionTimestamp != nil {
		phase = types.VolumePhaseDeleting
	} else if attachmentCount > 0 {
		phase = types.VolumePhaseInUse
	} else if pvc.Status.Phase == v1.ClaimPending {
		phase = types.VolumePhasePending
	}

	size := ""
	if qty, ok := pvc.Spec.Resources.Requests[v1.ResourceStorage]; ok {
		size = qty.String()
	}

	return &types.ManagedVolume{
		Name:             name,
		Namespace:        pvc.Namespace,
		PVCName:          pvc.Name,
		Size:             size,
		OwnerUser:        pvc.Annotations["oscar.grycap/owner-user"],
		CreatedByService: pvc.Labels[types.ManagedVolumeCreatedByServiceLabel],
		CreationMode:     pvc.Labels[types.ManagedVolumeCreationModeLabel],
		LifecyclePolicy:  pvc.Labels[types.ManagedVolumeLifecyclePolicyLabel],
		Status: types.VolumeStatus{
			Phase:           phase,
			AttachmentCount: attachmentCount,
		},
	}, nil
}

func getLifecyclePolicy(volume *types.ServiceVolumeConfig) string {
	if volume == nil {
		return ""
	}
	policy := strings.TrimSpace(volume.LifecyclePolicy)
	if policy == "" {
		return types.VolumeLifecycleDelete
	}
	return policy
}
