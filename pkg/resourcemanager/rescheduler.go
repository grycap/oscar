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
	"strings"
	"time"

	"github.com/grycap/oscar/v4/pkg/types"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Custom logger
var reSchedulerLogger = log.New(os.Stdout, "[RE-SCHEDULER] ", log.Flags())
var delegateJobFunc = DelegateJob

type reScheduleInfo struct {
	service   *types.Service
	jobName   string
	event     string
	namespace string
}

// StartReScheduler starts the ReScheduler loop to check if there are pending and suspended pods exceeding the cfg.ReSchedulerThreshold every cfg.ReSchedulerInterval
func StartReScheduler(cfg *types.Config, back types.ServerlessBackend, kubeClientset kubernetes.Interface) {
	for {
		// Get ReSchedulable pods and jobs
		jobs, pods, err := getReSchedulablePods(kubeClientset, cfg.ServicesNamespace)
		if err != nil {
			reSchedulerLogger.Println(err.Error())
			continue
		}

		// Get all reScheduleInfo elements
		reScheduleInfos := getReScheduleInfos(pods, back)
		infoJobs := getReScheduleJobs(jobs, back)

		// delegate pods (Suspended)
		for _, rsi := range infoJobs {

			if rsi.service == nil {
				reSchedulerLogger.Printf("skip reschedule for job %q: service not found", rsi.jobName)
				continue
			}

			err := delegateJobFunc(rsi.service, rsi.event, rsi.jobName, "", reSchedulerLogger, cfg, kubeClientset)
			if err != nil {
				reSchedulerLogger.Println(err.Error())
			} else {
				// Delete successfully reScheduled pods from the cluster
				// Create DeleteOptions and configure PropagationPolicy for deleting associated pods in background
				background := metav1.DeletePropagationBackground
				delOpts := metav1.DeleteOptions{
					PropagationPolicy: &background,
				}
				err := kubeClientset.BatchV1().Jobs(rsi.namespace).Delete(context.TODO(), rsi.jobName, delOpts)
				if err != nil {
					reSchedulerLogger.Printf("error deleting job \"%s\" in namespace \"%s\": %v", rsi.jobName, rsi.namespace, err)
				}
				//fmt.Println("Delete successfully reScheduled pod (Suspended) from the cluster ")
			}

		}

		// Delegate jobs (Pending)
		for _, rsi := range reScheduleInfos {
			if rsi.service == nil {
				reSchedulerLogger.Printf("skip reschedule for job %q: service not found", rsi.jobName)
				continue
			}

			err := delegateJobFunc(rsi.service, rsi.event, rsi.jobName, "", reSchedulerLogger, cfg, kubeClientset)
			if err != nil {
				reSchedulerLogger.Println(err.Error())
			} else {
				// Delete successfully reScheduled job from the cluster
				// Create DeleteOptions and configure PropagationPolicy for deleting associated pods in background
				background := metav1.DeletePropagationBackground
				delOpts := metav1.DeleteOptions{
					PropagationPolicy: &background,
				}
				err := kubeClientset.BatchV1().Jobs(rsi.namespace).Delete(context.TODO(), rsi.jobName, delOpts)
				if err != nil {
					reSchedulerLogger.Printf("error deleting job \"%s\" in namespace \"%s\": %v", rsi.jobName, rsi.namespace, err)
				}
				//fmt.Println("Delete successfully reScheduled job (Pending) from the cluster ")
			}
		}

		time.Sleep(time.Duration(cfg.ReSchedulerInterval) * time.Second)
	}
}

func getReSchedulablePods(kubeClientset kubernetes.Interface, namespace string) ([]batchv1.Job, []v1.Pod, error) {
	reSchedulablePods := []v1.Pod{}
	reSchedulableJobs := []batchv1.Job{}
	targetNamespaces, err := getReschedulerNamespaces(kubeClientset, namespace)
	if err != nil {
		reSchedulerLogger.Printf("error getting namespaces for rescheduler: %v\n", err)
		targetNamespaces = []string{namespace}
	}

	for _, ns := range targetNamespaces {

		listOpts := metav1.ListOptions{
			LabelSelector: types.ReSchedulerLabelKey,
			FieldSelector: fmt.Sprintf("status.phase=%s", v1.PodPending),
		}

		listOpts1 := metav1.ListOptions{
			LabelSelector: types.ReSchedulerLabelKey,
		}
		// suspended jobs
		jobs, err := kubeClientset.BatchV1().Jobs(ns).List(context.TODO(), listOpts1)
		if err != nil {
			reSchedulerLogger.Printf("error getting job list in namespace %s: %v\n", ns, err)
			continue
		}
		for _, job := range jobs.Items {
			if job.Spec.Suspend != nil && *job.Spec.Suspend == true {
				now_job := time.Now()
				suspendedTime := now_job.Sub(job.CreationTimestamp.Time).Seconds()
				thresholdJob, err := strconv.Atoi(job.Labels[types.ReSchedulerLabelKey])
				if err != nil {
					reSchedulerLogger.Printf("unable to parse rescheduler threshold from pod %s. Error: %v\n", job.Name, err)
					continue
				}
				// Check if threshold is exceeded
				if int(suspendedTime) > thresholdJob {
					reSchedulableJobs = append(reSchedulableJobs, job)
				}
			}

		}
		// pending pods
		pods, err := kubeClientset.CoreV1().Pods(ns).List(context.TODO(), listOpts)
		if err != nil {
			reSchedulerLogger.Printf("error getting pod list in namespace %s: %v\n", ns, err)
			continue
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
	}

	return reSchedulableJobs, reSchedulablePods, nil
}

func getReschedulerNamespaces(kubeClientset kubernetes.Interface, servicesNamespace string) ([]string, error) {
	if servicesNamespace == "" {
		servicesNamespace = "oscar-svc"
	}

	// Collect namespaces that match the services namespace prefix (e.g. oscar-svc-<owner-hash>)
	nsList, err := kubeClientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := []string{}
	for _, ns := range nsList.Items {
		if strings.HasPrefix(ns.Name, servicesNamespace) {
			result = append(result, ns.Name)
		}
	}

	if len(result) == 0 {
		result = append(result, servicesNamespace)
	}

	return result, nil
}

// Get information from the pods
func getReScheduleInfos(pods []v1.Pod, back types.ServerlessBackend) []reScheduleInfo {
	rsi := []reScheduleInfo{}

	// Map to store services' pointers
	svcPtrs := map[string]*types.Service{}

	for _, pod := range pods {
		serviceName := pod.Labels[types.ServiceLabel]
		serviceKey := fmt.Sprintf("%s/%s", pod.Namespace, serviceName)

		// Check if service is already in svcPtrs
		if _, ok := svcPtrs[serviceKey]; !ok {
			var err error
			svcPtrs[serviceKey], err = back.ReadService(pod.Namespace, serviceName)
			if err != nil {
				reSchedulerLogger.Printf("error getting service: %v\n", err)
				svcPtrs[serviceKey] = nil
			}
		}

		// Check if pod has the "job-name" label
		if jobName, ok := pod.Labels["job-name"]; ok {
			if svcPtrs[serviceKey] == nil {
				continue
			}
			rsi = append(rsi, reScheduleInfo{
				service:   svcPtrs[serviceKey],
				event:     getEvent(pod.Spec),
				jobName:   jobName,
				namespace: pod.Namespace,
			})
		}

	}

	return rsi
}

// Get information from the jobs
func getReScheduleJobs(jobs []batchv1.Job, back types.ServerlessBackend) []reScheduleInfo {
	rsi := []reScheduleInfo{}

	svcPtrs := map[string]*types.Service{}

	for _, job := range jobs {
		// get the service name from the job labels
		serviceName := job.Spec.Template.Labels[types.ServiceLabel]
		if serviceName == "" {
			continue
		}

		serviceKey := fmt.Sprintf("%s/%s", job.Namespace, serviceName)

		if _, ok := svcPtrs[serviceKey]; !ok {
			var err error
			svcPtrs[serviceKey], err = back.ReadService(job.Namespace, serviceName)
			if err != nil {
				reSchedulerLogger.Printf("error getting service %s: %v\n", serviceName, err)
				svcPtrs[serviceKey] = nil
			}
		}

		if svcPtrs[serviceKey] != nil {
			rsi = append(rsi, reScheduleInfo{
				service:   svcPtrs[serviceKey],
				event:     getEvent(job.Spec.Template.Spec),
				jobName:   job.Name,
				namespace: job.Namespace,
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
