package utils

import (
	"testing"

	v1 "k8s.io/api/core/v1"
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

func TestMergeSecretData(t *testing.T) {
	client := fake.NewSimpleClientset()
	namespace := "oscar-svc"

	// Seed the secret with Data directly, mimicking the apiserver materializing
	// StringData into Data after a CreateSecret call.
	if _, err := client.CoreV1().Secrets(namespace).Create(t.Context(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "merge-secret", Namespace: namespace},
		Data:       map[string][]byte{"key1": []byte("value1")},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("seed secret error: %v", err)
	}

	if err := MergeSecretData("merge-secret", namespace, map[string]string{"key2": "value2"}, client); err != nil {
		t.Fatalf("MergeSecretData error: %v", err)
	}

	secret, err := GetSecret("merge-secret", namespace, client)
	if err != nil {
		t.Fatalf("GetSecret error: %v", err)
	}
	if string(secret.Data["key1"]) != "value1" {
		t.Fatalf("expected existing key to be preserved, got %q", string(secret.Data["key1"]))
	}
	if string(secret.Data["key2"]) != "value2" {
		t.Fatalf("expected new key to be merged, got %q", string(secret.Data["key2"]))
	}
}

func TestMergeSecretDataCreatesSecret(t *testing.T) {
	client := fake.NewSimpleClientset()
	namespace := "oscar-svc"

	if err := MergeSecretData("new-secret", namespace, map[string]string{"key1": "value1"}, client); err != nil {
		t.Fatalf("MergeSecretData error: %v", err)
	}
	if !SecretExists("new-secret", namespace, client) {
		t.Fatalf("expected secret to be created")
	}
}

func TestGetSecretNotFound(t *testing.T) {
	client := fake.NewSimpleClientset()
	_, err := GetSecret("missing", "oscar-svc", client)
	if err == nil {
		t.Fatalf("expected error for missing secret")
	}
}
