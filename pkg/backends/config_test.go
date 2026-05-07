package backends

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestCreateOSCARCMConfiguration(t *testing.T) {
	client := testclient.NewSimpleClientset()

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	err := CreateOSCARCMConfiguration(client, cm, "default")
	if err != nil {
		t.Errorf("CreateOSCARCMConfiguration() error = %v", err)
	}

	result, err := client.CoreV1().ConfigMaps("default").Get(context.TODO(), "test-config", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get configmap: %v", err)
	}
	if result.Data["key1"] != "value1" {
		t.Errorf("Expected key1=value1, got %s", result.Data["key1"])
	}
}

func TestGetOSCARCMConfiguration(t *testing.T) {
	client := testclient.NewSimpleClientset()

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"key1": "value1",
		},
	}
	client.CoreV1().ConfigMaps("default").Create(context.TODO(), cm, metav1.CreateOptions{})

	result, err := GetOSCARCMConfiguration(client, "test-config", "default")
	if err != nil {
		t.Errorf("GetOSCARCMConfiguration() error = %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil ConfigMap")
	}
	if result.Data["key1"] != "value1" {
		t.Errorf("Expected key1=value1, got %s", result.Data["key1"])
	}
}

func TestUpdateOSCARCMConfiguration(t *testing.T) {
	client := testclient.NewSimpleClientset()

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"key1": "value1",
		},
	}
	client.CoreV1().ConfigMaps("default").Create(context.TODO(), cm, metav1.CreateOptions{})

	cm.Data["key1"] = "updated"
	err := UpdateOSCARCMConfiguration(client, cm, "default")
	if err != nil {
		t.Errorf("UpdateOSCARCMConfiguration() error = %v", err)
	}

	result, err := client.CoreV1().ConfigMaps("default").Get(context.TODO(), "test-config", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get configmap: %v", err)
	}
	if result.Data["key1"] != "updated" {
		t.Errorf("Expected key1=updated, got %s", result.Data["key1"])
	}
}