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
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/pkg/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// MakeJobsInfoHandler makes a handler for listing all existing jobs from a service and show their JobInfo
func MakeJobsInfoHandler(kubeClientset *kubernetes.Clientset, namespace string) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobsInfo := make(map[string]*types.JobInfo)

		// Get serviceName
		serviceName := c.Param("serviceName")

		// List jobs
		listOpts := metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", types.ServiceLabel, serviceName),
		}

		jobs, err := kubeClientset.BatchV1().Jobs(namespace).List(context.TODO(), listOpts)
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
		pods, err := kubeClientset.CoreV1().Pods(namespace).List(context.TODO(), listOpts)
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

// MakeDeleteJobsHandler makes a handler for deleting all jobs created by the provided service.
// If 'all' querystring is set to 'true' pending, running and failed jobs will also be deleted
func MakeDeleteJobsHandler(kubeClientset *kubernetes.Clientset, namespace string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get serviceName and jobName
		serviceName := c.Param("serviceName")
		// Get timestamps querystring (default to false)
		all, err := strconv.ParseBool(c.DefaultQuery("all", "false"))
		if err != nil {
			all = false
		}

		// Delete jobs
		listOpts := metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", types.ServiceLabel, serviceName),
		}

		if !all {
			// Only delete completed jobs
			listOpts.FieldSelector = "status.successful!=0"
		}

		err = kubeClientset.BatchV1().Jobs(namespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, listOpts)
		if err != nil {
			// Check if error is caused because the service is not found
			if !errors.IsNotFound(err) || !errors.IsGone(err) {
				c.String(http.StatusInternalServerError, err.Error())
			} else {
				c.Status(http.StatusNotFound)
			}
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// MakeGetLogsHandler makes a handler for getting logs from the 'oscar-container' inside the pod created by the specified job
func MakeGetLogsHandler(kubeClientset *kubernetes.Clientset, namespace string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get serviceName and jobName
		serviceName := c.Param("serviceName")
		jobName := c.Param("jobName")
		// Get timestamps querystring (default to false)
		timestamps, err := strconv.ParseBool(c.DefaultQuery("timestamps", "false"))
		if err != nil {
			timestamps = false
		}

		// Get job's pod (assuming there's only one pod per job)
		listOpts := metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s job-name=%s", types.ServiceLabel, serviceName, jobName),
		}
		pods, err := kubeClientset.CoreV1().Pods("oscar-fn").List(context.TODO(), listOpts)
		if err != nil || len(pods.Items) < 1 {
			// Check if error is caused because the service is not found
			if !errors.IsNotFound(err) || !errors.IsGone(err) {
				c.String(http.StatusInternalServerError, err.Error())
			} else {
				c.Status(http.StatusNotFound)
			}
			return
		}

		// Get logs
		podLogOpts := &v1.PodLogOptions{
			Timestamps: timestamps,
			Container:  types.ContainerName,
		}
		req := kubeClientset.CoreV1().Pods(namespace).GetLogs(pods.Items[0].Name, podLogOpts)
		result := req.Do(context.TODO())

		// Check result status code
		statusCode := new(int)
		if result.StatusCode(statusCode); *statusCode != 200 {
			c.Status(http.StatusNotFound)
			return
		}

		logs, err := result.Raw()
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.String(http.StatusOK, string(logs))
	}
}

// MakeDeleteJobHandler makes a handler for removing a job
func MakeDeleteJobHandler(kubeClientset *kubernetes.Clientset, namespace string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get serviceName and jobName
		serviceName := c.Param("serviceName")
		jobName := c.Param("jobName")

		// Get job in order to check if it is associated with the provided serviceName
		job, err := kubeClientset.BatchV1().Jobs(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
		if err != nil {
			// Check if error is caused because the service is not found
			if !errors.IsNotFound(err) || !errors.IsGone(err) {
				c.String(http.StatusInternalServerError, err.Error())
			} else {
				c.Status(http.StatusNotFound)
			}
			return
		}

		// Return StatusNotFound if job exists but is not associated with the provided serviceName
		if job.Labels[types.ServiceLabel] != serviceName {
			c.Status(http.StatusNotFound)
			return
		}

		// Delete the job
		err = kubeClientset.BatchV1().Jobs(namespace).Delete(context.TODO(), jobName, metav1.DeleteOptions{})
		if err != nil {
			// Check if error is caused because the service is not found
			if !errors.IsNotFound(err) || !errors.IsGone(err) {
				c.String(http.StatusInternalServerError, err.Error())
			} else {
				c.Status(http.StatusNotFound)
			}
			return
		}

		c.Status(http.StatusNoContent)
	}
}
