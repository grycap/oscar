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

package types

import (
	"fmt"

	"github.com/goccy/go-yaml"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// ContainerName name of the service container
	ContainerName = "oscar-container"

	// VolumeName name of the volume for mounting the OSCAR PVC
	VolumeName = "oscar-volume"

	// VolumePath path to mount the OSCAR PVC
	VolumePath = "/oscar/bin"

	// AlpineDirectory name of the Alpine binary directory
	AlpineDirectory = "alpine"

	// ConfigVolumeName name of the volume for mounting the service configMap
	ConfigVolumeName = "oscar-config"

	// ConfigPath path to mount the service configMap
	ConfigPath = "/oscar/config"

	// FDLFileName name of the FDL file to be stored in the service's configMap
	FDLFileName = "function_config.yaml"

	// ScriptFileName name of the user script file to be stored in the service's configMap
	ScriptFileName = "script.sh"

	// PVCName name of the OSCAR PVC
	PVCName = "oscar-pvc"

	// SupervisorName name of the FaaS Supervisor binary
	SupervisorName = "supervisor"

	// ServiceLabel label for deploying services in all backs
	ServiceLabel = "oscar_service"

	// EventVariable name used by the environment variable where events are stored
	EventVariable = "EVENT"

	// JobUUIDVariable name used by the environment variable of the job UUID
	JobUUIDVariable = "JOB_UUID"

	// YunikornApplicationIDLabel label to define the Yunikorn's application ID
	YunikornApplicationIDLabel = "applicationId"

	// YunikornQueueLabel label to define the Yunikorn's queue
	YunikornQueueLabel = "queue"

	// YunikornOscarQueue name of the Yunikorn's queue used for OSCAR services
	YunikornOscarQueue = "oscar-queue"

	// YunikornRootQueue name of the root Yunikorn's queue
	YunikornRootQueue = "root"

	// YunikornDefaultPartition name of the default Yunikorn partition
	YunikornDefaultPartition = "default"

	// KnativeVisibilityLabel name of the knative visibility label
	KnativeVisibilityLabel = "networking.knative.dev/visibility"

	// KnativeClusterLocalValue cluster-local value for the visibility label
	KnativeClusterLocalValue = "cluster-local"

	// KnativeMinScaleAnnotation annotation key to set the minimum number of replicas for a Knative service
	KnativeMinScaleAnnotation = "autoscaling.knative.dev/min-scale"

	// KnativeMaxScaleAnnotation annotation key to set the maximum number of replicas for a Knative service
	KnativeMaxScaleAnnotation = "autoscaling.knative.dev/max-scale"

	// AccessTokenEnvVar name of the environment variable used to propagate the access token
	AccessTokenEnvVar = "ACCESS_TOKEN"

	// ReSchedulerLabelKey label key to enable/disable the ReScheduler
	ReSchedulerLabelKey = "oscar_rescheduler"

	IsolationLevelUser = "USER"

	IsolationLevelService = "SERVICE"

	DefaultOwner = "cluster_admin"
)

// YAMLMarshal package-level yaml marshal function
var YAMLMarshal = yaml.Marshal

// Service represents an OSCAR service following the SCAR Function Definition Language
type Service struct {
	// Name the name of the service
	Name string `json:"name" binding:"required,max=39,min=1"`

	// ClusterID identifier for the current cluster, used to specify the cluster's StorageProvider in job delegations
	// Optional. (default: ""). OSCAR-CLI sets it using the ClusterID from the FDL
	ClusterID string `json:"cluster_id"`

	// Memory memory limit for the service following the kubernetes format
	// https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-memory
	// Optional. (default: 256Mi)
	Memory string `json:"memory"`

	// CPU cpu limit for the service following the kubernetes format
	// https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu
	// Optional. (default: 0.2)
	CPU string `json:"cpu"`

	// TotalMemory limit for the memory used by all the service's jobs running simultaneously
	// Apache YuniKorn scheduler is required to work
	// Same format as Memory, but internally translated to MB (integer)
	// Optional. (default: "")
	TotalMemory string `json:"total_memory"`

	// TotalCPU limit for the virtual CPUs used by all the service's jobs running simultaneously
	// Apache YuniKorn scheduler is required to work
	// Same format as CPU, but internally translated to millicores (integer)
	// Optional. (default: "")
	TotalCPU string `json:"total_cpu"`

	// EnableGPU parameter to request gpu usage in service's executions (synchronous and asynchronous)
	// Optional. (default: false)
	EnableGPU bool `json:"enable_gpu"`

	// EnableSGX parameter to use the SCONE k8s plugin
	// Optional. (default: false)
	EnableSGX bool `json:"enable_sgx"`

	// ImagePrefetch parameter to enable the image cache functionality
	// Optional. (default: false)
	ImagePrefetch bool `json:"image_prefetch"`

	// Synchronous struct to configure specific sync parameters
	// Only Knative ServerlessBackend applies this settings
	// Optional.
	Synchronous struct {
		// MinScale minimum number of active replicas (pods) for the service
		// Optional. (default: 0)
		MinScale int `json:"min_scale"`
		// MaxScale maximum number of active replicas (pods) for the service
		// Optional. (default: 0 [Unlimited])
		MaxScale int `json:"max_scale"`
	} `json:"synchronous"`

	// Replicas list of replicas to delegate jobs
	// Optional
	Replicas ReplicaList `json:"replicas,omitempty"`

	//Delegation Mode of job delegation for replicas
	// Opcional (default: manual)
	//"static" The user select the priority to delegate jobs to the replicas.
	//"random" The job delegation priority is generated randomly among the clusters of the available replicas.
	//"load-based" The job delegation priority is generated depending on the CPU and Memory available in the replica clusters.
	Delegation string `json:"delegation"`

	// ReSchedulerThreshold time (in seconds) that a job (with replicas) can be queued before delegating it
	// Optional
	ReSchedulerThreshold int `json:"rescheduler_threshold"`

	// LogLevel log level for the FaaS Supervisor
	// Optional. (default: INFO)
	LogLevel string `json:"log_level"`

	// Image Docker image for the service
	Image string `json:"image" binding:"required"`

	// Alpine parameter to set if image is based on Alpine
	// A custom release of faas-supervisor will be used
	// Optional. (default: false)
	Alpine bool `json:"alpine"`

	// PropagateToken parameter to make the service access token available inside the container
	// Optional. (default: false)
	PropagateToken bool `json:"propagate_token"`

	// Token token for sync and async invocations
	// Read only. This field is automatically generated by OSCAR
	Token string `json:"token"`

	// A parameter to disable the download of input files by the FaaS Supervisor
	// Optional. (default: false)
	FileStageIn bool `json:"file_stage_in"`

	// Input StorageIOConfig slice with the input service configuration
	// Optional
	Input []StorageIOConfig `json:"input"`

	// Output StorageIOConfig slice with the output service configuration
	// Optional
	Output []StorageIOConfig `json:"output"`

	// Script the user script to execute when the service is invoked
	Script string `json:"script,omitempty" binding:"required"`

	// ImagePullSecrets list of Kubernetes secrets to login to a private registry
	// Optional
	ImagePullSecrets []string `json:"image_pull_secrets,omitempty"`

	Expose Expose `json:"expose"`

	// The user-defined environment variables assigned to the service
	// Optional
	Environment struct {
		Vars    map[string]string `json:"variables"`
		Secrets map[string]string `json:"secrets"`
	} `json:"environment"`

	// Annotations user-defined Kubernetes annotations to be set in job's definition
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
	// Optional
	Annotations map[string]string `json:"annotations"`

	// Parameter to specify the VO from the user creating the service
	// Optional
	VO string `json:"vo"`

	// Labels user-defined Kubernetes labels to be set in job's definition
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
	// Optional
	Labels map[string]string `json:"labels"`

	// StorageProviders configuration for the storage providers used by the service
	// Optional. (default: MinIOProvider["default"] with the server's config credentials)
	StorageProviders *StorageProviders `json:"storage_providers,omitempty"`

	// Clusters configuration for the OSCAR clusters that can be used as service's replicas
	// Optional
	Clusters map[string]Cluster `json:"clusters,omitempty"`

	// EGI UID of the user that created the service
	// If the service is created through basic auth the default owner is "cluster_admin"
	Owner string `json:"owner"`

	InterLinkNodeName string `json:"interlink_node_name"`

	// Visibility sets which users will be able to interact with the service
	// "private" The default state of the service, which means only the owner of the same can interact with it
	// "public"  Every user can see the service and its buckets
	// "restricted" A list of users, set on the "allowed_users" variable are able to interact with the service
	Visibility string `json:"visibility"`

	// AllowedUsers list of EGI UID's identifying the users that will have visibility of the service and its MinIO storage provider
	// Optional (If the list is empty we asume the visibility is public for all cluster users)
	AllowedUsers []string `json:"allowed_users"`

	// IsolationLevel level of isolation for the buckets of the service (default:service)
	IsolationLevel string `json:"isolation_level" default:"SERVICE"`

	// BucketList autogenerated list of private buckets based on the allowed_users of the service
	BucketList []string `json:"bucket_list"`

	// Mount configuration to create a storage provider as a volume inside the service container
	// Optional
	Mount StorageIOConfig `json:"mount"`
}

type Expose struct {
	MinScale       int32 `json:"min_scale" default:"1"`
	MaxScale       int32 `json:"max_scale" default:"10"`
	APIPort        int   `json:"api_port,omitempty" `
	CpuThreshold   int32 `json:"cpu_threshold" default:"80" `
	RewriteTarget  bool  `json:"rewrite_target" default:"false" `
	NodePort       int32 `json:"nodePort" default:"0" `
	DefaultCommand bool  `json:"default_command" `
	SetAuth        bool  `json:"set_auth" `
}

// ToPodSpec returns a k8s podSpec from the Service
func (service *Service) ToPodSpec(cfg *Config) (*v1.PodSpec, error) {
	resources, err := createResources(service)
	if err != nil {
		return nil, err
	}

	podSpec := &v1.PodSpec{
		ImagePullSecrets: SetImagePullSecrets(service.ImagePullSecrets),
		Containers: []v1.Container{
			{
				Name:            ContainerName,
				Image:           service.Image,
				ImagePullPolicy: v1.PullAlways,
				Env:             ConvertEnvVars(service.Environment.Vars),
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      ConfigVolumeName,
						ReadOnly:  true,
						MountPath: ConfigPath,
					},
				},
				Command:   []string{service.GetSupervisorPath()},
				Resources: resources,
			},
		},
		Volumes: []v1.Volume{
			{
				Name: ConfigVolumeName,
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: service.Name,
						},
					},
				},
			},
		},
	}
	if cfg.InterLinkAvailable && service.InterLinkNodeName != "" {
		// Add specs of InterLink
		SetInterlinkService(podSpec)
	} else {
		// Add specs
		volumeMount := v1.VolumeMount{
			Name:      VolumeName,
			ReadOnly:  true,
			MountPath: VolumePath,
		}
		volume := v1.Volume{

			Name: VolumeName,
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: PVCName,
				},
			},
		}
		podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, volumeMount)
		podSpec.Volumes = append(podSpec.Volumes, volume)
	}

	if service.PropagateToken && service.Token != "" {
		addAccessTokenEnvVar(podSpec, service.Token)
	}

	if service.EnableSGX {
		SetSecurityContext(podSpec)
	}

	return podSpec, nil
}

// ToYAML returns the service as a Function Definition Language YAML
func (service Service) ToYAML() (string, error) {
	bytes, err := YAMLMarshal(service)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// GetMinIOWebhookARN returns the MinIO's notify_webhook ARN for the specified function
func (service *Service) GetMinIOWebhookARN() string {
	return fmt.Sprintf("arn:minio:sqs:%s:%s:webhook", service.StorageProviders.MinIO[DefaultProvider].Region, service.Name)
}

func ConvertEnvVars(vars map[string]string) []v1.EnvVar {
	envVars := []v1.EnvVar{}
	for k, v := range vars {
		envVars = append(envVars, v1.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	return envVars
}

func ConvertSecretsEnvVars(secretName string) []v1.EnvFromSource {
	return []v1.EnvFromSource{
		{
			SecretRef: &v1.SecretEnvSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: secretName,
				},
			},
		},
	}
}

func addAccessTokenEnvVar(p *v1.PodSpec, token string) {
	tokenEnvVar := v1.EnvVar{
		Name:  AccessTokenEnvVar,
		Value: token,
	}

	for i, cont := range p.Containers {
		if cont.Name == ContainerName {
			p.Containers[i].Env = append(p.Containers[i].Env, tokenEnvVar)
			break
		}
	}
}

func SetImagePullSecrets(secrets []string) []v1.LocalObjectReference {
	objects := []v1.LocalObjectReference{}
	for _, s := range secrets {
		objects = append(objects, v1.LocalObjectReference{
			Name: s,
		})
	}
	return objects
}

func SetSecurityContext(podSpec *v1.PodSpec) {
	ctx := v1.SecurityContext{
		Capabilities: &v1.Capabilities{
			Add: []v1.Capability{"SYS_RAWIO"},
		},
	}

	podSpec.Containers[0].SecurityContext = &ctx
}

func createResources(service *Service) (v1.ResourceRequirements, error) {
	resources := v1.ResourceRequirements{
		Limits: v1.ResourceList{},
	}

	if len(service.CPU) > 0 {
		cpu, err := resource.ParseQuantity(service.CPU)
		if err != nil {
			return resources, err
		}
		resources.Limits[v1.ResourceCPU] = cpu
	}

	if len(service.Memory) > 0 {
		memory, err := resource.ParseQuantity(service.Memory)
		if err != nil {
			return resources, err
		}
		resources.Limits[v1.ResourceMemory] = memory
	}

	if service.EnableGPU {
		gpu, err := resource.ParseQuantity("1")
		if err != nil {
			return resources, err
		}
		resources.Limits["nvidia.com/gpu"] = gpu
	}

	if service.EnableSGX {
		sgx, err := resource.ParseQuantity("1")
		if err != nil {
			return resources, err
		}
		resources.Limits["sgx.intel.com/enclave"] = sgx
	}

	return resources, nil
}

// GetSupervisorPath returns the appropriate supervisor path
func (service *Service) GetSupervisorPath() string {
	if service.Alpine {
		return fmt.Sprintf("%s/%s/%s", VolumePath, AlpineDirectory, SupervisorName)
	}
	return fmt.Sprintf("%s/%s", VolumePath, SupervisorName)
}

// HasReplicas checks if the service has replicas defined
func (service *Service) HasReplicas() bool {
	return len(service.Replicas) > 0
}
