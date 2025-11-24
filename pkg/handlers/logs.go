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
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// TODO Try using cookies to avoid excesive calls to the k8s API //

// MakeJobsInfoHandler godoc
// @Summary List jobs
// @Description List jobs created by a service including their status and timestamps.
// @Tags logs
// @Produce json
// @Param serviceName path string true "Service name"
// @Param page query string false "Continuation token"
// @Success 200 {object} types.JobsResponse
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/logs/{serviceName} [get]
func MakeJobsInfoHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobsInfo := make(map[string]*types.JobInfo)
		// Get serviceName
		serviceName := c.Param("serviceName")
		page := c.DefaultQuery("page", "")
		service, ok := getAuthorizedService(c, back, serviceName)
		if !ok {
			return
		}
		serviceNamespace := resolveServiceNamespace(service, cfg)
		uid, err := auth.GetUIDFromContext(c)
		// List jobs
		var labelSelector string
		if err != nil {
			labelSelector = fmt.Sprintf("%s=%s", types.ServiceLabel, serviceName)
		} else {
			labelSelector = fmt.Sprintf("%s=%s,%s=%s", types.ServiceLabel, serviceName, types.JobOwnerExecutionAnnotation, auth.FormatUID(uid))
		}
		listOpts := metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         int64(cfg.JobListingLimit),
			Continue:      page,
		}

		// List jobs' pods
		pods, err := kubeClientset.CoreV1().Pods(serviceNamespace).List(context.TODO(), listOpts)
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
				jobsInfo[jobName] = &types.JobInfo{
					Status:       string(pod.Status.Phase),
					CreationTime: pod.Status.StartTime,
				}
				// Loop through job.Status.ContainerStatuses to find oscar-container
				for _, contStatus := range pod.Status.ContainerStatuses {
					if contStatus.Name == types.ContainerName {
						if contStatus.State.Running != nil {
							jobsInfo[jobName].StartTime = &(contStatus.State.Running.StartedAt)
						} else if contStatus.State.Terminated != nil {
							jobsInfo[jobName].StartTime = &(contStatus.State.Terminated.StartedAt)
							jobsInfo[jobName].FinishTime = &(contStatus.State.Terminated.FinishedAt)
						}
					}
				}
			}
		}
		jr := types.JobsResponse{
			Jobs:         jobsInfo,
			NextPage:     pods.ListMeta.Continue,
			RemainingJob: pods.ListMeta.RemainingItemCount,
		}

		c.JSON(http.StatusOK, jr)
	}
}

// MakeDeleteJobsHandler godoc
// @Summary Delete jobs
// @Description Delete jobs created by the provided service. If `all` is true pending, running and failed jobs are also removed.
// @Tags logs
// @Param serviceName path string true "Service name"
// @Param all query bool false "Delete running and failed jobs as well"
// @Success 204 {string} string "No Content"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/logs/{serviceName} [delete]
func MakeDeleteJobsHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get serviceName and jobName
		serviceName := c.Param("serviceName")
		service, ok := getAuthorizedService(c, back, serviceName)
		if !ok {
			return
		}
		serviceNamespace := resolveServiceNamespace(service, cfg)

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

		// Create DeleteOptions and configure PropagationPolicy for deleting associated pods in background
		background := metav1.DeletePropagationBackground
		delOpts := metav1.DeleteOptions{
			PropagationPolicy: &background,
		}

		err = kubeClientset.BatchV1().Jobs(serviceNamespace).DeleteCollection(context.TODO(), delOpts, listOpts)
		if err != nil {
			// Check if error is caused because the service is not found
			if !errors.IsNotFound(err) && !errors.IsGone(err) {
				c.String(http.StatusInternalServerError, err.Error())
			} else {
				c.Status(http.StatusNotFound)
			}
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// MakeGetLogsHandler godoc
// @Summary Get job logs
// @Description Stream logs of a specific job execution.
// @Tags logs
// @Produce plain
// @Param serviceName path string true "Service name"
// @Param jobName path string true "Job name"
// @Param timestamps query bool false "Include timestamps"
// @Success 200 {string} string "Logs"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/logs/{serviceName}/{jobName} [get]
func MakeGetLogsHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get serviceName and jobName
		serviceName := c.Param("serviceName")
		service, ok := getAuthorizedService(c, back, serviceName)
		if !ok {
			return
		}
		serviceNamespace := resolveServiceNamespace(service, cfg)
		jobName := c.Param("jobName")
		// Get timestamps querystring (default to false)
		timestamps, err := strconv.ParseBool(c.DefaultQuery("timestamps", "false"))
		if err != nil {
			timestamps = false
		}

		// Get job's pod (assuming there's only one pod per job)
		listOpts := metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s,job-name=%s", types.ServiceLabel, serviceName, jobName),
		}
		pods, err := kubeClientset.CoreV1().Pods(serviceNamespace).List(context.TODO(), listOpts)
		if err != nil || len(pods.Items) < 1 {
			// Check if error is caused because the service is not found
			if !errors.IsNotFound(err) && !errors.IsGone(err) {
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
		req := kubeClientset.CoreV1().Pods(serviceNamespace).GetLogs(pods.Items[0].Name, podLogOpts)
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

// MakeDeleteJobHandler godoc
// @Summary Delete job
// @Description Delete a specific job and its pod.
// @Tags logs
// @Param serviceName path string true "Service name"
// @Param jobName path string true "Job name"
// @Success 204 {string} string "No Content"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/logs/{serviceName}/{jobName} [delete]
func MakeDeleteJobHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get serviceName and jobName
		serviceName := c.Param("serviceName")
		service, ok := getAuthorizedService(c, back, serviceName)
		if !ok {
			return
		}
		serviceNamespace := resolveServiceNamespace(service, cfg)
		jobName := c.Param("jobName")

		// Get job in order to check if it is associated with the provided serviceName
		job, err := kubeClientset.BatchV1().Jobs(serviceNamespace).Get(context.TODO(), jobName, metav1.GetOptions{})
		if err != nil {
			// Check if error is caused because the service is not found
			if !errors.IsNotFound(err) && !errors.IsGone(err) {
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

		// Create DeleteOptions and configure PropagationPolicy for deleting associated pods in background
		background := metav1.DeletePropagationBackground
		delOpts := metav1.DeleteOptions{
			PropagationPolicy: &background,
		}

		// Delete the job
		err = kubeClientset.BatchV1().Jobs(serviceNamespace).Delete(context.TODO(), jobName, delOpts)
		if err != nil {
			// Check if error is caused because the service is not found
			if !errors.IsNotFound(err) && !errors.IsGone(err) {
				c.String(http.StatusInternalServerError, err.Error())
			} else {
				c.Status(http.StatusNotFound)
			}
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func getAuthorizedService(c *gin.Context, back types.ServerlessBackend, serviceName string) (*types.Service, bool) {
	service, err := back.ReadService("", serviceName)
	if err != nil {
		if errors.IsNotFound(err) || errors.IsGone(err) {
			c.Status(http.StatusNotFound)
		} else {
			c.String(http.StatusInternalServerError, err.Error())
		}
		return nil, false
	}

	if !authorizeRequest(c, service) {
		return nil, false
	}

	return service, true
}

func authorizeRequest(c *gin.Context, service *types.Service) bool {
	authHeader := c.GetHeader("Authorization")
	if len(strings.Split(authHeader, "Bearer")) > 1 {
		uid, err := auth.GetUIDFromContext(c)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintln(err))
			return false
		}
		if service.Visibility == "public" {
			return true
		}
		isAllowed := len(service.AllowedUsers) == 0 || uid == service.Owner
		if !isAllowed {
			for _, id := range service.AllowedUsers {
				if uid == id {
					isAllowed = true
					break
				}
			}
		}

		if !isAllowed {
			c.String(http.StatusForbidden, "User %s doesn't have permision to get this service", uid)
			return false
		}
	}
	return true
}

func resolveServiceNamespace(service *types.Service, cfg *types.Config) string {
	if service.Namespace != "" {
		return service.Namespace
	}
	return cfg.ServicesNamespace
}
