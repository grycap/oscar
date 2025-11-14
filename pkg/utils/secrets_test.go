package utils

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSecretCRUD(t *testing.T) {
	client := fake.NewSimpleClientset()
	namespace := "oscar-svc"
	data := map[string]string{"api-key": "value"}

	if err := CreateSecret("test-secret", namespace, data, client); err != nil {
		t.Fatalf("CreateSecret error: %v", err)
	}
	if !SecretExists("test-secret", namespace, client) {
		t.Fatalf("expected secret to exist after creation")
	}

	updated := map[string]string{"api-key": "updated"}
	if err := UpdateSecretData("test-secret", namespace, updated, client); err != nil {
		t.Fatalf("UpdateSecretData error: %v", err)
	}

	if _, err := client.CoreV1().Secrets(namespace).Get(t.Context(), "test-secret", metav1.GetOptions{}); err != nil {
		t.Fatalf("failed retrieving secret after update: %v", err)
	}

	if err := DeleteSecret("test-secret", namespace, client); err != nil {
		t.Fatalf("DeleteSecret error: %v", err)
	}
	if SecretExists("test-secret", namespace, client) {
		t.Fatalf("expected secret to be removed")
	}
}
