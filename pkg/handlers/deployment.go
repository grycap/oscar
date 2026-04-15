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
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v3/pkg/backends"
	"github.com/grycap/oscar/v3/pkg/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	knapis "knative.dev/pkg/apis"
	knv1 "knative.dev/serving/pkg/apis/serving/v1"
)

type deploymentRuntimeServiceGetter interface {
	GetRuntimeService(namespace, name string) (*knv1.Service, error)
}

type deploymentRuntimeContext struct {
	status        types.ServiceDeploymentStatus
	logPods       []corev1.Pod
	usingLastLogs bool
}

type logEntryWithTime struct {
	entry   types.DeploymentLogEntry
	sortKey time.Time
	hasTime bool
}

// MakeGetDeploymentStatusHandler godoc
// @Summary Get service deployment summary
// @Description Retrieve the current deployment visibility summary for a service.
// @Tags deployment
// @Produce json
// @Param serviceName path string true "Service name"
// @Success 200 {object} types.ServiceDeploymentStatus
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/services/{serviceName}/deployment [get]
func MakeGetDeploymentStatusHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		serviceName, ok := validateServiceName(c, c.Param("serviceName"))
		if !ok {
			c.String(http.StatusBadRequest, serviceName)
			return
		}
		service, ok := getAuthorizedService(c, back, serviceName)
		if !ok {
			c.String(http.StatusForbidden, "You do not have permission to access this service")
			return
		}

		runtimeCtx, err := inspectDeploymentRuntime(back, kubeClientset, service, cfg)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.JSON(http.StatusOK, runtimeCtx.status)
	}
}

// MakeGetDeploymentLogsHandler godoc
// @Summary Get service deployment logs
// @Description Retrieve current or recent deployment logs for a service.
// @Tags deployment
// @Produce json
// @Param serviceName path string true "Service name"
// @Param timestamps query bool false "Include timestamps"
// @Param tailLines query int false "Maximum number of recent log entries to return"
// @Success 200 {object} types.DeploymentLogStream
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/services/{serviceName}/deployment/logs [get]
func MakeGetDeploymentLogsHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		serviceName, ok := validateServiceName(c, c.Param("serviceName"))
		if !ok {
			c.String(http.StatusBadRequest, serviceName)
			return
		}
		service, ok := getAuthorizedService(c, back, serviceName)
		if !ok {
			c.String(http.StatusForbidden, "You do not have permission to access this service")
			return
		}

		includeTimestamps, _ := strconv.ParseBool(c.DefaultQuery("timestamps", "false"))
		tailLines := parseTailLines(c.DefaultQuery("tailLines", "200"))

		runtimeCtx, err := inspectDeploymentRuntime(back, kubeClientset, service, cfg)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		response := types.DeploymentLogStream{
			ServiceName: service.Name,
			Entries:     []types.DeploymentLogEntry{},
		}

		if len(runtimeCtx.logPods) == 0 {
			response.Available = false
			response.Message = "Deployment logs are unavailable for this service."
			c.JSON(http.StatusOK, response)
			return
		}

		entries := collectDeploymentLogs(kubeClientset, service.Namespace, runtimeCtx.logPods, tailLines, includeTimestamps)
		if len(entries) == 0 {
			response.Available = false
			response.Message = "Deployment logs are not yet available."
			c.JSON(http.StatusOK, response)
			return
		}

		response.Available = true
		response.Entries = entries
		if runtimeCtx.usingLastLogs {
			response.Message = "Returning recent logs from the last runtime attempt while deployment status remains unavailable."
		}

		c.JSON(http.StatusOK, response)
	}
}

func inspectDeploymentRuntime(back types.ServerlessBackend, kubeClientset kubernetes.Interface, service *types.Service, cfg *types.Config) (deploymentRuntimeContext, error) {
	namespace := resolveServiceNamespace(service, cfg)
	service.Namespace = namespace

	if service.Expose.APIPort != 0 {
		return inspectExposedDeploymentRuntime(kubeClientset, service)
	}

	if getter, ok := back.(deploymentRuntimeServiceGetter); ok {
		knService, err := getter.GetRuntimeService(namespace, service.Name)
		if err == nil {
			return inspectKnativeDeploymentRuntime(kubeClientset, service, knService)
		}
		if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsGone(err) {
			return deploymentRuntimeContext{}, err
		}
	}

	return inspectPodBackedDeploymentRuntime(kubeClientset, service)
}

func deploymentSummaryFromStatus(status types.ServiceDeploymentStatus) *types.ServiceDeploymentSummary {
	return &types.ServiceDeploymentSummary{
		State:              status.State,
		Reason:             status.Reason,
		LastTransitionTime: status.LastTransitionTime,
		ActiveInstances:    status.ActiveInstances,
		AffectedInstances:  status.AffectedInstances,
		ResourceKind:       status.ResourceKind,
	}
}

func inspectExposedDeploymentRuntime(kubeClientset kubernetes.Interface, service *types.Service) (deploymentRuntimeContext, error) {
	pods, err := backends.ListExposedServicePods(kubeClientset, service.Namespace, service.Name)
	if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsGone(err) {
		return deploymentRuntimeContext{}, err
	}

	deployment, err := backends.GetExposedServiceDeployment(kubeClientset, service.Namespace, service.Name)
	if err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsGone(err) {
			items := podItems(pods)
			return deploymentRuntimeContext{
				status:        unavailableDeploymentStatus(service, "Current deployment resources are unavailable."),
				logPods:       items,
				usingLastLogs: len(items) > 0,
			}, nil
		}
		return deploymentRuntimeContext{}, err
	}

	return deploymentRuntimeContext{
		status:  deploymentStatusFromDeployment(service, deployment),
		logPods: podItems(pods),
	}, nil
}

func inspectKnativeDeploymentRuntime(kubeClientset kubernetes.Interface, service *types.Service, knService *knv1.Service) (deploymentRuntimeContext, error) {
	pods, err := backends.ListKnativeServicePods(kubeClientset, service.Namespace, service.Name)
	if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsGone(err) {
		return deploymentRuntimeContext{}, err
	}

	items := podItems(pods)
	current := filterCurrentRuntimePods(items)
	logPods := current
	usingLastLogs := false
	if len(logPods) == 0 && len(items) > 0 {
		logPods = items
		usingLastLogs = true
	}
	return deploymentRuntimeContext{
		status:        deploymentStatusFromKnativeService(service, knService, current),
		logPods:       logPods,
		usingLastLogs: usingLastLogs,
	}, nil
}

func inspectPodBackedDeploymentRuntime(kubeClientset kubernetes.Interface, service *types.Service) (deploymentRuntimeContext, error) {
	pods, err := backends.ListServicePods(kubeClientset, service.Namespace, service.Name)
	if err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsGone(err) {
			return deploymentRuntimeContext{
				status: unavailableDeploymentStatus(service, "Current runtime resources are unavailable."),
			}, nil
		}
		return deploymentRuntimeContext{}, err
	}

	items := podItems(pods)
	lastAttemptPods := filterCompletedRuntimePods(items)
	return deploymentRuntimeContext{
		status:        unavailableDeploymentStatus(service, "Current deployment visibility is unavailable for this service runtime."),
		logPods:       lastAttemptPods,
		usingLastLogs: len(lastAttemptPods) > 0,
	}, nil
}

func deploymentStatusFromDeployment(service *types.Service, deployment *appsv1.Deployment) types.ServiceDeploymentStatus {
	var desired int32 = 1
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}
	if desired < 0 {
		desired = 0
	}

	observed := deployment.Status.Replicas
	if observed < 0 {
		observed = 0
	}
	available := deployment.Status.AvailableReplicas
	affected := int(observed - available)
	if affected < 0 {
		affected = 0
	}

	state := types.DeploymentStatePending
	switch {
	case desired == 0:
		state = types.DeploymentStatePending
	case available >= desired:
		state = types.DeploymentStateReady
	case available > 0:
		state = types.DeploymentStateDegraded
	case hasDeploymentFailureCondition(deployment.Status.Conditions):
		state = types.DeploymentStateFailed
	default:
		state = types.DeploymentStatePending
	}

	reason, transitioned := latestDeploymentCondition(deployment.Status.Conditions)
	if reason == "" && state == types.DeploymentStateDegraded {
		reason = fmt.Sprintf("%d of %d instances are affected.", affected, desired)
	}
	if reason == "" && state == types.DeploymentStateReady {
		reason = "Deployment is ready."
	}

	return types.ServiceDeploymentStatus{
		ServiceName:        service.Name,
		Namespace:          service.Namespace,
		State:              state,
		Reason:             reason,
		LastTransitionTime: transitioned,
		ActiveInstances:    int(observed),
		AffectedInstances:  affected,
		ResourceKind:       types.DeploymentResourceKindExposedService,
	}
}

func deploymentStatusFromKnativeService(service *types.Service, knService *knv1.Service, pods []corev1.Pod) types.ServiceDeploymentStatus {
	total := len(pods)
	affected, podReason, podTransition := summarizePodHealth(pods)
	readyCondition := knService.Status.GetCondition(knapis.ConditionReady)

	state := types.DeploymentStatePending
	reason := podReason
	transitioned := podTransition
	if readyCondition != nil {
		transitioned = &readyCondition.LastTransitionTime.Inner
		reason = bestKnativeReason(knService.Status)
		switch readyCondition.Status {
		case corev1.ConditionTrue:
			state = types.DeploymentStateReady
			affected = 0
		case corev1.ConditionFalse:
			if total > 0 && affected < total {
				state = types.DeploymentStateDegraded
			} else {
				state = types.DeploymentStateFailed
			}
		default:
			state = types.DeploymentStatePending
		}
	}

	if reason == "" {
		switch state {
		case types.DeploymentStateReady:
			reason = "Runtime service is ready."
		case types.DeploymentStatePending:
			reason = "Runtime service is still reconciling."
		case types.DeploymentStateFailed:
			reason = "Runtime service is not ready."
		case types.DeploymentStateDegraded:
			reason = fmt.Sprintf("%d of %d instances are affected.", affected, total)
		}
	}

	return types.ServiceDeploymentStatus{
		ServiceName:        service.Name,
		Namespace:          service.Namespace,
		State:              state,
		Reason:             reason,
		LastTransitionTime: transitioned,
		ActiveInstances:    total,
		AffectedInstances:  affected,
		ResourceKind:       types.DeploymentResourceKindKnativeService,
	}
}

func bestKnativeReason(status knv1.ServiceStatus) string {
	for _, condType := range []knapis.ConditionType{
		knv1.ServiceConditionConfigurationsReady,
		knv1.ServiceConditionRoutesReady,
		knapis.ConditionReady,
	} {
		condition := status.GetCondition(condType)
		if condition == nil {
			continue
		}
		message := sanitizeKnativeReason(condition.Message)
		if message != "" {
			return message
		}
		if condition.Reason != "" {
			return condition.Reason
		}
	}
	return ""
}

func sanitizeKnativeReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}

	if cut := strings.Index(reason, "<!DOCTYPE html>"); cut >= 0 {
		reason = strings.TrimSpace(reason[:cut])
	}
	if cut := strings.Index(reason, "\n"); cut >= 0 {
		reason = strings.TrimSpace(reason[:cut])
	}

	return strings.Join(strings.Fields(reason), " ")
}

func unavailableDeploymentStatus(service *types.Service, reason string) types.ServiceDeploymentStatus {
	return types.ServiceDeploymentStatus{
		ServiceName:       service.Name,
		Namespace:         service.Namespace,
		State:             types.DeploymentStateUnavailable,
		Reason:            reason,
		ActiveInstances:   0,
		AffectedInstances: 0,
		ResourceKind:      types.DeploymentResourceKindUnavailable,
	}
}

func podItems(pods *corev1.PodList) []corev1.Pod {
	if pods == nil {
		return nil
	}
	return pods.Items
}

func filterCurrentRuntimePods(pods []corev1.Pod) []corev1.Pod {
	active := make([]corev1.Pod, 0, len(pods))
	for _, pod := range pods {
		switch pod.Status.Phase {
		case corev1.PodPending, corev1.PodRunning:
			active = append(active, pod)
		}
	}
	return active
}

func filterCompletedRuntimePods(pods []corev1.Pod) []corev1.Pod {
	completed := make([]corev1.Pod, 0, len(pods))
	for _, pod := range pods {
		switch pod.Status.Phase {
		case corev1.PodSucceeded, corev1.PodFailed:
			completed = append(completed, pod)
		}
	}
	return completed
}

func summarizePodHealth(pods []corev1.Pod) (int, string, *metav1.Time) {
	affected := 0
	var reason string
	var transitioned *metav1.Time

	for _, pod := range pods {
		if !isPodHealthy(pod) {
			affected++
			podReason, podTransition := podFailureReason(pod)
			if reason == "" && podReason != "" {
				reason = podReason
			}
			transitioned = latestTime(transitioned, podTransition)
		}
	}

	return affected, reason, transitioned
}

func isPodHealthy(pod corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}

	return false
}

func podFailureReason(pod corev1.Pod) (string, *metav1.Time) {
	if pod.Status.Message != "" {
		return pod.Status.Message, pod.Status.StartTime
	}
	if pod.Status.Reason != "" {
		return pod.Status.Reason, pod.Status.StartTime
	}

	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil {
			message := strings.Trim(strings.TrimSpace(status.State.Waiting.Reason+": "+status.State.Waiting.Message), ": ")
			return message, pod.Status.StartTime
		}
		if status.State.Terminated != nil {
			finished := status.State.Terminated.FinishedAt
			message := strings.Trim(strings.TrimSpace(status.State.Terminated.Reason+": "+status.State.Terminated.Message), ": ")
			return message, &finished
		}
	}

	if pod.Status.Phase != "" {
		return string(pod.Status.Phase), pod.Status.StartTime
	}
	return "", pod.Status.StartTime
}

func latestDeploymentCondition(conditions []appsv1.DeploymentCondition) (string, *metav1.Time) {
	var latest *appsv1.DeploymentCondition
	for i := range conditions {
		condition := &conditions[i]
		if latest == nil || condition.LastUpdateTime.After(latest.LastUpdateTime.Time) {
			latest = condition
		}
	}

	if latest == nil {
		return "", nil
	}
	message := strings.Trim(strings.TrimSpace(latest.Reason+": "+latest.Message), ": ")
	return message, &latest.LastUpdateTime
}

func hasDeploymentFailureCondition(conditions []appsv1.DeploymentCondition) bool {
	for _, condition := range conditions {
		if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
			return true
		}
		if condition.Type == appsv1.DeploymentProgressing && condition.Status == corev1.ConditionFalse {
			return true
		}
	}
	return false
}

func latestTime(current, candidate *metav1.Time) *metav1.Time {
	switch {
	case current == nil:
		return candidate
	case candidate == nil:
		return current
	case candidate.After(current.Time):
		return candidate
	default:
		return current
	}
}

func parseTailLines(value string) int64 {
	tailLines, err := strconv.ParseInt(value, 10, 64)
	if err != nil || tailLines < 1 {
		return 200
	}
	if tailLines > 1000 {
		return 1000
	}
	return tailLines
}

func collectDeploymentLogs(kubeClientset kubernetes.Interface, namespace string, pods []corev1.Pod, tailLines int64, includeTimestamps bool) []types.DeploymentLogEntry {
	entries := make([]logEntryWithTime, 0, len(pods))
	for _, pod := range pods {
		podEntries, err := getPodDeploymentLogs(kubeClientset, namespace, pod.Name, tailLines)
		if err != nil {
			continue
		}
		entries = append(entries, podEntries...)
	}

	sort.Slice(entries, func(i, j int) bool {
		switch {
		case entries[i].hasTime && entries[j].hasTime:
			return entries[i].sortKey.Before(entries[j].sortKey)
		case entries[i].hasTime:
			return true
		case entries[j].hasTime:
			return false
		default:
			return entries[i].entry.Message < entries[j].entry.Message
		}
	})

	if len(entries) > int(tailLines) {
		entries = entries[len(entries)-int(tailLines):]
	}

	responseEntries := make([]types.DeploymentLogEntry, 0, len(entries))
	for _, item := range entries {
		entry := item.entry
		if !includeTimestamps {
			entry.Timestamp = ""
		}
		responseEntries = append(responseEntries, entry)
	}
	return responseEntries
}

func getPodDeploymentLogs(kubeClientset kubernetes.Interface, namespace, podName string, tailLines int64) ([]logEntryWithTime, error) {
	req := kubeClientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container:  types.ContainerName,
		Timestamps: true,
		TailLines:  &tailLines,
	})
	result := req.Do(context.TODO())

	statusCode := new(int)
	if result.StatusCode(statusCode); *statusCode != http.StatusOK {
		return nil, errors.New("pod logs not available")
	}

	raw, err := result.Raw()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	entries := make([]logEntryWithTime, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entries = append(entries, parseDeploymentLogLine(line))
	}
	return entries, nil
}

func parseDeploymentLogLine(line string) logEntryWithTime {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 2 {
		if ts, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
			return logEntryWithTime{
				entry: types.DeploymentLogEntry{
					Timestamp: ts.UTC().Format(time.RFC3339),
					Message:   parts[1],
				},
				sortKey: ts.UTC(),
				hasTime: true,
			}
		}
	}

	return logEntryWithTime{
		entry: types.DeploymentLogEntry{
			Message: line,
		},
	}
}

func validateServiceName(c *gin.Context, serviceName string) (string, bool) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return "serviceName is required", false
	}
	if errs := validation.IsValidLabelValue(serviceName); len(errs) > 0 {
		return "invalid serviceName", false
	}
	return serviceName, true
}
