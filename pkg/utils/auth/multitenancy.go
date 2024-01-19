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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const ServicesNamespace = "oscar-svc"

type multitenancyConfig struct {
	kubeClientset *kubernetes.Clientset
	owner_uid     string
	usersCache    []string
}

func NewMultitenancyConfig(kubeClientset *kubernetes.Clientset, uid string) *multitenancyConfig {
	return &multitenancyConfig{
		kubeClientset: kubeClientset,
		owner_uid:     uid,
		usersCache:    []string{uid},
	}
}

// TODO periodically check that the users stored on cache still exist on MinIO (cronjob)
func (mc *multitenancyConfig) UpdateCacheStatus() {
	// 1. List users on MinIO
	// 2. List secrets
	// 3. Compare both lists and delete from secrets the missmatchs
	// 4. updateCache
}

func (mc *multitenancyConfig) UpdateCache(uid string) {
	mc.usersCache = append(mc.usersCache, uid)
}

func (mc *multitenancyConfig) ClearCache() {
	mc.usersCache = nil
}

func (mc *multitenancyConfig) UserExists(uid string) bool {
	for _, id := range mc.usersCache {
		if id == uid {
			return true
		}
	}
	return false
}

func (mc *multitenancyConfig) CreateSecretForOIDC(uid string, sk string) error {

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uid,
			Namespace: ServicesNamespace,
		},
		Immutable: new(bool),
		StringData: map[string]string{
			"oidc_uid":  uid,
			"accessKey": uid,
			"secretKey": sk,
		},
	}

	_, err := mc.kubeClientset.CoreV1().Secrets(ServicesNamespace).Create(context.TODO(), secret, metav1.CreateOptions{})

	if err != nil {
		return err
	}

	mc.UpdateCache(uid)

	return nil
}
