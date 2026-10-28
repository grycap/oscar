package types

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	knv1 "knative.dev/serving/pkg/apis/serving/v1"
)

const (
	kserveIsvcSuffix           = "-predictor"
	kserveIsvcPodDplSuffix     = "-predictor"
	kserveLLMIsvcSuffix        = "-kserve-workload-svc"
	kserveLLMIsvcPodDplSuffix  = "-kserve"
	kserveKeyLabelApp          = "oscar-app"
	prefixLabelApp             = "oscar-svc-ksv-"
	kserveISVCContainerName    = "kserve-container"
	kserveLLMISVCLabelKey      = "app.kubernetes.io/part-of"
	kserveLLMISVCLabelValue    = "llminferenceservice"
	kserveLLMISVCContainerName = "main"
)

var kserveLogger = log.New(os.Stdout, "[KSERVE-SERVICE] ", log.Flags())

const (
	//	defaultAuthMiddlewareName = "oidc-auth"
	httpRouteSuffix      = "-route"
	authMiddlewareSuffix = "-auth-mdw"
	authSecretSuffix     = "-auth-traefik" // #nosec G101
	corsMiddlewareSuffix = "-cors-mdw"
	defaultLLMCPUimage   = "ghcr.io/grycap/kserve-llm-d-cpu:latest"
	defaultLLMGPUimage   = "ghcr.io/grycap/kserve-llm-d-cuda:latest"
	kserveCpuOverhead    = "10m"
	kserveMemoryOverhead = "15Mi"
)

var (
	llmInferenceServiceGVR     = schema.GroupVersionResource{Group: "serving.kserve.io", Version: "v1alpha1", Resource: "llminferenceservices"}
	kserveIsvcGVR              = schema.GroupVersionResource{Group: "serving.kserve.io", Version: "v1beta1", Resource: "inferenceservices"}
	kserveHTTPRouteGVR         = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
	kserveTraefikMiddlewareGVR = schema.GroupVersionResource{Group: "traefik.io", Version: "v1alpha1", Resource: "middlewares"}
	// kserveSecretGVR            = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	defaultKserveCpuRequest    = resource.MustParse(DefaultServiceCPU)
	defaultKserveMemoryRequest = resource.MustParse(DefaultServiceMemory)
)

// Explicitly check that KserveInference and KserveLLMInference implement the needed interface
var _ ModelServingSpec = (*KserveInference)(nil)
var _ ModelServingAuthSpec = (*KserveInference)(nil)
var _ ModelServingScalableSpec = (*KserveInference)(nil)
var _ ModelServingSpec = (*KserveLLMInference)(nil)
var _ ModelServingAuthSpec = (*KserveLLMInference)(nil)
var _ ModelServingScalableSpec = (*KserveLLMInference)(nil)

func init() {
	RegisterModelServingSpec(kserveTypeInferenceService, func(raw json.RawMessage) (ModelServingSpec, error) {
		spec := &KserveInference{}
		if err := json.Unmarshal(raw, spec); err != nil {
			return nil, err
		}
		return spec, nil
	})
	RegisterModelServingSpec(kserveTypeLLMInferenceService, func(raw json.RawMessage) (ModelServingSpec, error) {
		spec := &KserveLLMInference{}
		if err := json.Unmarshal(raw, spec); err != nil {
			return nil, err
		}
		return spec, nil
	})
}

func kserveLabelSelector(serviceName string) string {
	return fmt.Sprintf("%s=%s", kserveKeyLabelApp, prefixLabelApp+serviceName)
}

// KserveInference represents the configuration for a KServe Inference Service.
// It supports various model formats and runtimes, allowing users to deploy machine learning models with KServe.
// It targets prediction models
type KserveInference struct {
	// ModelFormat the model format to use for KServe InferenceService.
	// Every model format has its own runtime and the available runtimes may vary depending on the cluster configuration.
	// ("onnx", "sklearn", "xgboost", "pytorch", "tensorflow", "triton", "huggingface").
	ModelFormat string `json:"model_format,omitempty"`

	// RuntimeImage the KServe runtime to use. Its a custom runtime if
	// the available runtimes in the model format are not enough
	// Ref: https://kserve.github.io/website/docs/concepts/resources/servingruntime
	// Optional.
	RuntimeImage string `json:"runtime_image,omitempty"`

	// Can be used to specify the protocol version for KServe (e.g., "v1", "v2").
	// Optional. (default: "v1")
	APIVersion string `json:"api_version,omitempty" default:"v1"`

	// Source the URI of the model storage for KServe
	// Required. It should follow the format expected by KServe, for example:
	Source string `json:"source"`

	// MinScale minimum number of active replicas (pods) for the service
	// Optional. (default: 1)
	MinScale int32 `json:"min_scale"`

	// MaxScale maximum number of active replicas (pods) for the service
	// Optional. (default: 1)
	MaxScale int32 `json:"max_scale"`

	// Args command-line arguments to be passed to the container
	// Optional
	Args []string `json:"args,omitempty"`

	// SetAuth parameter to set the authentication for the KServe InferenceService
	// Optional. (default: true)
	SetAuth bool `json:"set_auth"`
}

// UnmarshalJSON sets the defaults for omitted fields. Validation is done by Validate.
func (k *KserveInference) UnmarshalJSON(data []byte) error {
	type Alias KserveInference

	aux := Alias{
		APIVersion: "v1",
		MinScale:   1,
		MaxScale:   1,
		SetAuth:    true,
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*k = KserveInference(aux)
	return nil
}

func (k KserveInference) validate() error {
	if strings.TrimSpace(k.ModelFormat) == "" {
		return fmt.Errorf("ModelFormat is required")
	}

	if strings.TrimSpace(k.Source) == "" {
		return fmt.Errorf("model Source is required")
	}

	switch k.APIVersion {
	case "v1", "v2":
		return nil
	default:
		return fmt.Errorf("invalid API version %q, expected v1 or v2", k.APIVersion)
	}
}

func (k KserveInference) Type() string {
	return kserveTypeInferenceService
}

func (k KserveInference) Equal(other KserveInference) bool {
	return k.ModelFormat == other.ModelFormat &&
		k.RuntimeImage == other.RuntimeImage &&
		k.APIVersion == other.APIVersion
}

func (k KserveInference) validateUpdate(old ModelServingSpec) error {
	oldInference, ok := old.(*KserveInference)
	if !ok {
		return fmt.Errorf("cannot change ModelServing service type from %q to %q", old.Type(), k.Type())
	}

	err := k.validate()
	if err != nil {
		return fmt.Errorf("invalid new definition for KServe Inference update: %v", err)
	}

	if oldInference.RuntimeImage != k.RuntimeImage {
		return fmt.Errorf("cannot update runtime for ModelServing InferenceService")
	}
	if oldInference.ModelFormat != k.ModelFormat {
		return fmt.Errorf("cannot update model format for ModelServing InferenceService")
	}

	return nil
}

func (k KserveInference) GetAuth() bool {
	return k.SetAuth
}

func (k KserveInference) GetScaling() ModelServingScalable {
	return ModelServingScalable{
		MinScale: k.MinScale,
		MaxScale: k.MaxScale,
	}
}

func (k KserveInference) svcName(serviceName string) string {
	return serviceName + kserveIsvcSuffix
}

func (k KserveInference) podAndDplName(serviceName string) string {
	return serviceName + kserveIsvcPodDplSuffix
}

func (k KserveInference) labelSelector(serviceName string) string {
	return kserveLabelSelector(serviceName)
}

func (k KserveInference) getLabelSelectorKey() string {
	return kserveKeyLabelApp
}

func (k KserveInference) getLabelSelectorValue(serviceName string) string {
	return prefixLabelApp + serviceName
}

func (k KserveInference) getClusterInternalURI(modelServingServiceName string, namespace string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", k.svcName(modelServingServiceName), namespace)
}

func (k KserveInference) getConteinerName() string {
	return kserveISVCContainerName
}

func (k KserveInference) getKueueResourceList(service *Service) (request corev1.ResourceList, minScale int32, err error) {
	requests := v1.ResourceList{}
	kserveResourcesRequests, err := createKserveResources(service)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create Kserve resources for Kueue: %w", err)
	}
	requests = kserveResourcesRequests.Requests
	var kserveMinScale int32 = 1
	if scaling, ok := service.ModelServing.GetScaling(); ok && scaling.MinScale > 1 {
		kserveMinScale = scaling.MinScale
	}

	return requests, kserveMinScale, nil
}

func (k KserveInference) createModelServingService(service *Service, knativeService *knv1.Service, cfg *Config) error {
	if cfg == nil {
		panic("CreateKserveService: cfg is nil")
	}
	dynClient, err := getDynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}
	var owner *KserveServiceOwner = nil

	kserveLogger.Printf("Creating KServe Inference service '%s' for user '%s' with model format %s", service.Name, service.Owner, service.ModelServing.Inference().ModelFormat)
	kserveSvc, err := createKserveInferenceService(dynClient, service, owner, cfg)
	if err != nil {
		return fmt.Errorf("failed to create InferenceService: %v", err)
	}

	kserveLogger.Printf("Exposing KServe service '%s' for user '%s'", service.Name, service.Owner)
	err = exposeKserveService(dynClient, service, buildUnstructuredServiceOwnerRef(kserveSvc), cfg)
	if err != nil {
		// If exposing the service fails, delete the created InferenceService to avoid orphaned resources
		kserveLogger.Printf("Failed to expose KServe service '%s' for user '%s'", service.Name, service.Owner)
		deleteErr := k.deleteModelServingService(service, service.Namespace)
		if deleteErr != nil {
			return fmt.Errorf("failed to expose KServe Service: %v; additionally, failed to delete KServe Service: %v", err, deleteErr)
		}
		return fmt.Errorf("failed to expose KServe Service: %v", err)
	}
	return nil
}

func (k KserveInference) updateModelServingService(service *Service, oldService *Service, namespace string) error {
	dynClient, err := getDynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	kserveLogger.Printf("Updating KServe Inference service '%s' for user '%s'", service.Name, service.Owner)
	oldKserveSvc, err := getKserveInferenceService(dynClient, service.Name, namespace)
	if err != nil {
		return fmt.Errorf("failed to get existing InferenceService: %v", err)
	}
	err = updateKserveInferenceService(dynClient, service, oldKserveSvc)
	if err != nil {
		return fmt.Errorf("failed to update InferenceService: %v", err)
	}

	return nil
}

func (k KserveInference) deleteModelServingService(service *Service, namespace string) error {
	dynClient, err := getDynamicClient()
	if err != nil {
		return err
	}
	kserveLogger.Printf("Deleting KServe service '%s' for user '%s'", service.Name, service.Owner)
	err = dynClient.Resource(kserveIsvcGVR).Namespace(namespace).Delete(context.Background(), buildKserveName(service.Name), metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Kserve service: %v", err)
	}
	return nil
}

// KserveLLMInference represents the configuration for a KServe LLM Inference Service.
// It targets large language models (LLMs) using vLLM as the runtime
type KserveLLMInference struct {
	// At the moment only supported for LLMInferenceService,
	// the runtime image to use for KServe when IsLLM is true
	// Optional. (default: a custom image based on vLLM for CPU and another one with GPU support)
	RuntimeImage string `json:"runtime_image,omitempty"`

	// Source the URI of the model storage for KServe
	// Required. It should follow the format expected by KServe, for example:
	Source string `json:"source"`

	// MinScale minimum number of active replicas (pods) for the service
	// Optional. (default: 1)
	MinScale int32 `json:"min_scale"`

	// MaxScale maximum number of active replicas (pods) for the service
	// Optional. (default: 1)
	MaxScale int32 `json:"max_scale"`

	// Args command-line arguments to be passed to the container
	// Optional
	Args []string `json:"args,omitempty"`

	// SetAuth parameter to set the authentication for the KServe InferenceService
	// Optional. (default: true)
	SetAuth bool `json:"set_auth"`
}

// UnmarshalJSON sets the defaults for omitted fields. Validation is done by Validate.
func (k *KserveLLMInference) UnmarshalJSON(data []byte) error {
	type Alias KserveLLMInference

	aux := Alias{
		MinScale: 1,
		MaxScale: 1,
		SetAuth:  true,
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*k = KserveLLMInference(aux)
	return nil
}

func (k KserveLLMInference) validate() error {
	if strings.TrimSpace(k.Source) == "" {
		return fmt.Errorf("model Source is required")
	}

	return nil
}

func (k KserveLLMInference) Type() string {
	return kserveTypeLLMInferenceService
}

func (k KserveLLMInference) validateUpdate(old ModelServingSpec) error {
	oldLLMInference, ok := old.(*KserveLLMInference)
	if !ok {
		return fmt.Errorf("cannot change ModelServing service type from %q to %q", old.Type(), k.Type())
	}

	err := k.validate()
	if err != nil {
		return fmt.Errorf("invalid new definition for KServe LLM update: %v", err)
	}
	if oldLLMInference.RuntimeImage != k.RuntimeImage {
		return fmt.Errorf("cannot update runtime for KServe LLM")
	}

	return nil
}

func (k KserveLLMInference) GetAuth() bool {
	return k.SetAuth
}

func (k KserveLLMInference) GetScaling() ModelServingScalable {
	return ModelServingScalable{
		MinScale: k.MinScale,
		MaxScale: k.MaxScale,
	}
}

func (k KserveLLMInference) Equal(other KserveLLMInference) bool {
	return k.RuntimeImage == other.RuntimeImage
}

func (k KserveLLMInference) svcName(serviceName string) string {
	return serviceName + kserveLLMIsvcSuffix
}

func (k KserveLLMInference) podAndDplName(serviceName string) string {
	return serviceName + kserveLLMIsvcPodDplSuffix
}

func (k KserveLLMInference) labelSelector(serviceName string) string {
	return kserveLabelSelector(serviceName)
}

func (k KserveLLMInference) getLabelSelectorKey() string {
	return kserveKeyLabelApp
}

func (k KserveLLMInference) getLabelSelectorValue(serviceName string) string {
	return prefixLabelApp + serviceName
}

func (k KserveLLMInference) getClusterInternalURI(serviceName string, namespace string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", k.svcName(serviceName), namespace)
}

func (k KserveLLMInference) getConteinerName() string {
	return kserveLLMISVCContainerName
}

func (k KserveLLMInference) getKueueResourceList(service *Service) (request corev1.ResourceList, minScale int32, err error) {
	requests := v1.ResourceList{}
	kserveResourcesRequests, err := createKserveResources(service)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create Kserve resources for Kueue: %w", err)
	}
	requests = kserveResourcesRequests.Requests
	var kserveMinScale int32 = 1
	if scaling, ok := service.ModelServing.GetScaling(); ok && scaling.MinScale > 1 {
		kserveMinScale = scaling.MinScale
	}

	return requests, kserveMinScale, nil
}

func (k KserveLLMInference) createModelServingService(service *Service, knativeService *knv1.Service, cfg *Config) error {
	if cfg == nil {
		panic("CreateKserveService: cfg is nil")
	}

	dynClient, err := getDynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	var owner *KserveServiceOwner = nil
	kserveLogger.Printf("Creating KServe LLM service '%s' for user '%s'", service.Name, service.Owner)
	kserveSvc, err := createKserveLLMInferenceService(dynClient, service, owner, cfg)
	if err != nil {
		return fmt.Errorf("failed to create KServe LLM service: %v", err)
	}

	kserveLogger.Printf("Exposing KServe service '%s' for user '%s'", service.Name, service.Owner)
	err = exposeKserveService(dynClient, service, buildUnstructuredServiceOwnerRef(kserveSvc), cfg)
	if err != nil {
		// If exposing the service fails, delete the created InferenceService to avoid orphaned resources
		kserveLogger.Printf("Failed to expose KServe service '%s' for user '%s'", service.Name, service.Owner)
		deleteErr := k.deleteModelServingService(service, service.Namespace)
		if deleteErr != nil {
			return fmt.Errorf("failed to expose KServe Service: %v; additionally, failed to delete KServe Service: %v", err, deleteErr)
		}
		return fmt.Errorf("failed to expose KServe Service: %v", err)
	}
	return nil
}

func (k KserveLLMInference) updateModelServingService(service *Service, oldService *Service, namespace string) error {
	dynClient, err := getDynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	kserveLogger.Printf("Updating KServe LLM service '%s' for user '%s'", service.Name, service.Owner)
	oldKserveSvc, err := getKserveLLMInferenceService(dynClient, service.Name, namespace)
	if err != nil {
		return fmt.Errorf("failed to get existing LLMInferenceService: %v", err)
	}
	err = updateKserveLLMInferenceService(dynClient, service, oldKserveSvc)
	if err != nil {
		return fmt.Errorf("failed to update LLMInferenceService: %v", err)
	}

	return nil
}

func (k KserveLLMInference) deleteModelServingService(service *Service, namespace string) error {
	dynClient, err := getDynamicClient()
	if err != nil {
		return err
	}
	kserveLogger.Printf("Deleting KServe LLM service '%s' for user '%s'", service.Name, service.Owner)
	err = dynClient.Resource(llmInferenceServiceGVR).Namespace(namespace).Delete(context.Background(), buildKserveName(service.Name), metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Kserve LLM service: %v", err)
	}
	return nil
}

/*
* k8s methods
 */
type KserveServiceOwner struct {
	APIVersion string
	Name       string
	Namespace  string
	UID        k8sTypes.UID
	Kind       string
}

type dynamicClientFactory func() (*dynamic.DynamicClient, error)

var newDynamicClient dynamicClientFactory = func() (*dynamic.DynamicClient, error) {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(restCfg)
}

func getKserveInferenceService(dynClient *dynamic.DynamicClient, serviceName, namespace string) (*unstructured.Unstructured, error) {
	// Get existing object to preserve resourceVersion
	kserveSvc, err := dynClient.Resource(kserveIsvcGVR).Namespace(namespace).Get(context.Background(), buildKserveName(serviceName), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get InferenceService: %v", err)
	}
	return kserveSvc, nil
}

func getKserveLLMInferenceService(dynClient *dynamic.DynamicClient, serviceName, namespace string) (*unstructured.Unstructured, error) {
	// Get existing object to preserve resourceVersion
	kserveSvc, err := dynClient.Resource(llmInferenceServiceGVR).Namespace(namespace).Get(context.Background(), buildKserveName(serviceName), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get LLMInferenceService: %v", err)
	}
	return kserveSvc, nil
}

func updateKserveInferenceService(dynClient *dynamic.DynamicClient, service *Service, oldKserveSvc *unstructured.Unstructured) error {
	newKserveSvc, err := updateKserveInferenceServiceSpec(service, oldKserveSvc)
	if err != nil {
		return err
	}

	_, err = dynClient.Resource(kserveIsvcGVR).Namespace(newKserveSvc.GetNamespace()).Update(context.Background(), newKserveSvc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update InferenceService: %w", err)
	}
	return nil
}

func updateKserveLLMInferenceService(dynClient *dynamic.DynamicClient, service *Service, oldKserveSvc *unstructured.Unstructured) error {
	newKserveSvc, err := updateKserveLLMInferenceServiceSpec(service, oldKserveSvc)
	if err != nil {
		return err
	}
	_, err = dynClient.Resource(llmInferenceServiceGVR).Namespace(newKserveSvc.GetNamespace()).Update(context.Background(), newKserveSvc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update LLMInferenceService: %v", err)
	}
	return nil
}

func createKserveInferenceService(dynClient *dynamic.DynamicClient, service *Service, owner *KserveServiceOwner, cfg *Config) (*unstructured.Unstructured, error) {
	rawIsvc, err := newKserveInferenceServiceSpec(service, owner, cfg)
	if err != nil {
		return nil, err
	}

	kserveSvc, err := dynClient.Resource(kserveIsvcGVR).Namespace(rawIsvc.GetNamespace()).Create(context.Background(), rawIsvc, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return kserveSvc, nil
}

func createKserveLLMInferenceService(dynClient *dynamic.DynamicClient, service *Service, owner *KserveServiceOwner, cfg *Config) (*unstructured.Unstructured, error) {
	llmIsvc, err := newKserveLLMInferenceServiceSpec(service, owner, cfg)
	if err != nil {
		return nil, err
	}

	kserveSvc, err := dynClient.Resource(llmInferenceServiceGVR).Namespace(llmIsvc.GetNamespace()).Create(context.Background(), llmIsvc, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return kserveSvc, nil
}

// newKserveInferenceServiceSpec builds an unstructured InferenceService
// (serving.kserve.io/v1beta1) suitable for use with a dynamic Kubernetes client.
// It is functionally equivalent to newKserveInferenceServiceSpec.
func newKserveInferenceServiceSpec(service *Service, owner *KserveServiceOwner, cfg *Config) (*unstructured.Unstructured, error) {
	resources, err := createKserveResources(service)
	if err != nil {
		return nil, err
	}
	inference := service.ModelServing.Inference()

	modelSpec := map[string]any{
		"modelFormat":     map[string]any{"name": inference.ModelFormat},
		"storageUri":      inference.Source,
		"protocolVersion": inference.APIVersion,
	}

	if inference.RuntimeImage != "" {
		modelSpec["runtime"] = inference.RuntimeImage
	}
	// TO DO: consider if we want to inject root path for LLM services as well, and if so, how to handle the case when the framework is vllm that expects the prefix to be preserved for routing
	//injectRootPath(service)

	modelSpec["resources"] = resources
	modelSpec["args"] = inference.Args
	modelSpec["env"] = ConvertEnvVars(service.Environment.Vars)

	predictor := map[string]any{
		"model": modelSpec,
	}
	minScale, maxScale := normalizeScaleFromKserveService(service.ModelServing)
	predictor["minReplicas"] = minScale
	predictor["maxReplicas"] = maxScale

	labels := map[string]any{
		inference.getLabelSelectorKey(): inference.getLabelSelectorValue(service.Name),
		OscarUserServiceLabel:           "true",
	}

	if cfg.KueueEnable {
		localQueueName, ok := service.Labels["kueue.x-k8s.io/queue-name"]
		if ok && localQueueName != "" {
			labels["kueue.x-k8s.io/queue-name"] = localQueueName
		} else if service.Owner != DefaultOwner {
			return nil, fmt.Errorf("missing required label 'kueue.x-k8s.io/queue-name' for KServe service with Kueue enabled")
		}
	}
	namespace := service.Namespace
	if owner != nil {
		namespace = owner.Namespace
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": kserveIsvcGVR.Group + "/" + kserveIsvcGVR.Version,
		"kind":       "InferenceService",
		"metadata": map[string]any{
			"name":            buildKserveName(service.Name),
			"namespace":       namespace,
			"ownerReferences": getOwnerReference(owner),
			"labels":          labels,
		},
		"spec": map[string]any{
			"predictor": predictor,
		},
	}}, nil
}

// updateKserveInferenceServiceSpec updates the spec fields of an existing
// unstructured InferenceService object in place, preserving metadata (including resourceVersion)
// and any existing spec fields not managed by this function (e.g., canary, traffic, componentExtensionSpec).
func updateKserveInferenceServiceSpec(service *Service, oldIsvc *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	resources, err := createKserveResources(service)
	if err != nil {
		return nil, fmt.Errorf("failed to create KServe resources: %w", err)
	}
	inference := service.ModelServing.Inference()

	modelSpec := map[string]any{
		"modelFormat":     map[string]any{"name": inference.ModelFormat},
		"storageUri":      inference.Source,
		"protocolVersion": inference.APIVersion,
	}
	if inference.RuntimeImage != "" {
		modelSpec["runtime"] = inference.RuntimeImage
	}

	modelSpec["resources"] = resources
	modelSpec["args"] = inference.Args
	modelSpec["env"] = ConvertEnvVars(service.Environment.Vars)

	predictor := map[string]any{
		"model": modelSpec,
	}
	minScale, maxScale := normalizeScaleFromKserveService(service.ModelServing)
	predictor["minReplicas"] = minScale
	predictor["maxReplicas"] = maxScale

	oldLabels, ok := oldIsvc.Object["spec"].(map[string]any)["predictor"].(map[string]any)["labels"]
	if ok {
		predictor["labels"] = oldLabels
	}

	oldIsvc.Object["spec"] = map[string]any{
		"predictor": predictor,
	}
	return oldIsvc, nil
}

func newKserveLLMInferenceServiceSpec(service *Service, owner *KserveServiceOwner, cfg *Config) (*unstructured.Unstructured, error) {
	runtimeImage := defaultLLMCPUimage
	if service.EnableGPU {
		runtimeImage = defaultLLMGPUimage
	}

	llmInference := service.ModelServing.LLMInference()
	if llmInference.RuntimeImage != "" {
		runtimeImage = llmInference.RuntimeImage
	}

	// Build container spec
	container := map[string]any{
		"name":  "main",
		"image": runtimeImage,
		"securityContext": map[string]any{
			"runAsNonRoot": false,
		},
	}

	resources, err := createKserveResources(service)
	if err != nil {
		return nil, err
	}
	container["resources"] = resources
	container["args"] = llmInference.Args
	container["env"] = ConvertEnvVars(service.Environment.Vars)

	minScale, _ := normalizeScaleFromKserveService(service.ModelServing)

	labels := map[string]any{
		llmInference.getLabelSelectorKey(): llmInference.getLabelSelectorValue(service.Name),
		OscarUserServiceLabel:              "true",
	}

	if cfg.KueueEnable {
		localQueueName, ok := service.Labels["kueue.x-k8s.io/queue-name"]
		if ok && localQueueName != "" {
			labels["kueue.x-k8s.io/queue-name"] = localQueueName
		} else if service.Owner != DefaultOwner {
			return nil, fmt.Errorf("missing required label 'kueue.x-k8s.io/queue-name' for KServe service with Kueue enabled")
		}
	}
	namespace := service.Namespace
	if owner != nil {
		namespace = owner.Namespace
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": llmInferenceServiceGVR.Group + "/" + llmInferenceServiceGVR.Version,
		"kind":       "LLMInferenceService",
		"metadata": map[string]any{
			"name":            buildKserveName(service.Name),
			"namespace":       namespace,
			"ownerReferences": getOwnerReference(owner),
		},
		"spec": map[string]any{
			"model": map[string]any{
				"uri":  llmInference.Source,
				"name": service.Name,
			},
			"replicas": minScale,
			"labels":   labels,
			"template": map[string]any{
				"containers": []any{container},
			},
		},
	}}, nil
}

// updateKserveLLMInferenceServiceSpec updates the spec fields of an existing
// unstructured LLMInferenceService object in place, preserving metadata (including resourceVersion).
func updateKserveLLMInferenceServiceSpec(service *Service, oldLLMIsvc *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	runtimeImage := defaultLLMCPUimage
	if service.EnableGPU {
		runtimeImage = defaultLLMGPUimage
	}

	llmInference := service.ModelServing.LLMInference()
	if llmInference.RuntimeImage != "" {
		runtimeImage = llmInference.RuntimeImage
	}

	container := map[string]any{
		"name":  "main",
		"image": runtimeImage,
		"securityContext": map[string]any{
			"runAsNonRoot": false,
		},
	}
	resources, err := createKserveResources(service)
	if err != nil {
		return nil, err
	}
	container["resources"] = resources
	container["args"] = llmInference.Args
	container["env"] = ConvertEnvVars(service.Environment.Vars)

	minScale, _ := normalizeScaleFromKserveService(service.ModelServing)

	labels := map[string]any{}
	if oldlabels, ok := oldLLMIsvc.Object["spec"].(map[string]any)["labels"]; ok {
		labels = oldlabels.(map[string]any)
	}

	oldLLMIsvc.Object["spec"] = map[string]any{
		"model": map[string]any{
			"uri":  llmInference.Source,
			"name": service.Name,
		},
		"replicas": minScale,
		"labels":   labels,
		"template": map[string]any{
			"containers": []any{container},
		},
	}
	return oldLLMIsvc, nil
}

func getDynamicClient() (*dynamic.DynamicClient, error) {
	dynClient, err := newDynamicClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %v", err)
	}
	return dynClient, nil
}

func normalizeScaleFromKserveService(modelServing *ModelServing) (int32, int32) {
	var minScale int32 = 0
	var maxScale int32 = 1

	scaling, ok := modelServing.GetScaling()
	if !ok {
		return minScale, maxScale
	}

	if scaling.MinScale >= 1 {
		minScale = scaling.MinScale
	}
	if scaling.MaxScale >= 1 {
		maxScale = scaling.MaxScale
	}
	if minScale > maxScale {
		maxScale = minScale
	}
	if modelServing.GetType() == kserveTypeLLMInferenceService && minScale == 0 {
		minScale = 1
	}
	return minScale, maxScale
}

func buildKserveName(serviceName string) string {
	// TODO
	return serviceName
}

func getServiceAuthCookieName(serviceName string) string {
	return "oscar_service_" + strings.ReplaceAll(serviceName, "-", "_") + "_auth"
}

func getHTTPRouteBackendPort(service *Service) int64 {
	if service.ModelServing.GetType() == kserveTypeLLMInferenceService {
		return 8000
	}
	return 80
}

func exposeKserveService(dynClient *dynamic.DynamicClient, service *Service, owner *KserveServiceOwner, cfg *Config) error {
	gwName := strings.TrimSpace(cfg.HTTPRouteGatewayName)
	gwNamespace := strings.TrimSpace(cfg.HTTPRouteGatewayNamespace)
	if gwNamespace == "" || gwName == "" {
		return fmt.Errorf("gateway namespace and name must be provided in config to create HTTPRoute for KServe service")
	}

	if auth, _ := service.ModelServing.GetAuth(); auth {
		// TO DO: Create OIDC forwardAuth Traefik Middleware
		//err = createTraefikAuthMiddleware(dynClient, service, knSvc)
		err := createTraefikOIDCMiddleware(dynClient, service, owner, cfg)
		if err != nil {
			return fmt.Errorf("failed to create Auth middleware: %v", err)
		}
	}

	// Create HTTPRoute
	if err := createHTTPRoute(dynClient, service, owner, cfg); err != nil {
		return fmt.Errorf("failed to create HTTPRoute: %v", err)
	}

	return nil
}

// createTraefikOIDCMiddleware creates a Traefik Middleware of type ForwardAuth for OIDC authentication,
// which will be used in the HTTPRoute to protect the KServe service.
// TO DO: change implementation when decided how to handle authentication for KServe services
func createTraefikOIDCMiddleware(dynClient dynamic.Interface, service *Service, owner *KserveServiceOwner, cfg *Config) error {
	middleware := buildTraefikOIDCMiddlewareSpec(service, owner, cfg)

	_, err := dynClient.Resource(kserveTraefikMiddlewareGVR).Namespace(middleware.GetNamespace()).Create(context.Background(), middleware, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create OIDC middleware: %v", err)
	}
	return nil
}

func buildTraefikOIDCMiddlewareSpec(service *Service, owner *KserveServiceOwner, cfg *Config) *unstructured.Unstructured {
	authEndpointAddress := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d/system/services/%s/auth", cfg.Name, cfg.Namespace, cfg.ServicePort, service.Name)
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "traefik.io/v1alpha1",
		"kind":       "Middleware",
		"metadata": map[string]any{
			"name":            getTraefikAuthMiddlewareName(owner.Name),
			"namespace":       owner.Namespace,
			"ownerReferences": getOwnerReference(owner),
		},
		"spec": map[string]any{
			"forwardAuth": map[string]any{
				"address":                  authEndpointAddress,
				"trustForwardHeader":       true,
				"addAuthCookiesToResponse": []any{getServiceAuthCookieName(service.Name)},
				"authResponseHeaders":      []any{"Authorization"},
			},
		},
	}}
}

func getTraefikAuthMiddlewareName(serviceName string) string {
	return serviceName + "-auth-mdw"
}

// createHTTPRoute creates a Gateway API HTTPRoute to expose the KServe InferenceService.
func createHTTPRoute(gatewayClientset dynamic.Interface, service *Service, owner *KserveServiceOwner, cfg *Config) error {
	httpRoute := buildHTTPRouteSpec(service, owner, cfg)
	_, err := gatewayClientset.Resource(kserveHTTPRouteGVR).Namespace(owner.Namespace).Create(context.Background(), httpRoute, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create HTTPRoute: %v", err)
	}
	return nil
}

// buildHTTPRouteSpec builds an unstructured HTTPRoute object to expose the
// KServe InferenceService or LLMInferenceService.
func buildHTTPRouteSpec(service *Service, owner *KserveServiceOwner, cfg *Config) *unstructured.Unstructured {
	serviceName := service.Name
	httpRouteName := serviceName + httpRouteSuffix
	namespace := owner.Namespace
	apiPath := "/"
	host := fmt.Sprintf("%s.%s", serviceName, strings.TrimSpace(cfg.IngressHost))

	filters := []any{
		map[string]any{
			"type": "RequestHeaderModifier",
			"requestHeaderModifier": map[string]any{
				"set": []any{
					map[string]any{"name": "KServe-Isvc-Name", "value": serviceName},
					map[string]any{"name": "KServe-Isvc-Namespace", "value": namespace},
				},
			},
		},
	}
	if auth, _ := service.ModelServing.GetAuth(); auth {
		filters = append(filters, map[string]any{
			"type": "ExtensionRef",
			"extensionRef": map[string]any{
				"group": "traefik.io",
				"kind":  "Middleware",
				"name":  getTraefikAuthMiddlewareName(service.Name),
			},
		})
	}

	// If ExposedServicesUseSubdomainRoute is false (custom domain routing is disabled),
	// we need to rewrite the path to remove the prefix and use the IngressHost as the hostname.
	if !cfg.ExposedServicesUseSubdomainRoute {
		apiPath = getAPIPath(serviceName)
		host = strings.TrimSpace(cfg.IngressHost)
		filters = append(filters, map[string]any{
			"type": "URLRewrite",
			"urlRewrite": map[string]any{
				"path": map[string]any{
					"type":               "ReplacePrefixMatch",
					"replacePrefixMatch": "/",
				},
			},
		})
	}

	rule := map[string]any{
		"matches": []any{
			map[string]any{
				"path": map[string]any{
					"type":  "PathPrefix",
					"value": apiPath,
				},
			},
		},
		"filters": filters,
		"backendRefs": []any{
			map[string]any{
				"group":     "",
				"kind":      "Service",
				"name":      service.ModelServing.GetSvcName(serviceName),
				"namespace": namespace,
				"port":      getHTTPRouteBackendPort(service),
			},
		},
	}

	spec := map[string]any{
		"rules": []any{rule},
	}

	if host != "" {
		spec["hostnames"] = []any{host}
	}

	parentRef := map[string]any{
		"group":     "gateway.networking.k8s.io",
		"kind":      "Gateway",
		"name":      strings.TrimSpace(cfg.HTTPRouteGatewayName),
		"namespace": strings.TrimSpace(cfg.HTTPRouteGatewayNamespace),
	}
	spec["parentRefs"] = []any{parentRef}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]any{
			"name":            httpRouteName,
			"namespace":       namespace,
			"ownerReferences": getOwnerReference(owner),
		},
		"spec": spec,
	}}
}

func createKserveResources(service *Service) (v1.ResourceRequirements, error) {
	resources := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    defaultKserveCpuRequest,
			corev1.ResourceMemory: defaultKserveMemoryRequest,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    defaultKserveCpuRequest,
			corev1.ResourceMemory: defaultKserveMemoryRequest,
		},
	}

	if len(service.CPU) > 0 {
		cpu, err := resource.ParseQuantity(service.CPU)
		if err != nil {
			return resources, err
		}
		// Substract 10m from the requested CPU due to KServe modelcar overhead
		cpu.Sub(resource.MustParse(kserveCpuOverhead))
		resources.Limits[corev1.ResourceCPU] = cpu
		resources.Requests[corev1.ResourceCPU] = cpu
	}

	if len(service.Memory) > 0 {
		memory, err := resource.ParseQuantity(service.Memory)
		if err != nil {
			return resources, err
		}
		// Substract 15Mi from the requested memory due to KServe modelcar overhead
		memory.Sub(resource.MustParse(kserveMemoryOverhead))
		resources.Limits[corev1.ResourceMemory] = memory
		resources.Requests[corev1.ResourceMemory] = memory
	}

	if service.EnableGPU {
		gpu, err := resource.ParseQuantity("1")
		if err != nil {
			return resources, err
		}
		resources.Limits["nvidia.com/gpu"] = gpu
		resources.Requests["nvidia.com/gpu"] = gpu
	}

	return resources, nil
}

func getOwnerReference(owner *KserveServiceOwner) []metav1.OwnerReference {
	if owner == nil {
		return nil
	}
	controller := false
	blockOwnerDeletion := true

	return []metav1.OwnerReference{
		{
			APIVersion:         owner.APIVersion,
			Kind:               owner.Kind,
			Name:               owner.Name,
			UID:                owner.UID,
			Controller:         &controller,
			BlockOwnerDeletion: &blockOwnerDeletion,
		},
	}
}

func getAPIPath(serviceName string) string {
	return fmt.Sprintf("/system/services/%s/models", serviceName)
}

func buildUnstructuredServiceOwnerRef(kserveSvc *unstructured.Unstructured) *KserveServiceOwner {
	return &KserveServiceOwner{
		APIVersion: kserveSvc.GetAPIVersion(),
		Name:       kserveSvc.GetName(),
		Namespace:  kserveSvc.GetNamespace(),
		UID:        kserveSvc.GetUID(),
		Kind:       kserveSvc.GetKind(),
	}
}
