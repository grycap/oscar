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
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/apache/incubator-yunikorn-core/pkg/common/configs"
	"github.com/grycap/oscar/v2/pkg/types"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// readYunikornConfig Read the Yunikorn's config
func readYunikornConfig(cfg *types.Config, kubeClientset *kubernetes.Clientset) (*configs.SchedulerConfig, error) {
	cm, err := kubeClientset.CoreV1().ConfigMaps(cfg.YunikornNamespace).Get(context.TODO(), cfg.YunikornConfigMap, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("cannot read the YuniKorn's \"%s\" configMap in namespace \"%s\"", cfg.YunikornConfigMap, cfg.YunikornNamespace)
	}
	schedulerConfig := &configs.SchedulerConfig{}

	// Unmarshal the FDL stored in the configMap
	if err = yaml.Unmarshal([]byte(cm.Data[cfg.YunikornConfigFileName]), schedulerConfig); err != nil {
		return nil, fmt.Errorf("the YuniKorn's configuration file \"%s\" cannot be read in configMap \"%s\"", cfg.YunikornConfigFileName, cfg.YunikornConfigMap)
	}

	return schedulerConfig, nil
}

// updateYunikornConfig updates the Yunikorn's configuration file in its configMap
func updateYunikornConfig(cfg *types.Config, kubeClientset *kubernetes.Clientset, schedulerConfig *configs.SchedulerConfig) error {
	// Get the old configMap
	cm, err := kubeClientset.CoreV1().ConfigMaps(cfg.YunikornNamespace).Get(context.TODO(), cfg.YunikornConfigMap, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("cannot read the YuniKorn's \"%s\" configMap in namespace \"%s\"", cfg.YunikornConfigMap, cfg.YunikornNamespace)
	}

	// Marshal the new schedulerConfig
	bytes, err := yaml.Marshal(schedulerConfig)
	if err != nil {
		return errors.New("unable to encode YuniKorn's configuration in YAML")
	}

	// Set the new config file
	cm.Data[cfg.YunikornConfigFileName] = string(bytes)

	// Update the configMap
	_, err = kubeClientset.CoreV1().ConfigMaps(cfg.YunikornNamespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error updating YuniKorn's configMap: %v", err)
	}

	return nil
}

// AddYunikornQueue Add (or update) a service's queue to Yunikorn's config
func AddYunikornQueue(cfg *types.Config, kubeClientset *kubernetes.Clientset, svc *types.Service) error {
	// Read the config
	yConfig, err := readYunikornConfig(cfg, kubeClientset)
	if err != nil {
		return err
	}

	// Get the pointer of the Oscar queue
	oQueue := getOscarQueue(yConfig)

	// Parse the total resources
	var memory string
	var cpu string
	if svc.TotalMemory != "" {
		intMemory, err := convertMemory(svc.TotalMemory)
		if err != nil {
			log.Println(err.Error())
		} else {
			memory = strconv.Itoa(intMemory)
		}
	}
	if svc.TotalCPU != "" {
		intCPU, err := convertCPU(svc.TotalCPU)
		if err != nil {
			log.Println(err.Error())
		} else {
			cpu = strconv.Itoa(intCPU)
		}
	}

	// Create the Resources struct
	maxResources := make(map[string]string)
	if memory != "" {
		maxResources["memory"] = memory
	}
	if cpu != "" {
		maxResources["vcore"] = cpu
	}

	resources := configs.Resources{
		Max: maxResources,
	}

	// Update the service's queue if already exists
	found := false
	for i, q := range oQueue.Queues {
		if q.Name == svc.Name {
			oQueue.Queues[i].Resources = resources
			found = true
			break
		}
	}

	// Create the service's queue if doesn't exists
	if !found {
		oQueue.Queues = append(oQueue.Queues, configs.QueueConfig{
			Name:      svc.Name,
			Resources: resources,
		})
	}

	// Update the configMap
	if err := updateYunikornConfig(cfg, kubeClientset, yConfig); err != nil {
		return err
	}

	return nil
}

// DeleteYunikornQueue delete a service's queue in Yunikorn's config
func DeleteYunikornQueue(cfg *types.Config, kubeClientset *kubernetes.Clientset, svc *types.Service) error {
	// Read the config
	yConfig, err := readYunikornConfig(cfg, kubeClientset)
	if err != nil {
		return err
	}

	// Get the pointer of the Oscar queue
	oQueue := getOscarQueue(yConfig)

	// Search the service's queue
	index := -1
	for i, q := range oQueue.Queues {
		if q.Name == svc.Name {
			index = i
			break
		}
	}

	// Remove the service's queue
	if index != -1 {
		oQueue.Queues = append(oQueue.Queues[:index], oQueue.Queues[index+1:]...)
	}

	// Update the configMap
	if err := updateYunikornConfig(cfg, kubeClientset, yConfig); err != nil {
		return err
	}

	return nil
}

// convertMemory parse the memory k8s resource to integer (MB)
// https://yunikorn.apache.org/docs/user_guide/resource_quota_management/#converting-kubernetes-resources-and-quotas
func convertMemory(stringMemory string) (int, error) {
	quantity, err := resource.ParseQuantity(stringMemory)
	if err != nil {
		return -1, fmt.Errorf("unable to parse total_memory: %v", err)
	}
	bytes := quantity.Value()
	return int(bytes) / 1000 / 1000, nil
}

// convertCPU parse the CPU k8s resource to integer (Milli cpu)
// https://yunikorn.apache.org/docs/user_guide/resource_quota_management/#converting-kubernetes-resources-and-quotas
func convertCPU(stringCPU string) (int, error) {
	quantity, err := resource.ParseQuantity(stringCPU)
	if err != nil {
		return -1, fmt.Errorf("unable to parse total_cpu: %v", err)
	}
	millicpu := quantity.MilliValue()
	return int(millicpu), nil
}

// getOscarQueue returns a pointer to the OSCAR's Yunikorn queue (configs.QueueConfig)
// If the Queue doesn't exists, create a new one in the SchedulerConfig
// (the existance of the default partition and the root queue is assumed)
func getOscarQueue(schedulerConfig *configs.SchedulerConfig) *configs.QueueConfig {
	// First get a pointer to the root queue
	root := &configs.QueueConfig{}
	for i, partition := range schedulerConfig.Partitions {
		if partition.Name == types.YunikornDefaultPartition {
			for j, queue := range partition.Queues {
				if queue.Name == types.YunikornRootQueue {
					root = &schedulerConfig.Partitions[i].Queues[j]
					break
				}
			}
		}
	}

	// Now search the OSCAR queue and return it if exists create it if doesn't exists
	for i, queue := range root.Queues {
		if queue.Name == types.YunikornOscarQueue {
			return &root.Queues[i]
		}
	}

	// Create it if doesn't exists and return it
	root.Queues = append(root.Queues, configs.QueueConfig{Name: types.YunikornOscarQueue})
	return &root.Queues[len(root.Queues)-1]
}
