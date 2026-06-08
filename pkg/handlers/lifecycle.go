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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grycap/oscar/v4/pkg/backends"
	"github.com/grycap/oscar/v4/pkg/backends/resources"
	"github.com/grycap/oscar/v4/pkg/types"
	"github.com/grycap/oscar/v4/pkg/utils/auth"
	appsv1 "k8s.io/api/apps/v1"
	autosv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	lifecyclePreviousReplicasAnnotation = "oscar.grycap.io/previous-replicas"
	lifecycleStoppedAnnotation          = "oscar.grycap.io/stopped"
	lifecycleRestartedAtAnnotation      = "oscar.grycap.io/restarted-at"
)

// MakeStopExposedServiceHandler godoc
// @Summary Stop exposed service
// @Description Stop an exposed service by scaling its Deployment to zero replicas.
// @Tags services
// @Produce json
// @Param serviceName path string true "Service name"
// @Success 200 {object} types.ServiceDeploymentStatus
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/services/{serviceName}/stop [post]
func MakeStopExposedServiceHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return makeExposedServiceLifecycleHandler(back, kubeClientset, cfg, stopExposedService)
}

// MakeStartExposedServiceHandler godoc
// @Summary Start exposed service
// @Description Start a stopped exposed service by restoring its desired replicas.
// @Tags services
// @Produce json
// @Param serviceName path string true "Service name"
// @Success 200 {object} types.ServiceDeploymentStatus
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/services/{serviceName}/start [post]
func MakeStartExposedServiceHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return makeExposedServiceLifecycleHandler(back, kubeClientset, cfg, startExposedService)
}

// MakeRestartExposedServiceHandler godoc
// @Summary Restart exposed service
// @Description Restart an exposed service by rolling its Deployment pods.
// @Tags services
// @Produce json
// @Param serviceName path string true "Service name"
// @Success 200 {object} types.ServiceDeploymentStatus
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Forbidden"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BasicAuth
// @Security BearerAuth
// @Router /system/services/{serviceName}/restart [post]
func MakeRestartExposedServiceHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config) gin.HandlerFunc {
	return makeExposedServiceLifecycleHandler(back, kubeClientset, cfg, restartExposedService)
}

type exposedServiceLifecycleAction func(context.Context, kubernetes.Interface, *types.Service, *types.Config) error

func makeExposedServiceLifecycleHandler(back types.ServerlessBackend, kubeClientset kubernetes.Interface, cfg *types.Config, action exposedServiceLifecycleAction) gin.HandlerFunc {
	return func(c *gin.Context) {
		serviceName, ok := validateServiceName(c, c.Param("serviceName"))
		if !ok {
			c.String(http.StatusBadRequest, serviceName)
			return
		}
		service, ok := getAuthorizedServiceOwner(c, back, serviceName)
		if !ok {
			return
		}
		if len(service.Expose.APIPort) == 0 {
			c.String(http.StatusBadRequest, "service %q is not exposed", service.Name)
			return
		}
		service.Namespace = resolveLifecycleServiceNamespace(service, cfg)

		if err := action(c.Request.Context(), kubeClientset, service, cfg); err != nil {
			if apierrors.IsNotFound(err) || apierrors.IsGone(err) {
				c.Status(http.StatusNotFound)
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			return
		}

		status, err := inspectExposedDeploymentRuntimeStatusOnly(kubeClientset, service)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, status)
	}
}

func getAuthorizedServiceOwner(c *gin.Context, back types.ServerlessBackend, serviceName string) (*types.Service, bool) {
	service, err := back.ReadService("", serviceName)
	if err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsGone(err) {
			c.Status(http.StatusNotFound)
		} else {
			c.String(http.StatusInternalServerError, err.Error())
		}
		return nil, false
	}
	if !isBearerRequest(c) {
		return service, true
	}

	uid, err := auth.GetUIDFromContext(c)
	if err != nil {
		c.String(http.StatusUnauthorized, err.Error())
		return nil, false
	}
	if service.Owner != uid {
		c.String(http.StatusForbidden, "User %s doesn't have permission to manage this service", uid)
		return nil, false
	}
	return service, true
}

func stopExposedService(ctx context.Context, kubeClientset kubernetes.Interface, service *types.Service, cfg *types.Config) error {
	deployment, err := backends.GetExposedServiceDeployment(kubeClientset, service.Namespace, service.Name)
	if err != nil {
		return err
	}

	replicas := currentDeploymentReplicas(deployment)
	if replicas > 0 {
		setDeploymentAnnotation(deployment, lifecyclePreviousReplicasAnnotation, strconv.FormatInt(int64(replicas), 10))
	}
	setDeploymentAnnotation(deployment, lifecycleStoppedAnnotation, "true")

	zero := int32(0)
	deployment.Spec.Replicas = &zero
	if _, err := kubeClientset.AppsV1().Deployments(service.Namespace).Update(ctx, deployment, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return deleteExposedServiceHPA(ctx, kubeClientset, service.Namespace, service.Name)
}

func startExposedService(ctx context.Context, kubeClientset kubernetes.Interface, service *types.Service, cfg *types.Config) error {
	deployment, err := backends.GetExposedServiceDeployment(kubeClientset, service.Namespace, service.Name)
	if err != nil {
		return err
	}

	replicas := desiredStartReplicas(service, deployment)
	deployment.Spec.Replicas = &replicas
	setDeploymentAnnotation(deployment, lifecycleStoppedAnnotation, "false")
	if _, err := kubeClientset.AppsV1().Deployments(service.Namespace).Update(ctx, deployment, metav1.UpdateOptions{}); err != nil {
		return err
	}

	if err := resources.EnsureExposeAuthResources(*service, service.Namespace, kubeClientset, cfg); err != nil {
		return err
	}

	return ensureExposedServiceHPA(ctx, kubeClientset, service.Namespace, service)
}

func restartExposedService(ctx context.Context, kubeClientset kubernetes.Interface, service *types.Service, cfg *types.Config) error {
	deployment, err := backends.GetExposedServiceDeployment(kubeClientset, service.Namespace, service.Name)
	if err != nil {
		return err
	}

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = map[string]string{}
	}
	deployment.Spec.Template.Annotations[lifecycleRestartedAtAnnotation] = time.Now().UTC().Format(time.RFC3339Nano)
	_, err = kubeClientset.AppsV1().Deployments(service.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	return err
}

func resolveLifecycleServiceNamespace(service *types.Service, cfg *types.Config) string {
	if service.Namespace != "" {
		return service.Namespace
	}
	if cfg != nil {
		return cfg.ServicesNamespace
	}
	return ""
}

func currentDeploymentReplicas(deployment *appsv1.Deployment) int32 {
	if deployment.Spec.Replicas != nil {
		return *deployment.Spec.Replicas
	}
	return 0
}

func desiredStartReplicas(service *types.Service, deployment metav1.Object) int32 {
	if annotations := deployment.GetAnnotations(); annotations != nil {
		if value, ok := annotations[lifecyclePreviousReplicasAnnotation]; ok {
			if replicas, err := strconv.ParseInt(value, 10, 32); err == nil && replicas > 0 {
				return int32(replicas)
			}
		}
	}
	if service.Expose.MinScale > 0 {
		return service.Expose.MinScale
	}
	return 1
}

func setDeploymentAnnotation(deployment metav1.Object, key, value string) {
	annotations := deployment.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[key] = value
	deployment.SetAnnotations(annotations)
}

func deleteExposedServiceHPA(ctx context.Context, kubeClientset kubernetes.Interface, namespace, serviceName string) error {
	hpas, err := kubeClientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	targetName := serviceName + "-dpl"
	for _, hpa := range hpas.Items {
		if hpa.Spec.ScaleTargetRef.Kind == "Deployment" && hpa.Spec.ScaleTargetRef.Name == targetName {
			err := kubeClientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).Delete(ctx, hpa.Name, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsGone(err) {
				return err
			}
		}
	}
	return nil
}

func ensureExposedServiceHPA(ctx context.Context, kubeClientset kubernetes.Interface, namespace string, service *types.Service) error {
	hpas, err := kubeClientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	targetName := service.Name + "-dpl"
	for _, hpa := range hpas.Items {
		if hpa.Spec.ScaleTargetRef.Kind == "Deployment" && hpa.Spec.ScaleTargetRef.Name == targetName {
			return nil
		}
	}

	hpa := &autosv1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-hpa", service.Name),
			Namespace: namespace,
		},
		Spec: autosv1.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autosv1.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       targetName,
				APIVersion: "apps/v1",
			},
			MinReplicas:                    &service.Expose.MinScale,
			MaxReplicas:                    service.Expose.MaxScale,
			TargetCPUUtilizationPercentage: &service.Expose.CpuThreshold,
		},
	}
	_, err = kubeClientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).Create(ctx, hpa, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}
