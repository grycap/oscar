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
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/grycap/oscar/v3/pkg/backends/resources"
	"github.com/grycap/oscar/v3/pkg/resourcemanager"
	"github.com/grycap/oscar/v3/pkg/types"
	"github.com/grycap/oscar/v3/pkg/utils"
	"github.com/grycap/oscar/v3/pkg/utils/auth"
	genericErrors "github.com/pkg/errors"
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
	command   = []string{"/bin/sh"}
	jobLogger = log.New(os.Stdout, "[JOB-HANDLER] ", log.Flags())
)

const (
	SupervisorPath        = "./supervisor"
	NodeSelectorKey       = "kubernetes.io/hostname"
	MinIODefaultPath      = "/var/run/secrets/providers/minio.default"
	MinIOSecretVolumeName = "minio-user"

	// Annotations for InterLink nodes
	InterLinkDNSPolicy          = "ClusterFirst"
	InterLinkRestartPolicy      = "OnFailure"
	InterLinkTolerationKey      = "virtual-node.interlink/no-schedule"
	InterLinkTolerationOperator = "Exists"
)

// MakeJobHandler godoc
// @Summary Invoke service asynchronously
// @Description Invoke a service asynchronously creating a Kubernetes Job.
// @Tags async
// @Accept json
// @Accept octet-stream
// @Param serviceName path string true "Service name"
// @Param payload body string false "Event payload"
// @Success 201 {string} string "Created"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Internal Server Error"
// @Security BearerAuth
// @Router /job/{serviceName} [post]
func MakeJobHandler(cfg *types.Config, kubeClientset kubernetes.Interface, back types.ServerlessBackend, rm resourcemanager.ResourceManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		service, err := back.ReadService("", c.Param("serviceName"))
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
		serviceNamespace := service.Namespace
		if serviceNamespace == "" {
			serviceNamespace = cfg.ServicesNamespace
		}

		// Check auth token
		authHeader := c.GetHeader("Authorization")
		splitToken := strings.Split(authHeader, "Bearer ")
		if len(splitToken) != 2 {
			c.Status(http.StatusUnauthorized)
			return
		}

		// Check if reqToken is the service token
		var uidFromToken string
		var minIOSecretKey string
		rawToken := strings.TrimSpace(splitToken[1])
		if len(rawToken) == tokenLength {

			if rawToken != service.Token {
				c.Status(http.StatusUnauthorized)
				return
			}
			// Use
			minIOSecretKey = service.Owner
		} else {
			//  If isn't service token check if it is an oidc token
			issuer, err := auth.GetIssuerFromToken(rawToken)
			if err != nil {
				c.String(http.StatusBadGateway, fmt.Sprintf("%v", err))
			}
			oidcManager := auth.ClusterOidcManagers[issuer]
			if oidcManager == nil {
				c.String(http.StatusBadRequest, fmt.Sprintf("Error getting oidc manager for issuer '%s'", issuer))
				return
			}

			ui, err := oidcManager.GetUserInfo(rawToken)
			uidFromToken = ui.Subject
			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}
			uid := auth.FormatUID(uidFromToken)
			if len(uid) > 63 {
				uid = uid[:63]
			}
			service.Labels[types.JobOwnerExecutionAnnotation] = uid
			if !oidcManager.IsAuthorised(rawToken) {
				c.Status(http.StatusUnauthorized)
				return
			}

			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}

			if !oidcManager.UserHasVO(ui, service.VO) {
				c.String(http.StatusUnauthorized, "this user isn't enrrolled on the vo: %v", service.VO)
				return
			}
		}
		// Add secrets as environment variables if defined
		if utils.SecretExists(service.Name, serviceNamespace, back.GetKubeClientset()) {
			podSpec.Containers[0].EnvFrom = []v1.EnvFromSource{
				{
					SecretRef: &v1.SecretEnvSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: service.Name,
						},
					},
				},
			}
		}
		// Get the event from request body
		eventBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		// Check if it has the MinIO event format
		requestUserUID, sourceIPAddress, err := decodeEventBytes(eventBytes)
		if err != nil {
			// Check if the request was made with OIDC token to get user UID
			if uidFromToken != "" {
				minIOSecretKey = uidFromToken
				c.Set("uidOrigin", uidFromToken)
			} else {
				// Set as nil string if unable to get an UID
				jobLogger.Println("WARNING:", err)
				c.Set("uidOrigin", "nil")
			}
		} else {
			if service.Labels == nil {
				service.Labels = make(map[string]string)
			}
			uid := auth.FormatUID(uidFromToken)
			if len(uid) > 62 {
				uid = uid[:62]
			}
			service.Labels[types.JobOwnerExecutionAnnotation] = uid
			c.Set("IPAddress", sourceIPAddress)
			c.Set("uidOrigin", requestUserUID)
			minIOSecretKey = requestUserUID
		}

		if minIOSecretKey == "" {
			minIOSecretKey = service.Owner
		}

		if err := ensureMinIOSecret(kubeClientset, minIOSecretKey, serviceNamespace); err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("error ensuring credentials for user %s: %v", minIOSecretKey, err))
			return
		}

		c.Next()

		// Mount user MinIO credentials
		podSpec.Volumes = append(podSpec.Volumes, v1.Volume{
			Name: MinIOSecretVolumeName,
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: auth.FormatUID(minIOSecretKey),
				},
			},
		})

		podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, v1.VolumeMount{
			Name:      MinIOSecretVolumeName,
			ReadOnly:  true,
			MountPath: MinIODefaultPath,
		})

		// Initialize event envVar and args var
		var event v1.EnvVar
		var args []string

		if cfg.InterLinkAvailable && service.InterLinkNodeName != "" {
			command, event, args = types.SetInterlinkJob(podSpec, service, cfg, eventBytes)
		} else {

			if service.Mount.Provider != "" {
				args = []string{"-c", fmt.Sprintf("echo $%s | %s", types.EventVariable, service.GetSupervisorPath()) + ";echo \"I finish\" > /tmpfolder/finish-file;"}
				resources.SetMount(podSpec, *service, cfg)
			} else {
				args = []string{"-c", fmt.Sprintf("echo $%s | %s", types.EventVariable, service.GetSupervisorPath())}
			}

			event = v1.EnvVar{
				Name:  types.EventVariable,
				Value: string(eventBytes),
			}
		}

		// Make JOB_UUID envVar
		serviceNameLenght := len(service.Name)
		serviceName := service.Name
		jobUUID := uuid.New().String()

		if serviceNameLenght >= 25 {
			serviceName = serviceName[:16]
		}
		jobUUID = serviceName + "-" + jobUUID
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
				jobLogger.Printf("unable to delegate job. Error: %v\n", err)
			}
		}

		// Create job definition
		ttl := int32(cfg.TTLJob) // #nosec
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				// UUID used as a name for jobs
				// To filter jobs by service name use the label "oscar_service"
				Name:        jobUUID,
				Namespace:   serviceNamespace,
				Labels:      service.Labels,
				Annotations: service.Annotations,
			},
			Spec: batchv1.JobSpec{
				BackoffLimit:            &backoffLimit,
				TTLSecondsAfterFinished: &ttl,
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

		_, err = kubeClientset.BatchV1().Jobs(serviceNamespace).Create(context.TODO(), job, metav1.CreateOptions{})
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.Status(http.StatusCreated)
	}
}

func decodeEventBytes(eventBytes []byte) (string, string, error) {

	defer func() {
		// recover from panic, if one occurs
		if r := recover(); r != nil {
			jobLogger.Println("Recovered from panic:", r)
		}
	}()
	// Extract user UID from MinIO event
	var decoded map[string]interface{}
	if err := json.Unmarshal(eventBytes, &decoded); err != nil {
		return "", "", err
	}

	if records, panicErr := decoded["Records"].([]interface{}); panicErr {
		r := records[0].(map[string]interface{})

		eventInfo := r["requestParameters"].(map[string]interface{})
		uid := eventInfo["principalId"]
		sourceIPAddress := eventInfo["sourceIPAddress"]

		return uid.(string), sourceIPAddress.(string), nil
	} else {
		return "", "", genericErrors.New("Failed to decode records")
	}

}

func ensureMinIOSecret(kubeClientset kubernetes.Interface, uid, namespace string) error {
	if namespace == "" || namespace == auth.ServicesNamespace {
		return nil
	}
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return nil
	}

	secretName := auth.FormatUID(uid)
	if _, err := kubeClientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{}); err == nil {
		return nil
	} else if !errors.IsNotFound(err) {
		return err
	}

	baseSecret, err := kubeClientset.CoreV1().Secrets(auth.ServicesNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	newSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        secretName,
			Namespace:   namespace,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
		Data: map[string][]byte{},
		Type: baseSecret.Type,
	}

	for k, v := range baseSecret.Data {
		if v != nil {
			copied := make([]byte, len(v))
			copy(copied, v)
			newSecret.Data[k] = copied
		} else {
			newSecret.Data[k] = nil
		}
	}
	for k, v := range baseSecret.Labels {
		newSecret.Labels[k] = v
	}
	for k, v := range baseSecret.Annotations {
		newSecret.Annotations[k] = v
	}

	if _, err := kubeClientset.CoreV1().Secrets(namespace).Create(context.TODO(), newSecret, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
