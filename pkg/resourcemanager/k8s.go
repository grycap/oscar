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

package resourcemanager

import (
	"context"
	"fmt"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type nodeResources struct {
	// memory in bytes, as returned by quantity.Value()
	memory int64
	// cpu in MilliValue, as returned by quantity.MilliValue()
	cpu int64
}

// KubeResourceManager struct to represent the Kubernetes resource manager
type KubeResourceManager struct {
	resources     []nodeResources
	kubeClientset kubernetes.Interface
	mutex         sync.Mutex
}

// UpdateResources update the available resources in the cluster
func (krm *KubeResourceManager) UpdateResources() error {
	// List all (working) nodes
	nodes, err := krm.kubeClientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting node list: %v", err)
	}

	// Get list all Running pods
	pods, err := krm.kubeClientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{FieldSelector: "status.phase!=Succeeded,status.phase!=Failed"})
	if err != nil {
		return fmt.Errorf("error getting pod list: %v", err)
	}

	// Define new nodeResources slice
	res := []nodeResources{}

	for _, node := range nodes.Items {
		// Only count Schedulable and Ready nodes
		if !node.Spec.Unschedulable && isNodeReady(node) {
			nodeCPU, nodeMemory := getNodeAvailableResources(node, pods)
			nodeRes := nodeResources{memory: nodeMemory, cpu: nodeCPU}
			res = append(res, nodeRes)
		}
	}

	// Ensure mutual exclusion
	krm.mutex.Lock()
	krm.resources = res
	krm.mutex.Unlock()

	return nil
}

// IsSchedulable check if a Service's v1.ResourceRequirements can be scheduled in the cluster
func (krm *KubeResourceManager) IsSchedulable(resources v1.ResourceRequirements) bool {
	serviceMemory := resources.Limits.Memory().Value()
	serviceCPU := resources.Limits.Cpu().MilliValue()

	// Ensure mutual exclusion
	krm.mutex.Lock()
	defer krm.mutex.Unlock()

	// Check if the job can be scheduled at least in one node
	for _, nodeRes := range krm.resources {
		if serviceMemory < nodeRes.memory && serviceCPU < nodeRes.cpu {
			return true
		}
	}

	return false
}

func getNodeAvailableResources(node v1.Node, pods *v1.PodList) (cpu int64, memory int64) {
	// Get allocatable resources from node status
	memory = node.Status.Allocatable.Memory().Value()
	cpu = node.Status.Allocatable.Cpu().MilliValue()

	// Filter podList by nodename and subtract used resources
	for _, pod := range pods.Items {
		if pod.Spec.NodeName == node.Name {
			for _, container := range pod.Spec.Containers {
				memory -= container.Resources.Requests.Memory().Value()
				cpu -= container.Resources.Requests.Cpu().MilliValue()
			}
		}
	}

	return
}

func isNodeReady(node v1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}
