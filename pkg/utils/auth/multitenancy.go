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
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"regexp"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const ServicesNamespace = "oscar-svc"
const ServiceLabelLength = 8

var mcLogger = log.New(os.Stdout, "[OIDC-AUTH] ", log.Flags())

type MultitenancyConfig struct {
	kubeClientset kubernetes.Interface
	owner_uid     string
	usersCache    []string
}

func NewMultitenancyConfig(kubeClientset kubernetes.Interface, uid string) *MultitenancyConfig {
	return &MultitenancyConfig{
		kubeClientset: kubeClientset,
		owner_uid:     uid,
		usersCache:    []string{},
	}
}

// TODO periodically check that the users stored on cache still exist on MinIO (cronjob)
/* func (mc *MultitenancyConfig) UpdateCacheStatus() {
	// 1. List users on MinIO
	// 2. List secrets
	// 3. Compare both lists and delete from secrets the missmatchs
	// 4. updateCache
} */

func (mc *MultitenancyConfig) UpdateCache(uid string) {
	mc.usersCache = append(mc.usersCache, uid)
}

func (mc *MultitenancyConfig) ClearCache() {
	// TODO delete associated secrets
	mc.usersCache = nil
}

// UserExists checks if a MinIO user has been created and stored on cache.
func (mc *MultitenancyConfig) UserExists(uid string) bool {
	if len(mc.usersCache) > 1 {
		// If the cache has users search for the uid
		for _, cacheUID := range mc.usersCache {
			if cacheUID == uid {
				return true
			}
		}
	}
	// If the container has been restarted a user can exist
	// but not be on the cache due to lack of persistence
	secretName := FormatUID(uid)
	secret, err := mc.kubeClientset.CoreV1().Secrets(ServicesNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	if secret != nil {
		mc.UpdateCache(uid)
		return true
	}

	return false
}

func (mc *MultitenancyConfig) CheckUsersInCache(uids []string) []string {
	var notFoundUsers []string
	var found bool

	if len(mc.usersCache) == 0 {
		secrets, _ := mc.kubeClientset.CoreV1().Secrets(ServicesNamespace).List(context.TODO(), metav1.ListOptions{})
		if secrets != nil {
			for _, s := range secrets.Items {
				mc.UpdateCache(s.Name)
			}
		}
	}

	for _, uid := range uids {
		found = false
		for _, cacheUID := range mc.usersCache {
			if uid == cacheUID {
				found = true
				break
			}
		}
		if !found {
			notFoundUsers = append(notFoundUsers, uid)
		}
	}
	return notFoundUsers
}

func (mc *MultitenancyConfig) CreateSecretForOIDC(uid string, sk string) error {

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      FormatUID(uid),
			Namespace: ServicesNamespace,
		},
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

func (mc *MultitenancyConfig) GetUserCredentials(uid string) (string, string, error) {
	secretName := FormatUID((uid))
	secret, err := mc.kubeClientset.CoreV1().Secrets(ServicesNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}

	encodedData := secret.Data
	access_key := string(encodedData["accessKey"])
	secret_key := string(encodedData["secretKey"])

	if access_key != "" && secret_key != "" {
		return access_key, secret_key, nil
	}
	return "", "", fmt.Errorf("error decoding secret data")
}

func GenerateRandomKey(length int) (string, error) {
	key := make([]byte, length)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(key), nil
}

func FormatUID(uid string) string {
	uidr, _ := regexp.Compile("[0-9a-z]+@")
	idx := uidr.FindStringIndex(uid)
	// If the regex is not matched assume it is not an EGI uid
	// and return the original string
	if idx == nil {
		return uid
	}
	return uid[0 : idx[1]-1]
}
