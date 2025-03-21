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
	"fmt"

	"github.com/grycap/oscar/v3/pkg/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreatePodSecrets(service *types.Service, cfg *types.Config, kubeClientset kubernetes.Interface) error {
	secret := getPodSecretSpec(service, cfg)
	_, err := kubeClientset.CoreV1().Secrets(cfg.ServicesNamespace).Create(context.TODO(), &secret, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	for key, value := range service.Environment.Secrets {
		fmt.Println(value)
		service.Environment.Secrets[key] = ""
	}

	return nil
}

func ReadPodSecrets(service types.Service, cfg types.Config) error {
	return nil

}

func UpdatePodSecrets(service *types.Service, cfg *types.Config, kubeClientset kubernetes.Interface) error {
	if existsSecret(service.Name, cfg, kubeClientset) && service.Environment.Secrets != nil {
		secret := getPodSecretSpec(service, cfg)
		_, err := kubeClientset.CoreV1().Secrets(cfg.ServicesNamespace).Update(context.TODO(), &secret, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		for key, value := range service.Environment.Secrets {
			fmt.Println(value)
			service.Environment.Secrets[key] = ""
		}
	} else if existsSecret(service.Name, cfg, kubeClientset) && service.Environment.Secrets == nil {
		DeletePodSecrets(service, cfg, kubeClientset)
	} else if !existsSecret(service.Name, cfg, kubeClientset) && service.Environment.Secrets != nil {
		CreatePodSecrets(service, cfg, kubeClientset)
	}
	return nil

}

func DeletePodSecrets(service *types.Service, cfg *types.Config, kubeClientset kubernetes.Interface) error {
	if existsSecret(service.Name, cfg, kubeClientset) {
		errSecret := kubeClientset.CoreV1().Secrets(cfg.ServicesNamespace).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})
		if errSecret != nil {
			return errSecret
		}
	}
	return nil
}

func existsSecret(name string, cfg *types.Config, kubeClientset kubernetes.Interface) bool {
	_, err := kubeClientset.CoreV1().Secrets(cfg.ServicesNamespace).Get(context.TODO(), name, metav1.GetOptions{})
	return !errors.IsNotFound(err)
}

func getPodSecretSpec(service *types.Service, cfg *types.Config) v1.Secret {
	inmutable := false
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: cfg.ServicesNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		Immutable:  &inmutable,
		StringData: service.Environment.Secrets,
		Type:       "Opaque",
	}
	return *secret
}
