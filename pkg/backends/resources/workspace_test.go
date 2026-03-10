package resources

import (
	"context"
	"testing"

	"github.com/grycap/oscar/v3/pkg/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestEnsureWorkspacePVC(t *testing.T) {
	client := fake.NewSimpleClientset()
	svc := types.Service{
		Name: "svc",
		Workspace: &types.WorkspaceConfig{
			Size:      "1Gi",
			MountPath: "/data",
		},
	}
	if err := EnsureWorkspacePVC(context.Background(), client, svc, "default"); err != nil {
		t.Fatalf("unexpected error creating workspace pvc: %v", err)
	}
	if _, err := client.CoreV1().PersistentVolumeClaims("default").Get(context.Background(), svc.GetWorkspacePVCName(), metav1.GetOptions{}); err != nil {
		t.Fatalf("workspace pvc not found: %v", err)
	}
	pvc, err := client.CoreV1().PersistentVolumeClaims("default").Get(context.Background(), svc.GetWorkspacePVCName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("workspace pvc not found: %v", err)
	}
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != workspaceStorageClassName {
		t.Fatalf("expected workspace pvc storage class %q, got %v", workspaceStorageClassName, pvc.Spec.StorageClassName)
	}
	if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != v1.ReadWriteMany {
		t.Fatalf("expected workspace pvc access mode ReadWriteMany, got %v", pvc.Spec.AccessModes)
	}
}

func TestDeleteWorkspacePVC(t *testing.T) {
	client := fake.NewSimpleClientset()
	svc := types.Service{
		Name: "svc",
		Workspace: &types.WorkspaceConfig{
			Size:      "1Gi",
			MountPath: "/data",
		},
	}
	if err := EnsureWorkspacePVC(context.Background(), client, svc, "default"); err != nil {
		t.Fatalf("unexpected error creating workspace pvc: %v", err)
	}
	if err := DeleteWorkspacePVC(context.Background(), client, svc, "default"); err != nil {
		t.Fatalf("unexpected error deleting workspace pvc: %v", err)
	}
}

func TestEnsureWorkspacePVCReuseFromService(t *testing.T) {
	client := fake.NewSimpleClientset()
	source := types.Service{
		Name: "openclaw-workspace",
		Workspace: &types.WorkspaceConfig{
			Size:      "1Gi",
			MountPath: "/data",
		},
	}
	target := types.Service{
		Name: "workspace-files",
		Workspace: &types.WorkspaceConfig{
			MountPath:        "/data",
			ReuseFromService: source.Name,
		},
	}

	if err := EnsureWorkspacePVC(context.Background(), client, source, "default"); err != nil {
		t.Fatalf("unexpected error creating source workspace pvc: %v", err)
	}
	if err := EnsureWorkspacePVC(context.Background(), client, target, "default"); err != nil {
		t.Fatalf("unexpected error reusing source workspace pvc: %v", err)
	}
	if _, err := client.CoreV1().PersistentVolumeClaims("default").Get(context.Background(), target.GetWorkspacePVCName(), metav1.GetOptions{}); err != nil {
		t.Fatalf("expected referenced workspace pvc to exist: %v", err)
	}
}

func TestEnsureWorkspacePVCReuseFromServiceMissingPVC(t *testing.T) {
	client := fake.NewSimpleClientset()
	target := types.Service{
		Name: "workspace-files",
		Workspace: &types.WorkspaceConfig{
			MountPath:        "/data",
			ReuseFromService: "openclaw-workspace",
		},
	}

	if err := EnsureWorkspacePVC(context.Background(), client, target, "default"); err == nil {
		t.Fatalf("expected error when reusing a workspace pvc that does not exist")
	}
}
