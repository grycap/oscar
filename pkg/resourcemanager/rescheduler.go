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
	"log"
	"os"
	"strconv"
	"time"

	"github.com/grycap/oscar/v2/pkg/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Custom logger
var reSchedulerLogger = log.New(os.Stdout, "[RE-SCHEDULER] ", log.Flags())

type reScheduleInfo struct {
	service *types.Service
	jobName string
	event   string
}

// StartReScheduler starts the ReScheduler loop to check if there are pending pods exceeding the cfg.ReSchedulerThreshold every cfg.ReSchedulerInterval
func StartReScheduler(cfg *types.Config, back types.ServerlessBackend, kubeClientset kubernetes.Interface) {
	for {
		// Get ReSchedulable pods
		pods, err := getReSchedulablePods(kubeClientset, cfg.ServicesNamespace)
		if err != nil {
			reSchedulerLogger.Println(err.Error())
			continue
		}

		// Get all reScheduleInfo elements
		reScheduleInfos := getReScheduleInfos(pods, back)

		// Delegate jobs
		for _, rsi := range reScheduleInfos {
			err := DelegateJob(rsi.service, rsi.event)
			if err != nil {
				reSchedulerLogger.Println(err.Error())
			} else {
				// Delete successfully reScheduled job from the cluster
				// Create DeleteOptions and configure PropagationPolicy for deleting associated pods in background
				background := metav1.DeletePropagationBackground
				delOpts := metav1.DeleteOptions{
					PropagationPolicy: &background,
				}
				err := kubeClientset.BatchV1().Jobs(cfg.ServicesNamespace).Delete(context.TODO(), rsi.jobName, delOpts)
				if err != nil {
					reSchedulerLogger.Printf("error deleting job \"%s\": %v", rsi.jobName, err)
				}
			}
		}

		time.Sleep(time.Duration(cfg.ReSchedulerInterval) * time.Second)
	}
}

func getReSchedulablePods(kubeClientset kubernetes.Interface, namespace string) ([]v1.Pod, error) {
	reSchedulablePods := []v1.Pod{}

	// List all schedulable jobs' pods (pending)
	listOpts := metav1.ListOptions{
		LabelSelector: types.ReSchedulerLabelKey,
		FieldSelector: fmt.Sprintf("status.phase=%s", v1.PodPending),
	}
	pods, err := kubeClientset.CoreV1().Pods(namespace).List(context.TODO(), listOpts)
	if err != nil {
		return reSchedulablePods, fmt.Errorf("error getting pod list: %v", err)
	}

	for _, pod := range pods.Items {
		// Check that pod has the ServiceLabel
		if _, ok := pod.Labels[types.ServiceLabel]; ok {
			now := time.Now()
			pendingTime := now.Sub(pod.CreationTimestamp.Time).Seconds()
			threshold, err := strconv.Atoi(pod.Labels[types.ReSchedulerLabelKey])
			if err != nil {
				reSchedulerLogger.Printf("unable to parse rescheduler threshold from pod %s. Error: %v\n", pod.Name, err)
				continue
			}
			// Check if threshold is exceeded
			if int(pendingTime) > threshold {
				reSchedulablePods = append(reSchedulablePods, pod)
			}
		}
	}

	return reSchedulablePods, nil
}

func getReScheduleInfos(pods []v1.Pod, back types.ServerlessBackend) []reScheduleInfo {
	rsi := []reScheduleInfo{}

	// Map to store services' pointers
	svcPtrs := map[string]*types.Service{}

	for _, pod := range pods {
		serviceName := pod.Labels[types.ServiceLabel]

		// Check if service is already in svcPtrs
		if _, ok := svcPtrs[serviceName]; !ok {
			var err error
			svcPtrs[serviceName], err = back.ReadService(serviceName)
			if err != nil {
				reSchedulerLogger.Printf("error getting service: %v\n", err)
			}
		}

		// Check if pod has the "job-name" label
		if jobName, ok := pod.Labels["job-name"]; ok {
			rsi = append(rsi, reScheduleInfo{
				service: svcPtrs[serviceName],
				event:   getEvent(pod.Spec),
				jobName: jobName,
			})
		}

	}

	return rsi
}

func getEvent(podSpec v1.PodSpec) string {
	for _, c := range podSpec.Containers {
		if c.Name == types.ContainerName {
			for _, envVar := range c.Env {
				if envVar.Name == types.EventVariable {
					return envVar.Value
				}
			}
		}
	}

	return ""
}
