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
	"testing"

	"github.com/apache/yunikorn-core/pkg/common/configs"
	"github.com/grycap/oscar/v3/pkg/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func getFakeClientset() (*types.Config, *fake.Clientset) {
	cfg := &types.Config{
		YunikornNamespace:      "default",
		YunikornConfigMap:      "yunikorn-config",
		YunikornConfigFileName: "yunikorn.yaml",
	}

	cfgmap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.YunikornConfigMap,
			Namespace: cfg.YunikornNamespace,
		},
		Data: map[string]string{
			cfg.YunikornConfigFileName: `
partitions:
  - name: default
	queues:
	  - name: root
		queues:
		  - name: oscar
			queues:
			  - name: test-service
`,
		},
	}
	return cfg, fake.NewSimpleClientset(cfgmap)
}

func TestReadYunikornConfig(t *testing.T) {
	cfg, clientset := getFakeClientset()

	schedulerConfig, err := readYunikornConfig(cfg, clientset)
	if err != nil {
		t.Errorf("Error Reading Yunikorn config: %v", err)
	}

	if schedulerConfig.Partitions[0].Name != "default" {
		t.Errorf("Error Reading Yunikorn config. SchedulerConfig is nil")
	}
}

func TestUpdateYunikornConfig(t *testing.T) {
	cfg, clientset := getFakeClientset()

	schedulerConfig := &configs.SchedulerConfig{
		Partitions: []configs.PartitionConfig{
			{
				Name: "default",
				Queues: []configs.QueueConfig{
					{
						Name: "root",
						Queues: []configs.QueueConfig{
							{
								Name: "oscar",
							},
						},
					},
				},
			},
		},
	}

	err := updateYunikornConfig(cfg, clientset, schedulerConfig)
	if err != nil {
		t.Errorf("Error Updating Yunikorn config: %v", err)
	}
}

func TestAddYunikornQueue(t *testing.T) {
	cfg, clientset := getFakeClientset()

	svc := &types.Service{
		Name:        "test-service",
		TotalMemory: "4Gi",
		TotalCPU:    "2",
	}

	err := AddYunikornQueue(cfg, clientset, svc)
	if err != nil {
		t.Errorf("Error Adding Yunikorn config: %v", err)
	}
}

func TestDeleteYunikornQueue(t *testing.T) {
	cfg, clientset := getFakeClientset()

	svc := &types.Service{
		Name: "test-service",
	}

	err := DeleteYunikornQueue(cfg, clientset, svc)
	if err != nil {
		t.Errorf("Error Deleting Yunikorn config: %v", err)
	}
}
