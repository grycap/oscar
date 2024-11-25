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

package auth

import (
	"context"
	"encoding/base64"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewMultitenancyConfig(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	uid := "test-uid"
	mc := NewMultitenancyConfig(clientset, uid)

	if mc.owner_uid != uid {
		t.Errorf("expected owner_uid to be %s, got %s", uid, mc.owner_uid)
	}
}

func TestUpdateCache(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	mc := NewMultitenancyConfig(clientset, "test-uid")

	mc.UpdateCache("user1")
	if len(mc.usersCache) != 1 {
		t.Errorf("expected usersCache length to be 1, got %d", len(mc.usersCache))
	}
}

func TestClearCache(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	mc := NewMultitenancyConfig(clientset, "test-uid")

	mc.UpdateCache("user1")
	mc.ClearCache()
	if len(mc.usersCache) != 0 {
		t.Errorf("expected usersCache length to be 0, got %d", len(mc.usersCache))
	}
}

func TestUserExists(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	mc := NewMultitenancyConfig(clientset, "test-uid")

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user1",
			Namespace: ServicesNamespace,
		},
	}
	clientset.CoreV1().Secrets(ServicesNamespace).Create(context.TODO(), secret, metav1.CreateOptions{})

	exists := mc.UserExists("user1@egi.eu")
	if !exists {
		t.Errorf("expected user1 to exist")
	}
}

func TestCreateSecretForOIDC(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	mc := NewMultitenancyConfig(clientset, "test-uid")

	err := mc.CreateSecretForOIDC("user1@egi.eu", "secret-key")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	secret, err := clientset.CoreV1().Secrets(ServicesNamespace).Get(context.TODO(), "user1", metav1.GetOptions{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if secret.StringData["secretKey"] != "secret-key" {
		t.Errorf("expected secretKey to be 'secret-key', got %s", secret.StringData["secretKey"])
	}
}

func TestGetUserCredentials(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	mc := NewMultitenancyConfig(clientset, "test-uid")

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user1",
			Namespace: ServicesNamespace,
		},
		Data: map[string][]byte{
			"accessKey": []byte("access-key"),
			"secretKey": []byte("secret-key"),
		},
	}
	clientset.CoreV1().Secrets(ServicesNamespace).Create(context.TODO(), secret, metav1.CreateOptions{})

	accessKey, secretKey, err := mc.GetUserCredentials("user1@egi.eu")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if accessKey != "access-key" {
		t.Errorf("expected accessKey to be 'access-key', got %s", accessKey)
	}

	if secretKey != "secret-key" {
		t.Errorf("expected secretKey to be 'secret-key', got %s", secretKey)
	}
}

func TestGenerateRandomKey(t *testing.T) {
	key, err := GenerateRandomKey(32)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	dkey, _ := base64.RawURLEncoding.DecodeString(key)
	if len(dkey) != 32 {
		t.Errorf("expected key length to be 32, got %d", len(key))
	}
}

func TestCheckUsersInCache(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	mc := NewMultitenancyConfig(clientset, "test-uid")

	mc.UpdateCache("user1")
	mc.UpdateCache("user2")

	notFoundUsers := mc.CheckUsersInCache([]string{"user1", "user3"})
	if len(notFoundUsers) != 1 {
		t.Errorf("expected notFoundUsers length to be 1, got %d", len(notFoundUsers))
	}
}
