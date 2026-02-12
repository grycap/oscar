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

package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func extractRefreshTokenSecret(service *types.Service) string {
	if service == nil || service.Environment.Secrets == nil {
		return ""
	}

	refreshToken, ok := service.Environment.Secrets[types.RefreshTokenSecretKey]
	if !ok {
		return ""
	}

	delete(service.Environment.Secrets, types.RefreshTokenSecretKey)
	return strings.TrimSpace(refreshToken)
}

func createRefreshTokenSecret(service *types.Service, namespace string, refreshToken string, kubeClientset kubernetes.Interface) error {
	if refreshToken == "" {
		return nil
	}

	secretName := utils.RefreshTokenSecretName(service.Name)
	if utils.SecretExists(secretName, namespace, kubeClientset) {
		return fmt.Errorf("refresh-token secret already exists")
	}

	return utils.CreateSecret(secretName, namespace, map[string]string{
		types.RefreshTokenSecretKey: refreshToken,
	}, kubeClientset)
}

func upsertRefreshTokenSecret(service *types.Service, namespace string, refreshToken string, kubeClientset kubernetes.Interface) error {
	if refreshToken == "" {
		return nil
	}

	secretName := utils.RefreshTokenSecretName(service.Name)
	if utils.SecretExists(secretName, namespace, kubeClientset) {
		return utils.UpdateSecretData(secretName, namespace, map[string]string{
			types.RefreshTokenSecretKey: refreshToken,
		}, kubeClientset)
	}

	return utils.CreateSecret(secretName, namespace, map[string]string{
		types.RefreshTokenSecretKey: refreshToken,
	}, kubeClientset)
}

func readRefreshTokenSecretValue(serviceName string, namespace string, kubeClientset kubernetes.Interface) (string, error) {
	if serviceName == "" || namespace == "" || kubeClientset == nil {
		return "", fmt.Errorf("missing refresh-token secret context")
	}

	secretName := utils.RefreshTokenSecretName(serviceName)
	secret, err := kubeClientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	tokenBytes, ok := secret.Data[types.RefreshTokenSecretKey]
	if !ok {
		return "", fmt.Errorf("refresh-token secret missing key %q", types.RefreshTokenSecretKey)
	}

	return strings.TrimSpace(string(tokenBytes)), nil
}
