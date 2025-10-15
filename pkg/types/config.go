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
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// OpenFaaSBackend string to identify the OpenFaaS Serverless Backend in the configuration
	OpenFaaSBackend = "openfaas"
	// KnativeBackend string to identify the Knative Serverless Backend in the configuration
	KnativeBackend = "knative"

	stringType            = "string"
	stringSliceType       = "slice"
	intType               = "int"
	boolType              = "bool"
	secondsType           = "seconds"
	urlType               = "url"
	serverlessBackendType = "serverlessBackend"
)

type configVar struct {
	name         string
	envVarName   string
	required     bool
	varType      string
	defaultValue string
}

// UpdateConfigRequest represents the payload for mutable runtime configuration fields.
type UpdateConfigRequest struct {
	AllowedImageRepositories []string `json:"allowed_image_repositories"`
}

// Config stores the configuration for the OSCAR server
type Config struct {
	// MinIOProvider access info
	MinIOProvider *MinIOProvider `json:"-"`

	// Basic auth username
	Username string `json:"-"`

	// Basic auth password
	Password string `json:"-"`

	// Kubernetes name for the deployment and service (default: oscar)
	Name string `json:"name"`

	// Kubernetes namespace for the deployment and service (default: oscar)
	Namespace string `json:"namespace"`

	// Kubernetes namespace for services and jobs (default: oscar-svc)
	ServicesNamespace string `json:"services_namespace"`

	// Parameter used to check if the cluster have GPUs
	GPUAvailable bool `json:"gpu_available"`

	// Parameter used to check if the cluster have vega nodes
	InterLinkAvailable bool `json:"interLink_available"`

	// Port used for the ClusterIP k8s service (default: 8080)
	ServicePort int `json:"-"`

	// Serverless framework used to deploy services (Openfaas | Knative)
	// If not defined only async invocations allowed (Using KubeBackend)
	ServerlessBackend string `json:"serverless_backend,omitempty"`

	// OpenfaasNamespace namespace where the OpenFaaS gateway is deployed
	OpenfaasNamespace string `json:"-"`

	// OpenfaasPort service port where the OpenFaaS gateway is exposed
	OpenfaasPort int `json:"-"`

	// OpenfaasBasicAuthSecret name of the secret used to store the OpenFaaS credentials
	OpenfaasBasicAuthSecret string `json:"-"`

	// OpenfaasPrometheusPort service port where the OpenFaaS' Prometheus is exposed
	OpenfaasPrometheusPort int `json:"-"`

	// OpenfaasScalerEnable option to enable the Openfaas scaler
	OpenfaasScalerEnable bool `json:"-"`

	// OpenfaasScalerInterval time interval to check if any function could be scaled
	OpenfaasScalerInterval string `json:"-"`

	// OpenfaasScalerInactivityDuration
	OpenfaasScalerInactivityDuration string `json:"-"`

	// WatchdogMaxInflight
	WatchdogMaxInflight int `json:"-"`

	// WatchdogWriteDebug
	WatchdogWriteDebug bool `json:"-"`

	// WatchdogExecTimeout
	WatchdogExecTimeout int `json:"-"`

	// WatchdogReadTimeout
	WatchdogReadTimeout int `json:"-"`

	// WatchdogWriteTimeout
	WatchdogWriteTimeout int `json:"-"`

	// WatchdogHealthCheckInterval
	WatchdogHealthCheckInterval int `json:"-"`

	// HTTP timeout for reading the payload (default: 300)
	ReadTimeout time.Duration `json:"-"`

	// HTTP timeout for writing the response (default: 300)
	WriteTimeout time.Duration `json:"-"`

	// YunikornEnable option to configure Apache Yunikorn
	YunikornEnable bool `json:"yunikorn_enable"`

	// YunikornNamespace
	YunikornNamespace string `json:"-"`

	// YunikornConfigMap
	YunikornConfigMap string `json:"-"`

	// YunikornConfigFileName
	YunikornConfigFileName string `json:"-"`

	// ResourceManagerEnable option to enable the Resource Manager to delegate jobs
	// when there are no available resources in the cluster (if the service has replicas)
	ResourceManagerEnable bool `json:"-"`

	// // ResourceManager parameter to set the ResourceManager to use ("kubernetes" or "yunikorn")
	// // TODO: decide if this parameter is necessary or use kubernetes by default and yunikorn always if it's enabled
	// ResourceManager string `json:"-"`

	// ResourceManagerInterval time interval (in seconds) to update the available resources in the cluster
	ResourceManagerInterval int `json:"-"`

	// ReSchedulerEnable option to enable the ReScheduler to delegate jobs to a replica
	// when a threshold is reached
	ReSchedulerEnable bool `json:"-"`

	// ReSchedulerInterval time interval (in seconds) to check if pending jobs
	ReSchedulerInterval int `json:"-"`

	// ReSchedulerThreshold default time (in seconds) that a job (with replicas) can be queued before delegating it
	ReSchedulerThreshold int `json:"-"`

	// OIDCEnable parameter to enable OIDC support
	OIDCEnable bool `json:"-"`

	// OIDCValidIssuers List of allowed providers to authenticate
	OIDCValidIssuers []string `json:"-"`

	// OIDCSubject OpenID Connect Subject (user identifier)
	OIDCSubject string `json:"-"`

	// OIDCGroups OpenID comma-separated group list to grant access in the cluster.
	// Groups defined in the "eduperson_entitlement" OIDC scope,
	// as described here: https://docs.egi.eu/providers/check-in/sp/#10-groups
	OIDCGroups []string `json:"oidc_groups"`

	UsersAdmin []string `json:"-"`

	//
	IngressHost string `json:"-"`

	// Github path of FaaS Supervisor (needed for Interlink config)
	SupervisorKitImage string `json:"-"`

	// Ingress CORS allowed origins for exposed services
	IngressServicesCORSAllowedOrigins string `json:"-"`

	// Ingress CORS allowed methods for exposed services
	IngressServicesCORSAllowedMethods string `json:"-"`

	// Ingress CORS allowed headers for exposed services
	IngressServicesCORSAllowedHeaders string `json:"-"`

	//Time to Life of job SecondsAfterFinished
	TTLJob int `json:"-"`

	//Job listing limit
	JobListingLimit int `json:"-"`

	allowedImageReposMutex   sync.RWMutex `json:"-"`
	AllowedImageRepositories []string     `json:"allowed_image_repositories"`
}

var configVars = []configVar{
	{"Username", "OSCAR_USERNAME", true, stringType, ""},
	{"Password", "OSCAR_PASSWORD", true, stringType, ""},
	{"MinIOProvider.AccessKey", "MINIO_ACCESS_KEY", true, stringType, ""},
	{"MinIOProvider.SecretKey", "MINIO_SECRET_KEY", true, stringType, ""},
	{"MinIOProvider.Region", "MINIO_REGION", false, stringType, "us-east-1"},
	{"MinIOProvider.Verify", "MINIO_TLS_VERIFY", false, boolType, "true"},
	{"MinIOProvider.Endpoint", "MINIO_ENDPOINT", false, urlType, "https://minio-service.minio:9000"},
	{"Name", "OSCAR_NAME", false, stringType, "oscar"},
	{"Namespace", "OSCAR_NAMESPACE", false, stringType, "oscar"},
	{"ServicesNamespace", "OSCAR_SERVICES_NAMESPACE", false, stringType, "oscar-svc"},
	{"ServerlessBackend", "SERVERLESS_BACKEND", false, serverlessBackendType, ""},
	{"OpenfaasNamespace", "OPENFAAS_NAMESPACE", false, stringType, "openfaas"},
	{"OpenfaasPort", "OPENFAAS_PORT", false, intType, "8080"},
	{"OpenfaasBasicAuthSecret", "OPENFAAS_BASIC_AUTH_SECRET", false, stringType, "basic-auth"},
	{"OpenfaasPrometheusPort", "OPENFAAS_PROMETHEUS_PORT", false, intType, "9090"},
	{"OpenfaasScalerEnable", "OPENFAAS_SCALER_ENABLE", false, boolType, "false"},
	{"OpenfaasScalerInterval", "OPENFAAS_SCALER_INTERVAL", false, stringType, "2m"},
	{"OpenfaasScalerInactivityDuration", "OPENFAAS_SCALER_INACTIVITY_DURATION", false, stringType, "10m"},
	{"WatchdogMaxInflight", "WATCHDOG_MAX_INFLIGHT", false, intType, "1"},
	{"WatchdogWriteDebug", "WATCHDOG_WRITE_DEBUG", false, boolType, "true"},
	{"WatchdogExecTimeout", "WATCHDOG_EXEC_TIMEOUT", false, intType, "0"},
	{"WatchdogReadTimeout", "WATCHDOG_READ_TIMEOUT", false, intType, "300"},
	{"WatchdogWriteTimeout", "WATCHDOG_WRITE_TIMEOUT", false, intType, "300"},
	{"WatchdogHealthCheckInterval", "WATCHDOG_HEALTHCHECK_INTERVAL", false, intType, "5"},
	{"ReadTimeout", "READ_TIMEOUT", false, secondsType, "300"},
	{"WriteTimeout", "WRITE_TIMEOUT", false, secondsType, "300"},
	{"ServicePort", "OSCAR_SERVICE_PORT", false, intType, "8080"},
	{"YunikornEnable", "YUNIKORN_ENABLE", false, boolType, "false"},
	{"YunikornNamespace", "YUNIKORN_NAMESPACE", false, stringType, "yunikorn"},
	{"YunikornConfigMap", "YUNIKORN_CONFIGMAP", false, stringType, "yunikorn-configs"},
	{"YunikornConfigFileName", "YUNIKORN_CONFIG_FILENAME", false, stringType, "queues.yaml"},
	{"ResourceManagerEnable", "RESOURCE_MANAGER_ENABLE", false, boolType, "false"},
	//{"ResourceManager", "RESOURCE_MANAGER", false, resourceManagerType, "kubernetes"},
	{"ResourceManagerInterval", "RESOURCE_MANAGER_INTERVAL", false, intType, "15"},
	{"ReSchedulerEnable", "RESCHEDULER_ENABLE", false, boolType, "false"},
	{"ReSchedulerInterval", "RESCHEDULER_INTERVAL", false, intType, "15"},
	{"ReSchedulerThreshold", "RESCHEDULER_THRESHOLD", false, intType, "30"},
	{"OIDCEnable", "OIDC_ENABLE", false, boolType, "false"},
	{"OIDCValidIssuers", "OIDC_ISSUERS", false, stringSliceType, ""},
	{"OIDCSubject", "OIDC_SUBJECT", false, stringType, ""},
	{"OIDCGroups", "OIDC_GROUPS", false, stringSliceType, ""},
	{"UsersAdmin", "USERS_ADMIN", false, stringSliceType, ""},
	{"IngressHost", "INGRESS_HOST", false, stringType, ""},
	{"SupervisorKitImage", "SUPERVISOR_KIT_IMAGE", false, stringType, ""},
	{"IngressServicesCORSAllowedOrigins", "INGRESS_SERVICES_CORS_ALLOWED_ORIGINS", false, stringType, "https://dashboard.oscar.grycap.net,https://dashboard-devel.oscar.grycap.net,https://dashboard-demo.oscar.grycap.net,http://oscar.oscar.svc.cluster.local,http://host.docker.internal,http://localhost,http://localhost:5173"},
	{"IngressServicesCORSAllowedMethods", "INGRESS_SERVICES_CORS_ALLOWED_METHODS", false, stringType, "GET, PUT, POST, DELETE, PATCH, HEAD"},
	{"IngressServicesCORSAllowedHeaders", "INGRESS_SERVICES_CORS_ALLOWED_HEADERS", false, stringType, "Authorization, Content-Type"},
	{"TTLJob", "TTL_JOB", false, intType, "2592000"},
	{"JobListingLimit", "JOB_LISTING_LIMIT", false, intType, "70"},
	{"AllowedImageRepositories", "ALLOWED_IMAGE_REPOSITORIES", false, stringSliceType, ""},
}

func readConfigVar(cfgVar configVar) (string, error) {
	value := os.Getenv(cfgVar.envVarName)
	if len(value) == 0 {
		if cfgVar.required {
			return "", fmt.Errorf("the configuration variable %s must be provided", cfgVar.envVarName)
		}
		value = cfgVar.defaultValue
	}
	return value, nil
}

func setValue(value any, configField string, cfg *Config) {
	// Check if there if the field is inside a substruct
	fields := strings.Split(configField, ".")
	if len(fields) > 2 {
		log.Fatalf("cannot access field %s", configField)
	}

	// Get the reflect value of cfg (pointer)
	valPtr := reflect.ValueOf(cfg)
	// Get the reflect value of the cfg struct
	valCfg := reflect.Indirect(valPtr).FieldByName(fields[0])

	// If there is a subfield get its value
	if len(fields) == 2 {
		valCfg = reflect.Indirect(valCfg).FieldByName(fields[1])
	}

	// Set the value
	valCfg.Set(reflect.ValueOf(value))
}

func parseStringSlice(s string) []string {
	strs := []string{}

	// Split by commas
	vals := strings.Split(s, ",")

	// Trim spaces and append
	for _, v := range vals {
		strs = append(strs, strings.TrimSpace(v))
	}

	return strs
}

func parseSeconds(s string) (time.Duration, error) {
	if len(s) > 0 {
		parsed, err := strconv.Atoi(s)
		if err == nil && parsed > 0 {
			return time.Duration(parsed) * time.Second, nil
		}
	}
	return time.Duration(0), fmt.Errorf("the value must be a positive integer")
}

func parseServerlessBackend(s string) (string, error) {
	if len(s) > 0 {
		str := strings.ToLower(s)
		if str != OpenFaaSBackend && str != KnativeBackend {
			return "", fmt.Errorf("must be \"Openfaas\" or \"Knative\"")
		}
		return str, nil
	}
	return s, nil
}

// ReadConfig reads environment variables to create the OSCAR server configuration
func ReadConfig() (*Config, error) {
	config := &Config{}
	config.MinIOProvider = &MinIOProvider{}

	for _, cv := range configVars {
		var value any
		var parseErr error
		strValue, err := readConfigVar(cv)
		if err != nil {
			return nil, err
		}

		// Parse the environment variable depending of its type
		switch cv.varType {
		case stringType:
			value = strings.TrimSpace(strValue)
		case stringSliceType:
			value = parseStringSlice(strValue)
		case intType:
			value, parseErr = strconv.Atoi(strValue)
		case boolType:
			value, parseErr = strconv.ParseBool(strValue)
		case secondsType:
			value, parseErr = parseSeconds(strValue)
		case serverlessBackendType:
			value, parseErr = parseServerlessBackend(strValue)
		case urlType:
			// Only check if can be parsed
			_, parseErr = url.Parse(strValue)
			value = strValue
		default:
			continue
		}

		// If there are some parseErr return error
		if parseErr != nil {
			return nil, fmt.Errorf("the %s value is not valid. Expected type: %s. Error: %v", cv.envVarName, cv.varType, parseErr)
		}

		// Set the value in the Config struct
		setValue(value, cv.name, config)

	}

	// Normalize allowed image repositories after parsing environment variables
	if len(config.AllowedImageRepositories) > 0 {
		if err := config.SetAllowedImageRepositories(config.AllowedImageRepositories); err != nil {
			return nil, err
		}
	}

	return config, nil
}

// SetAllowedImageRepositories atomically updates the allowed Docker image repositories.
// Entries are normalized to lowercase canonical references (registry[/path]).
func (cfg *Config) SetAllowedImageRepositories(repos []string) error {
	cfg.allowedImageReposMutex.Lock()
	defer cfg.allowedImageReposMutex.Unlock()

	cleaned := make([]string, 0, len(repos))
	for _, repo := range repos {
		repo = strings.TrimSpace(repo)
		if repo == "" {
			continue
		}
		normalized, err := normalizeAllowedRepositoryEntry(repo)
		if err != nil {
			return err
		}
		cleaned = append(cleaned, normalized)
	}

	cfg.AllowedImageRepositories = cleaned
	return nil
}

// GetAllowedImageRepositories returns a copy of the allowed Docker image repositories.
func (cfg *Config) GetAllowedImageRepositories() []string {
	cfg.allowedImageReposMutex.RLock()
	defer cfg.allowedImageReposMutex.RUnlock()
	if len(cfg.AllowedImageRepositories) == 0 {
		return nil
	}
	repos := make([]string, len(cfg.AllowedImageRepositories))
	copy(repos, cfg.AllowedImageRepositories)
	return repos
}

const defaultImageRegistry = "docker.io"

func normalizeAllowedRepositoryEntry(repo string) (string, error) {
	registry, repository, err := parseAllowedRepository(repo)
	if err != nil {
		return "", err
	}

	if repository == "" {
		return registry, nil
	}
	return fmt.Sprintf("%s/%s", registry, repository), nil
}

func parseAllowedRepository(repo string) (string, string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", "", fmt.Errorf("allowed repository cannot be empty")
	}

	repo = strings.ToLower(repo)
	repo = strings.TrimSuffix(repo, "/")
	if repo == "" {
		return "", "", fmt.Errorf("allowed repository cannot be empty")
	}

	if strings.Contains(repo, "://") {
		return "", "", fmt.Errorf("allowed repository must not include a URI scheme")
	}

	if !strings.Contains(repo, "/") {
		return repo, "", nil
	}

	firstSlash := strings.Index(repo, "/")
	if firstSlash == 0 {
		return "", "", fmt.Errorf("allowed repository is not valid")
	}
	firstSegment := repo[:firstSlash]
	rest := repo[firstSlash+1:]

	if rest == "" {
		return firstSegment, "", nil
	}

	if strings.Contains(firstSegment, ".") || strings.Contains(firstSegment, ":") || firstSegment == "localhost" {
		return firstSegment, rest, nil
	}

	return defaultImageRegistry, repo, nil
}

func parseImageReference(image string) (string, string, error) {
	image = strings.TrimSpace(image)
	if image == "" {
		return "", "", fmt.Errorf("image reference cannot be empty")
	}

	image = strings.ToLower(image)
	if idx := strings.Index(image, "@"); idx != -1 {
		image = image[:idx]
	}

	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > -1 && (lastSlash == -1 || lastColon > lastSlash) {
		image = image[:lastColon]
	}

	image = strings.TrimSuffix(image, "/")
	if image == "" {
		return "", "", fmt.Errorf("image reference cannot be empty")
	}

	if !strings.Contains(image, "/") {
		return defaultImageRegistry, fmt.Sprintf("library/%s", image), nil
	}

	firstSlash := strings.Index(image, "/")
	firstSegment := image[:firstSlash]
	rest := image[firstSlash+1:]

	if rest == "" {
		return firstSegment, "", nil
	}

	if strings.Contains(firstSegment, ".") || strings.Contains(firstSegment, ":") || firstSegment == "localhost" {
		return firstSegment, rest, nil
	}

	return defaultImageRegistry, image, nil
}

// IsImageRepositoryAllowed returns true if the provided image reference matches any of the allowed repositories.
func (cfg *Config) IsImageRepositoryAllowed(image string) bool {
	repos := cfg.GetAllowedImageRepositories()
	if len(repos) == 0 {
		return true
	}

	registry, repository, err := parseImageReference(image)
	if err != nil {
		return false
	}

	for _, allowed := range repos {
		allowedRegistry, allowedRepo, err := parseAllowedRepository(allowed)
		if err != nil {
			continue
		}

		if allowedRegistry != "" && allowedRegistry != registry {
			continue
		}

		if allowedRepo == "" {
			return true
		}

		if strings.HasPrefix(repository, allowedRepo) {
			if len(repository) == len(allowedRepo) {
				return true
			}
			if repository[len(allowedRepo)] == '/' {
				return true
			}
		}
	}

	return false
}

// ValidateImageRepository validates if an image reference is allowed and returns a descriptive error otherwise.
func (cfg *Config) ValidateImageRepository(image string) error {
	if cfg.IsImageRepositoryAllowed(image) {
		return nil
	}
	return fmt.Errorf("image %q is not allowed on this OSCAR cluster", image)
}

// Clone returns a shallow copy of the configuration with a safe copy of mutable slices.
func (cfg *Config) Clone() *Config {
	if cfg == nil {
		return nil
	}

	cfg.allowedImageReposMutex.RLock()
	defer cfg.allowedImageReposMutex.RUnlock()

	clone := *cfg
	if len(cfg.AllowedImageRepositories) > 0 {
		clone.AllowedImageRepositories = make([]string, len(cfg.AllowedImageRepositories))
		copy(clone.AllowedImageRepositories, cfg.AllowedImageRepositories)
	}

	if len(cfg.OIDCValidIssuers) > 0 {
		clone.OIDCValidIssuers = append([]string(nil), cfg.OIDCValidIssuers...)
	}
	if len(cfg.OIDCGroups) > 0 {
		clone.OIDCGroups = append([]string(nil), cfg.OIDCGroups...)
	}
	if len(cfg.UsersAdmin) > 0 {
		clone.UsersAdmin = append([]string(nil), cfg.UsersAdmin...)
	}

	return &clone
}

// CheckAvailableGPUs checks if there are "nvidia.com/gpu" resources in the cluster
func (cfg *Config) CheckAvailableGPUs(kubeClientset kubernetes.Interface) {
	nodes, err := kubeClientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "!node-role.kubernetes.io/control-plane,!node-role.kubernetes.io/master"})
	if err != nil {
		log.Printf("Error getting list of nodes: %v\n", err)
	}
	for _, node := range nodes.Items {
		gpu := node.Status.Allocatable["nvidia.com/gpu"]
		if gpu.Value() > 0 {
			cfg.GPUAvailable = true
			return
		}
	}
}

// CheckAvailableInterLink checks if there is a node with the virtual kubelet annotation
func (cfg *Config) CheckAvailableInterLink(kubeClientset kubernetes.Interface) {
	nodes, err := kubeClientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "!node-role.kubernetes.io/control-plane,!node-role.kubernetes.io/master,type=virtual-kubelet"})
	if err != nil {
		log.Printf("Error getting list of nodes: %v\n", err)
	}
	if len(nodes.Items) > 0 {
		cfg.InterLinkAvailable = true
		log.Printf("INFO: InterLink Available")
	} else {
		cfg.InterLinkAvailable = false
		log.Printf("INFO: InterLink Unavailable")
	}
	//cfg.InterLinkAvailable = true

}
