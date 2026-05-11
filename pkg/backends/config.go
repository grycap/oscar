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

package backends

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateOSCARCMConfiguration(back kubernetes.Interface, configMap *v1.ConfigMap, namespace string) error {
	_, err := back.CoreV1().ConfigMaps(namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
	return err
}

func GetOSCARCMConfiguration(back kubernetes.Interface, nameConfigMap string, namespace string) (*v1.ConfigMap, error) {
	cm, err := back.CoreV1().ConfigMaps(namespace).Get(context.TODO(), nameConfigMap, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return cm, nil
}

func UpdateOSCARCMConfiguration(back kubernetes.Interface, configMap *v1.ConfigMap, namespace string) error {
	_, err := back.CoreV1().ConfigMaps(namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
	return err
}
