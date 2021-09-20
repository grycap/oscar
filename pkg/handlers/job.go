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
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/grycap/oscar/v2/pkg/types"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Variables used to configure jobs
var (
	// No retries
	backoffLimit int32 = 0
	// Don't restart jobs in order to keep logs
	restartPolicy = v1.RestartPolicyNever
	// commands and args used for passing event to faas-supervisor
	command = []string{"/bin/sh"}
	args    = []string{"-c", fmt.Sprintf("echo $%s | %s/%s", types.EventVariable, types.VolumePath, types.SupervisorName)}
)

// MakeJobHandler makes a handler to manage async invocations
func MakeJobHandler(cfg *types.Config, kubeClientset *kubernetes.Clientset, back types.ServerlessBackend) gin.HandlerFunc {
	return func(c *gin.Context) {
		service, err := back.ReadService(c.Param("serviceName"))
		if err != nil {
			// Check if error is caused because the service is not found
			if errors.IsNotFound(err) || errors.IsGone(err) {
				c.Status(http.StatusNotFound)
			} else {
				c.String(http.StatusInternalServerError, err.Error())
			}
			return
		}

		// Get the event from request body
		eventBytes, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		// Make event envVar
		event := v1.EnvVar{
			Name:  types.EventVariable,
			Value: string(eventBytes),
		}

		// Get podSpec from the service
		podSpec, err := service.ToPodSpec(cfg)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		// Add podSpec variables
		podSpec.RestartPolicy = restartPolicy
		for i, c := range podSpec.Containers {
			if c.Name == types.ContainerName {
				podSpec.Containers[i].Command = command
				podSpec.Containers[i].Args = args
				podSpec.Containers[i].Env = append(podSpec.Containers[i].Env, event)
			}
		}

		// Create job definition
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				// UUID used as a name for jobs
				// To filter jobs by service name use the label "oscar_service"
				Name:      uuid.New().String(),
				Namespace: cfg.ServicesNamespace,
				Labels: map[string]string{
					types.ServiceLabel: service.Name,
				},
			},
			Spec: batchv1.JobSpec{
				BackoffLimit: &backoffLimit,
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							types.ServiceLabel: service.Name,
						},
					},
					Spec: *podSpec,
				},
			},
		}

		// Create job
		_, err = kubeClientset.BatchV1().Jobs(cfg.ServicesNamespace).Create(context.TODO(), job, metav1.CreateOptions{})
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		c.Status(http.StatusCreated)
	}
}
