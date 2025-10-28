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
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// TODO Try using cookies to avoid excesive calls to the k8s API //

// MakeJobsInfoHandler makes a handler for listing all existing jobs from a service and show their JobInfo
func MakeJobsInfoHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobsInfo := make(map[string]*types.JobInfo)
		// Get serviceName
		serviceName := c.Param("serviceName")
		page := c.DefaultQuery("page", "")
		isOIDCAuthorised(c, back, serviceName)

		// List jobs
		listOpts := metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", types.ServiceLabel, serviceName),
			Limit:         int64(cfg.JobListingLimit),
			Continue:      page,
		}

		// List jobs' pods
		pods, err := kubeClientset.CoreV1().Pods(cfg.ServicesNamespace).List(context.TODO(), listOpts)
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

// MakeDeleteJobsHandler makes a handler for deleting all jobs created by the provided service.
// If 'all' querystring is set to 'true' pending, running and failed jobs will also be deleted
func MakeDeleteJobsHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get serviceName and jobName
		serviceName := c.Param("serviceName")
		isOIDCAuthorised(c, back, serviceName)

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

		err = kubeClientset.BatchV1().Jobs(cfg.ServicesNamespace).DeleteCollection(context.TODO(), delOpts, listOpts)
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

// MakeGetLogsHandler makes a handler for getting logs from the 'oscar-container' inside the pod created by the specified job
func MakeGetLogsHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get serviceName and jobName
		serviceName := c.Param("serviceName")
		isOIDCAuthorised(c, back, serviceName)
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
		pods, err := kubeClientset.CoreV1().Pods(cfg.ServicesNamespace).List(context.TODO(), listOpts)
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
		req := kubeClientset.CoreV1().Pods(cfg.ServicesNamespace).GetLogs(pods.Items[0].Name, podLogOpts)
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

// MakeGetSystemLogsHandler makes a handler for getting OSCAR manager logs (Basic Auth only)
func MakeGetSystemLogsHandler(kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			c.String(http.StatusForbidden, "OIDC tokens are not allowed for system logs")
			return
		}

		timestamps, err := strconv.ParseBool(c.DefaultQuery("timestamps", "false"))
		if err != nil {
			timestamps = false
		}

		appLabel := cfg.Name
		if appLabel == "" {
			appLabel = "oscar"
		}

		listOpts := metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=%s", appLabel),
		}
		pods, err := kubeClientset.CoreV1().Pods(cfg.Namespace).List(context.TODO(), listOpts)
		if err != nil {
			if !errors.IsNotFound(err) && !errors.IsGone(err) {
				c.String(http.StatusInternalServerError, err.Error())
			} else {
				c.Status(http.StatusNotFound)
			}
			return
		}

		if len(pods.Items) == 0 {
			c.Status(http.StatusNotFound)
			return
		}

		targetPod := pods.Items[0]
		for _, pod := range pods.Items[1:] {
			if pod.CreationTimestamp.After(targetPod.CreationTimestamp.Time) {
				targetPod = pod
			}
		}

		containerName := ""
		if len(targetPod.Spec.Containers) > 0 {
			containerName = targetPod.Spec.Containers[0].Name
			for _, container := range targetPod.Spec.Containers {
				if container.Name == cfg.Name {
					containerName = container.Name
					break
				}
			}
		}

		podLogOpts := &v1.PodLogOptions{
			Timestamps: timestamps,
		}
		if containerName != "" {
			podLogOpts.Container = containerName
		}

		if prev, err := strconv.ParseBool(c.DefaultQuery("previous", "false")); err == nil && prev {
			podLogOpts.Previous = true
		}

		req := kubeClientset.CoreV1().Pods(cfg.Namespace).GetLogs(targetPod.Name, podLogOpts)

		stream, err := req.Stream(context.TODO())
		if err != nil {
			if statusErr, ok := err.(*errors.StatusError); ok {
				code := int(statusErr.Status().Code)
				if code == 0 {
					code = http.StatusInternalServerError
				}
				c.String(code, statusErr.Error())
				return
			}
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		defer stream.Close()

		buf := new(bytes.Buffer)
		if _, err = buf.ReadFrom(stream); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		logEntries := parseExecutionLogs(buf.String())

		c.JSON(http.StatusOK, gin.H{
			"logs": logEntries,
		})
	}
}

type executionLogEntry struct {
	Timestamp string `json:"timestamp"`
	Status    int    `json:"status"`
	Latency   string `json:"latency"`
	ClientIP  string `json:"client_ip"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	User      string `json:"user"`
	Raw       string `json:"raw"`
}

// parseExecutionLogs filters OSCAR execution log lines and structures them.
func parseExecutionLogs(raw string) []executionLogEntry {
	const prefix = "[GIN-EXECUTIONS-LOGGER]"

	logText := utils.NormalizeLineEndings(raw)
	lines := strings.Split(logText, "\n")
	filtered := make([]executionLogEntry, 0, len(lines))

	for _, line := range lines {
		rawLine := strings.TrimSpace(line)
		if len(rawLine) == 0 || !strings.Contains(rawLine, prefix) {
			continue
		}

		entry := executionLogEntry{Raw: rawLine}

		withoutPrefix := strings.TrimSpace(strings.TrimPrefix(rawLine, prefix))
		parts := strings.Split(withoutPrefix, "|")
		if len(parts) < 6 {
			continue
		}

		rawTimestamp := strings.TrimSpace(parts[0])
		if parsedTime, err := time.ParseInLocation("2006/01/02 - 15:04:05", rawTimestamp, time.Local); err == nil {
			entry.Timestamp = parsedTime.UTC().Format(time.RFC3339)
		} else {
			entry.Timestamp = rawTimestamp
		}
		if status, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
			entry.Status = status
		}
		entry.Latency = strings.TrimSpace(parts[2])
		entry.ClientIP = strings.TrimSpace(parts[3])
		methodPath := strings.TrimSpace(parts[4])
		user := strings.TrimSpace(parts[5])

		methodPathFields := strings.Fields(methodPath)
		if len(methodPathFields) < 2 {
			continue
		}
		entry.Method = methodPathFields[0]
		entry.Path = methodPathFields[1]
		entry.User = user

		if entry.Method != "POST" {
			continue
		}
		if !strings.HasPrefix(entry.Path, "/run") && !strings.HasPrefix(entry.Path, "/job") {
			continue
		}

		filtered = append(filtered, entry)
	}

	return filtered
}

// MakeDeleteJobHandler makes a handler for removing a job
func MakeDeleteJobHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get serviceName and jobName
		serviceName := c.Param("serviceName")
		isOIDCAuthorised(c, back, serviceName)
		jobName := c.Param("jobName")

		// Get job in order to check if it is associated with the provided serviceName
		job, err := kubeClientset.BatchV1().Jobs(cfg.ServicesNamespace).Get(context.TODO(), jobName, metav1.GetOptions{})
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
		err = kubeClientset.BatchV1().Jobs(cfg.ServicesNamespace).Delete(context.TODO(), jobName, delOpts)
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

func isOIDCAuthorised(c *gin.Context, back types.ServerlessBackend, serviceName string) {
	// If is oidc auth get service and check on allowed users
	authHeader := c.GetHeader("Authorization")
	if len(strings.Split(authHeader, "Bearer")) > 1 {
		service, _ := back.ReadService(serviceName)
		uid, err := auth.GetUIDFromContext(c)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintln(err))
		}

		var isAllowed bool
		if len(service.AllowedUsers) == 0 {
			isAllowed = true
		} else {
			for _, id := range service.AllowedUsers {
				if uid == id {
					isAllowed = true
					break
				}
			}
		}

		if !isAllowed {
			c.String(http.StatusForbidden, "User %s doesn't have permision to get this service", uid)
			return
		}
	}
}
