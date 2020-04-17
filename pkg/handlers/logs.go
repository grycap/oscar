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
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/pkg/types"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// MakeJobsInfoHandler list all existing jobs from a service and show their JobInfo
func MakeJobsInfoHandler(kubeClientset *kubernetes.Clientset, namespace string) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobsInfo := make(map[string]*types.JobInfo)

		// Get serviceName
		serviceName := c.Param("serviceName")

		// List jobs
		listOpt := metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", types.ServiceLabel, serviceName),
		}

		jobs, err := kubeClientset.BatchV1().Jobs(namespace).List(context.TODO(), listOpt)
		if err != nil {
			// Check if error is caused because the service is not found
			if errors.IsNotFound(err) || errors.IsGone(err) {
				c.Status(http.StatusNotFound)
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			return
		}

		// Populate jobsInfo with keys (job names) and creation time
		for _, job := range jobs.Items {
			if job.Status.StartTime != nil {
				jobsInfo[job.Name] = &types.JobInfo{
					CreationTime: *job.Status.StartTime,
				}
			}
		}

		// List jobs' pods
		pods, err := kubeClientset.CoreV1().Pods(namespace).List(context.TODO(), listOpt)
		if err != nil {
			// Check if error is caused because the service is not found
			if errors.IsNotFound(err) || errors.IsGone(err) {
				c.Status(http.StatusNotFound)
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			return
		}

		// Populate jobsInfo with status, start and finish times (from pods)
		for _, pod := range pods.Items {
			if jobName, ok := pod.Labels["job-name"]; ok {
				jobsInfo[jobName].Status = string(pod.Status.Phase)
				// Loop through job.Status.ContainerStatuses to find oscar-container
				for _, contStatus := range pod.Status.ContainerStatuses {
					if contStatus.Name == types.ContainerName {
						if contStatus.State.Running != nil {
							jobsInfo[jobName].StartTime = contStatus.State.Running.StartedAt
						} else if contStatus.State.Terminated != nil {
							jobsInfo[jobName].StartTime = contStatus.State.Terminated.StartedAt
							jobsInfo[jobName].StartTime = contStatus.State.Terminated.FinishedAt
						}
					}
				}
			}
		}

		c.JSON(http.StatusOK, jobsInfo)
	}
}

// MakeDeleteJobsHandler
// TODO: delete all COMPLETED jobs for the provided service name
func MakeDeleteJobsHandler(kubeClientset *kubernetes.Clientset, namespace string) gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}

// MakeGetLogsHandler
// TODO: get logs for the 'oscar-container' from the specified job's pod
func MakeGetLogsHandler(kubeClientset *kubernetes.Clientset, namespace string) gin.HandlerFunc {
	return func(c *gin.Context) {
		//

	}
}

// MakeDeleteJobHandler
// TODO: remove the specified Job
func MakeDeleteJobHandler(kubeClientset *kubernetes.Clientset, namespace string) gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}
