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

package utils

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateSecret(secretName string, namespace string, secretData map[string]string, kubeClientset kubernetes.Interface) error {
	secret := getPodSecretSpec(secretName, secretData, namespace)
	_, err := kubeClientset.CoreV1().Secrets(namespace).Create(context.TODO(), &secret, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func UpdateSecretData(secretName string, namespace string, secretData map[string]string, kubeClientset kubernetes.Interface) error {
	secret := getPodSecretSpec(secretName, secretData, namespace)
	_, err := kubeClientset.CoreV1().Secrets(namespace).Update(context.TODO(), &secret, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func DeleteSecret(secretName string, namespace string, kubeClientset kubernetes.Interface) error {
	if SecretExists(secretName, namespace, kubeClientset) {
		errSecret := kubeClientset.CoreV1().Secrets(namespace).Delete(context.TODO(), secretName, metav1.DeleteOptions{})
		if errSecret != nil {
			return errSecret
		}
	}
	return nil
}

func SecretExists(name string, namespace string, kubeClientset kubernetes.Interface) bool {
	_, err := kubeClientset.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	return !errors.IsNotFound(err)
}

func GetExistingSecret(name string, namespace string, kubeClientset kubernetes.Interface) (*v1.Secret, error) {
	secret, err := kubeClientset.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return secret, nil
}

func getPodSecretSpec(secretName string, secretData map[string]string, namespace string) v1.Secret {
	inmutable := false
	return v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Immutable:  &inmutable,
		StringData: secretData,
	}
}
