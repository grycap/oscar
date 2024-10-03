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
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/grycap/oscar/v3/pkg/resourcemanager"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Variables used to configure jobs
var (
	// No retries
	backoffLimit int32
	// Don't restart jobs in order to keep logs
	restartPolicy = v1.RestartPolicyNever
	// command used for passing the event to faas-supervisor
	command = []string{"/bin/sh"}
)

const (
	SupervisorPath  = "./supervisor"
	NodeSelectorKey = "kubernetes.io/hostname"

	// Annotations for InterLink nodes
	InterLinkDNSPolicy          = "ClusterFirst"
	InterLinkRestartPolicy      = "OnFailure"
	InterLinkTolerationKey      = "virtual-node.interlink/no-schedule"
	InterLinkTolerationOperator = "Exists"
)

// MakeJobHandler makes a han/home/slangarita/Escritorio/interlink-cluster/PodCern/PodCern.yamldler to manage async invocations
func MakeJobHandler(cfg *types.Config, kubeClientset *kubernetes.Clientset, back types.ServerlessBackend, rm resourcemanager.ResourceManager) gin.HandlerFunc {
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

		// Get podSpec from the service
		podSpec, err := service.ToPodSpec(cfg)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		// Check auth token
		authHeader := c.GetHeader("Authorization")
		splitToken := strings.Split(authHeader, "Bearer ")
		if len(splitToken) != 2 {
			c.Status(http.StatusUnauthorized)
			return
		}

		// Check if reqToken is the service token
		rawToken := strings.TrimSpace(splitToken[1])
		if len(rawToken) == tokenLength {

			if rawToken != service.Token {
				c.Status(http.StatusUnauthorized)
				return
			}
		}

		//  If isn't service token check if it is an oidc token
		if len(rawToken) != tokenLength {
			oidcManager, _ := auth.NewOIDCManager(cfg.OIDCIssuer, cfg.OIDCSubject, cfg.OIDCGroups)

			if !oidcManager.IsAuthorised(rawToken) {
				c.Status(http.StatusUnauthorized)
				return
			}

			hasVO, err := oidcManager.UserHasVO(rawToken, service.VO)

			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}

			if !hasVO {
				c.String(http.StatusUnauthorized, "this user isn't enrrolled on the vo: %v", service.VO)
				return
			}
		}

		// Get the event from request body
		eventBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		// Extract user UID from MinIO event
		var decoded map[string]interface{}
		if err := json.Unmarshal(eventBytes, &decoded); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		records := decoded["Records"].([]interface{})
		// Check if it has the MinIO event format
		if records != nil {
			r := records[0].(map[string]interface{})

			eventInfo := r["requestParameters"].(map[string]interface{})
			uid := eventInfo["principalId"]
			sourceIPAddress := eventInfo["sourceIPAddress"]

			c.Set("IPAddress", sourceIPAddress)
			c.Set("uidOrigin", uid)
		} else {
			c.Set("uidOrigin", "nil")
		}
		c.Next()

		// Initialize event envVar and args var
		event := v1.EnvVar{}
		var args []string

		if cfg.InterLinkAvailable && service.InterLinkNodeName != "" {
			command, event, args = types.SetInterlinkJob(podSpec, service, cfg, eventBytes)
		} else {

			if service.Mount.Provider != "" {
				args = []string{"-c", fmt.Sprintf("echo $%s | %s", types.EventVariable, service.GetSupervisorPath()) + ";echo \"I finish\" > /tmpfolder/finish-file;"}
				types.SetMount(podSpec, *service, cfg)
			} else {
				event = v1.EnvVar{
					Name:  types.EventVariable,
					Value: string(eventBytes),
				}
				args = []string{"-c", fmt.Sprintf("echo $%s | %s", types.EventVariable, service.GetSupervisorPath())}
			}
		}

		// Make JOB_UUID envVar
		jobUUID := uuid.New().String()
		jobUUIDVar := v1.EnvVar{
			Name:  types.JobUUIDVariable,
			Value: jobUUID,
		}

		// Make RESOURCE_ID envVar
		resourceIDVar := v1.EnvVar{
			Name: "RESOURCE_ID",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		}

		// Add podSpec variables
		podSpec.RestartPolicy = restartPolicy
		for i, c := range podSpec.Containers {
			if c.Name == types.ContainerName {
				podSpec.Containers[i].Command = command
				podSpec.Containers[i].Args = args
				podSpec.Containers[i].Env = append(podSpec.Containers[i].Env, event)
				podSpec.Containers[i].Env = append(podSpec.Containers[i].Env, jobUUIDVar)
				podSpec.Containers[i].Env = append(podSpec.Containers[i].Env, resourceIDVar)
			}
		}

		// Delegate job if can't be scheduled and has defined replicas
		if rm != nil && service.HasReplicas() {
			if !rm.IsSchedulable(podSpec.Containers[0].Resources) {
				err := resourcemanager.DelegateJob(service, event.Value, resourcemanager.ResourceManagerLogger)
				if err == nil {
					// TODO: check if another status code suits better
					c.Status(http.StatusCreated)
					return
				}
				log.Printf("unable to delegate job. Error: %v\n", err)
			}
		}

		// Create job definition
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				// UUID used as a name for jobs
				// To filter jobs by service name use the label "oscar_service"
				Name:        jobUUID,
				Namespace:   cfg.ServicesNamespace,
				Labels:      service.Labels,
				Annotations: service.Annotations,
			},
			Spec: batchv1.JobSpec{
				BackoffLimit: &backoffLimit,
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      service.Labels,
						Annotations: service.Annotations,
					},
					Spec: *podSpec,
				},
			},
		}

		// Add ReScheduler label if there are replicas defined and the cfg.ReSchedulerEnable is true
		if service.HasReplicas() && cfg.ReSchedulerEnable {
			if service.ReSchedulerThreshold != 0 {
				job.Labels[types.ReSchedulerLabelKey] = strconv.Itoa(service.ReSchedulerThreshold)
			} else {
				job.Labels[types.ReSchedulerLabelKey] = strconv.Itoa(cfg.ReSchedulerThreshold)
			}
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
