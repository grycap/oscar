package resources

import (
	"context"
	"testing"

	"github.com/grycap/oscar/v4/pkg/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestEnsureServiceVolumeCreate(t *testing.T) {
	client := fake.NewSimpleClientset()
	cfg := &types.Config{StorageClassName: "nfs"}
	svc := types.Service{
		Name:  "svc",
		Owner: "owner",
		Volume: &types.ServiceVolumeConfig{
			Size:      "1Gi",
			MountPath: "/data",
		},
	}
	if err := EnsureServiceVolume(context.Background(), cfg, client, svc, "default"); err != nil {
		t.Fatalf("unexpected error creating volume pvc: %v", err)
	}
	if _, err := client.CoreV1().PersistentVolumeClaims("default").Get(context.Background(), svc.GetVolumePVCName(), metav1.GetOptions{}); err != nil {
		t.Fatalf("volume pvc not found: %v", err)
	}
	pvc, err := client.CoreV1().PersistentVolumeClaims("default").Get(context.Background(), svc.GetVolumePVCName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("volume pvc not found: %v", err)
	}
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != cfg.StorageClassName {
		t.Fatalf("expected volume pvc storage class %q, got %v", cfg.StorageClassName, pvc.Spec.StorageClassName)
	}
	if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != v1.ReadWriteMany {
		t.Fatalf("expected volume pvc access mode ReadWriteMany, got %v", pvc.Spec.AccessModes)
	}
}

func TestDeleteServiceVolume(t *testing.T) {
	client := fake.NewSimpleClientset()
	cfg := &types.Config{StorageClassName: "nfs"}
	svc := types.Service{
		Name:  "svc",
		Owner: "owner",
		Volume: &types.ServiceVolumeConfig{
			Size:      "1Gi",
			MountPath: "/data",
		},
	}
	if err := EnsureServiceVolume(context.Background(), cfg, client, svc, "default"); err != nil {
		t.Fatalf("unexpected error creating volume pvc: %v", err)
	}
	if err := DeleteServiceVolume(context.Background(), client, svc, "default"); err != nil {
		t.Fatalf("unexpected error deleting volume pvc: %v", err)
	}
}

func TestEnsureServiceVolumeMountExisting(t *testing.T) {
	client := fake.NewSimpleClientset()
	cfg := &types.Config{StorageClassName: "nfs"}
	target := types.Service{
		Name: "consumer",
		Volume: &types.ServiceVolumeConfig{
			Name:      "shared-data",
			MountPath: "/data",
		},
	}
	_, _ = client.CoreV1().PersistentVolumeClaims("default").Create(context.Background(), &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-data",
			Namespace: "default",
			Labels: map[string]string{
				types.ManagedVolumeLabel:     "true",
				types.ManagedVolumeNameLabel: "shared-data",
			},
		},
	}, metav1.CreateOptions{})

	if err := EnsureServiceVolume(context.Background(), cfg, client, target, "default"); err != nil {
		t.Fatalf("unexpected error mounting existing volume: %v", err)
	}
	if _, err := client.CoreV1().PersistentVolumeClaims("default").Get(context.Background(), target.GetVolumePVCName(), metav1.GetOptions{}); err != nil {
		t.Fatalf("expected referenced volume pvc to exist: %v", err)
	}
}

func TestEnsureServiceVolumeMissingMountedPVC(t *testing.T) {
	client := fake.NewSimpleClientset()
	cfg := &types.Config{StorageClassName: "nfs"}
	target := types.Service{
		Name: "consumer",
		Volume: &types.ServiceVolumeConfig{
			Name:      "shared-data",
			MountPath: "/data",
		},
	}

	if err := EnsureServiceVolume(context.Background(), cfg, client, target, "default"); err == nil {
		t.Fatalf("expected error when mounting a volume pvc that does not exist")
	}
}

func TestListManagedVolumesAndAttachments(t *testing.T) {
	client := fake.NewSimpleClientset(
		&v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shared-data",
				Namespace: "default",
				Labels: map[string]string{
					types.ManagedVolumeLabel:                 "true",
					types.ManagedVolumeNameLabel:             "shared-data",
					types.ManagedVolumeCreationModeLabel:     types.VolumeCreationModeService,
					types.ManagedVolumeCreatedByServiceLabel: "creator",
					types.ManagedVolumeLifecyclePolicyLabel:  types.VolumeLifecycleRetain,
				},
				Annotations: map[string]string{
					"oscar.grycap/owner-user": "owner",
				},
			},
		},
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "consumer",
				Namespace: "default",
			},
			Data: map[string]string{
				types.FDLFileName: "name: consumer\nvolume:\n  name: shared-data\n  mount_path: /data\n",
			},
		},
	)

	volumes, err := ListManagedVolumes(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("unexpected error listing managed volumes: %v", err)
	}
	if len(volumes) != 1 {
		t.Fatalf("expected one managed volume, got %d", len(volumes))
	}
	if volumes[0].Name != "shared-data" || volumes[0].Status.AttachmentCount != 1 || volumes[0].Status.Phase != types.VolumePhaseInUse {
		t.Fatalf("unexpected managed volume payload: %+v", volumes[0])
	}
	if len(volumes[0].Attachments) != 1 {
		t.Fatalf("expected one attachment in payload, got %+v", volumes[0])
	}
	if volumes[0].Attachments[0].ServiceName != "consumer" {
		t.Fatalf("unexpected attachment service: %+v", volumes[0].Attachments)
	}
	if volumes[0].Attachments[0].MountPath != "/data" {
		t.Fatalf("unexpected attachment mount path: %+v", volumes[0].Attachments)
	}

	attachments, err := CountVolumeAttachments(context.Background(), client, "default", "shared-data")
	if err != nil {
		t.Fatalf("unexpected attachment count error: %v", err)
	}
	if attachments != 1 {
		t.Fatalf("expected one attachment, got %d", attachments)
	}

	references, err := ListVolumeAttachments(context.Background(), client, "default", "shared-data")
	if err != nil {
		t.Fatalf("unexpected attachment listing error: %v", err)
	}
	if len(references) != 1 {
		t.Fatalf("expected one attachment reference, got %+v", references)
	}
	if references[0].ServiceName != "consumer" || references[0].MountPath != "/data" {
		t.Fatalf("unexpected attachment reference: %+v", references[0])
	}
}
