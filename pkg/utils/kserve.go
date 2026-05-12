package utils

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/foomo/htpasswd"
	"github.com/grycap/oscar/v3/pkg/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	knv1 "knative.dev/serving/pkg/apis/serving/v1"
)

const (
	KserveISVCContainerName    = "kserve-container"
	KserveLLMISVCLabelKey      = "app.kubernetes.io/part-of"
	KserveLLMISVCLabelValue    = "llminferenceservice"
	KserveLLMISVCContainerName = "main"
)

const (
	//	defaultAuthMiddlewareName = "oidc-auth"
	httpRouteSuffix      = "-route"
	authMiddlewareSuffix = "-auth-mdw"
	authSecretSuffix     = "-auth-traefik" // #nosec G101
	corsMiddlewareSuffix = "-cors-mdw"
	defaultLLMCPUimage   = "vllm/vllm-openai-cpu:latest"
	defaultLLMGPUimage   = "vllm/vllm-openai:latest"
	kserveKeyLabelApp    = "oscar-app"
	prefixLabelApp       = "oscar-svc-ksv-"
)

var (
	llmInferenceServiceGVR     = schema.GroupVersionResource{Group: "serving.kserve.io", Version: "v1alpha1", Resource: "llminferenceservices"}
	kserveIsvcGVR              = schema.GroupVersionResource{Group: "serving.kserve.io", Version: "v1beta1", Resource: "inferenceservices"}
	kserveHTTPRouteGVR         = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
	kserveTraefikMiddlewareGVR = schema.GroupVersionResource{Group: "traefik.io", Version: "v1alpha1", Resource: "middlewares"}
	kserveSecretGVR            = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	// Mapping of supported model formats to their corresponding KServe runtime types and frameworks
	kserveTypeByFormat = map[string]kserveRuntime{
		"onnx":        {kserveType: "predictor", framework: "triton", protocolV1: false, protocolV2: true},
		"sklearn":     {kserveType: "predictor", framework: "mlserver", protocolV1: true, protocolV2: true},
		"xgboost":     {kserveType: "predictor", framework: "mlserver", protocolV1: true, protocolV2: true},
		"pytorch":     {kserveType: "predictor", framework: "mlserver", protocolV1: true, protocolV2: true},
		"tensorflow":  {kserveType: "predictor", framework: "mlserver", protocolV1: true, protocolV2: true},
		"triton":      {kserveType: "predictor", framework: "triton", protocolV1: true, protocolV2: true},
		"huggingface": {kserveType: "predictor", framework: "vllm", protocolV1: true, protocolV2: true},
		"llm":         {kserveType: "llm", framework: "vllm", protocolV1: true, protocolV2: true},
	}

	defaultKserveCpuRequest    = resource.MustParse("0.2")
	defaultKserveMemoryRequest = resource.MustParse("256Mi")
)
var kserveLogger = log.New(os.Stdout, "[KSERVE-SERVICE] ", log.Flags())

type kserveRuntime struct {
	kserveType string
	framework  string
	protocolV1 bool
	protocolV2 bool
}

func IsKserveService(service *types.Service) bool {
	// If the service has KServe configuration
	if service.Kserve == nil || (service.Kserve.ModelFormat == "" || service.Kserve.StorageUri == "") {
		return false
	}
	return true
}

func IsKserveSupported(cfg *types.Config) bool {
	return cfg.KserveEnable && cfg.ExposedServicesRouteKind == "httproute"
}

// ValidateKserveService checks if the provided service has valid KServe configuration.
func ValidateKserveService(service *types.Service) error {
	if !IsKserveService(service) {
		return fmt.Errorf("service does not have KServe configuration")
	}
	if !validModelFormat(service) {
		return fmt.Errorf("invalid ModelFormat: %s", service.Kserve.ModelFormat)
	}
	if service.Kserve.APIVersion != "" && service.Kserve.APIVersion != protocolVersion(service) {
		return fmt.Errorf("invalid APIVersion: %s for ModelFormat: %s", service.Kserve.APIVersion, service.Kserve.ModelFormat)
	}
	return nil
}

// CreateKserveService creates a KServe service based on the provided service and Knative service.
func CreateKserveService(service *types.Service, knativeService *knv1.Service, cfg *types.Config) error {
	if err := ValidateKserveService(service); err != nil {
		return err
	}

	dynClient, err := getDynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}
	//kserveLogger.Printf("Creating KServe service '%s' for user '%s' with model format %s", service.Name, service.Owner, service.Kserve.ModelFormat)

	if getKserveType(service.Kserve.ModelFormat) == "llm" {
		// For LLM services, we use a different InferenceService definition (LLMInferenceService)
		llmIsvc, err := NewKserveLLMInferenceServiceDefinition(service, knativeService, cfg)
		if err != nil {
			return err
		}

		_, err = dynClient.Resource(llmInferenceServiceGVR).Namespace(knativeService.Namespace).Create(context.Background(), llmIsvc, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create InferenceService: %v", err)
		}
		return nil
	}

	rawIsvc, err := NewKserveInferenceServiceDefinition(service, knativeService, cfg)
	if err != nil {
		return err
	}
	_, err = dynClient.Resource(kserveIsvcGVR).Namespace(knativeService.Namespace).Create(context.Background(), rawIsvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create InferenceService: %v", err)
	}

	err = exposeKserveInferenceService(service, knativeService, cfg)
	if err != nil {
		// If exposing the service fails, delete the created InferenceService to avoid orphaned resources
		deleteErr := DeleteKserveInferenceService(service.Name, service.Namespace)
		if deleteErr != nil {
			return fmt.Errorf("failed to expose InferenceService: %v; additionally, failed to delete InferenceService: %v", err, deleteErr)
		}
		return fmt.Errorf("failed to expose InferenceService: %v", err)
	}
	return nil
}

func UpdateKserveService(service *types.Service, oldService *types.Service, namespace string) error {
	if err := ValidateKserveService(service); err != nil {
		return err
	}
	if err := checkKserveUpdate(oldService, service); err != nil {
		return err
	}

	dynClient, err := getDynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	if getKserveType(service.Kserve.ModelFormat) == "llm" {
		// For LLM services, we use a different InferenceService definition (LLMInferenceService)
		// Get existing object to preserve resourceVersion
		oldLLMIsvc, err := dynClient.Resource(llmInferenceServiceGVR).Namespace(namespace).Get(context.Background(), buildKserveName(service.Name), metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get LLMInferenceService: %v", err)
		}
		updatedLLMIsvc, err := UpdateKserveLLMInferenceServiceDefinition(service, oldLLMIsvc)
		if err != nil {
			return err
		}

		_, err = dynClient.Resource(llmInferenceServiceGVR).Namespace(namespace).Update(context.Background(), updatedLLMIsvc, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update LLMInferenceService: %v", err)
		}
		return nil
	}

	// Get existing object to preserve resourceVersion
	oldIsvc, err := dynClient.Resource(kserveIsvcGVR).Namespace(namespace).Get(context.Background(), buildKserveName(service.Name), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get InferenceService: %v", err)
	}

	updatedIsvc, err := UpdateKserveInferenceServiceDefinition(service, oldIsvc)
	if err != nil {
		return err
	}

	_, err = dynClient.Resource(kserveIsvcGVR).Namespace(namespace).Update(context.Background(), updatedIsvc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update InferenceService: %v", err)
	}
	return nil
}

// NewKserveInferenceServiceDefinition builds an unstructured InferenceService
// (serving.kserve.io/v1beta1) suitable for use with a dynamic Kubernetes client.
// It is functionally equivalent to NewKserveInferenceServiceDefinition.
func NewKserveInferenceServiceDefinition(service *types.Service, knSvc *knv1.Service, cfg *types.Config) (*unstructured.Unstructured, error) {
	if err := ValidateKserveService(service); err != nil {
		return nil, err
	}

	resources, err := createKserveResources(service.Kserve)
	if err != nil {
		return nil, err
	}

	modelSpec := map[string]any{
		"modelFormat":     map[string]any{"name": service.Kserve.ModelFormat},
		"storageUri":      service.Kserve.StorageUri,
		"protocolVersion": protocolVersion(service),
	}
	// TO DO: consider if we want to inject root path for LLM services as well, and if so, how to handle the case when the framework is vllm that expects the prefix to be preserved for routing
	//injectRootPath(service)

	modelSpec["resources"] = resources
	modelSpec["args"] = service.Kserve.Args
	modelSpec["env"] = types.ConvertEnvVars(service.Kserve.Env)

	predictor := map[string]any{
		"model": modelSpec,
		"labels": map[string]any{
			types.KueueOwnerLabel:       formatUID(service.Owner),
			"kueue.x-k8s.io/queue-name": BuildLocalQueueName(service.Name),
		},
	}
	minScale, maxScale := normalizeScaleFromKserveService(service.Kserve)
	predictor["minReplicas"] = minScale
	predictor["maxReplicas"] = maxScale

	labels := map[string]any{
		kserveKeyLabelApp:           prefixLabelApp + service.Name,
		types.OscarUserServiceLabel: "true",
	}
	if cfg.KueueEnable {
		labels["kueue.x-k8s.io/queue-name"] = BuildLocalQueueName(service.Name)
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": kserveIsvcGVR.Group + "/" + kserveIsvcGVR.Version,
		"kind":       "InferenceService",
		"metadata": map[string]any{
			"name":            buildKserveName(service.Name),
			"namespace":       knSvc.Namespace,
			"ownerReferences": getOwnerReference(knSvc),
			"labels":          labels,
		},
		"spec": map[string]any{
			"predictor": predictor,
		},
	}}, nil
}

// UpdateKserveInferenceServiceDefinition updates the spec fields of an existing
// unstructured InferenceService object in place, preserving metadata (including resourceVersion).
func UpdateKserveInferenceServiceDefinition(service *types.Service, oldIsvc *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if err := ValidateKserveService(service); err != nil {
		return nil, err
	}

	resources, err := createKserveResources(service.Kserve)
	if err != nil {
		return nil, err
	}

	modelSpec := map[string]any{
		"modelFormat":     map[string]any{"name": service.Kserve.ModelFormat},
		"storageUri":      service.Kserve.StorageUri,
		"protocolVersion": protocolVersion(service),
	}
	modelSpec["resources"] = resources
	modelSpec["args"] = service.Kserve.Args
	modelSpec["env"] = types.ConvertEnvVars(service.Kserve.Env)

	predictor := map[string]any{
		"model": modelSpec,
		"labels": map[string]any{
			types.KueueOwnerLabel:       formatUID(service.Owner),
			"kueue.x-k8s.io/queue-name": BuildLocalQueueName(service.Name),
		},
	}
	minScale, maxScale := normalizeScaleFromKserveService(service.Kserve)
	predictor["minReplicas"] = minScale
	predictor["maxReplicas"] = maxScale

	oldIsvc.Object["spec"] = map[string]any{
		"predictor": predictor,
	}
	return oldIsvc, nil
}

func GetKserveInferenceService(serviceName, namespace string) (*unstructured.Unstructured, error) {
	dynClient, err := getDynamicClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %v", err)
	}

	isvc, err := dynClient.Resource(kserveIsvcGVR).Namespace(namespace).Get(context.Background(), buildKserveName(serviceName), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get InferenceService: %v", err)
	}
	return isvc, nil
}

func DeleteKserveInferenceService(serviceName, namespace string) error {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %v", err)
	}
	dynClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}
	err = dynClient.Resource(kserveIsvcGVR).Namespace(namespace).Delete(context.Background(), buildKserveName(serviceName), metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Kserve InferenceService: %v", err)
	}
	return nil
}

func NewKserveLLMInferenceServiceDefinition(service *types.Service, knSvc *knv1.Service, cfg *types.Config) (*unstructured.Unstructured, error) {
	if err := ValidateKserveService(service); err != nil {
		return nil, err
	}
	if service.Kserve.ModelFormat != "llm" {
		return nil, fmt.Errorf("invalid ModelFormat for LLMInferenceService: %s", service.Kserve.ModelFormat)
	}

	runtimeImage := defaultLLMCPUimage
	if service.Kserve.EnableGPU {
		runtimeImage = defaultLLMGPUimage
	}

	modelName := service.Name
	if service.Kserve.LLM != nil {
		modelName = service.Kserve.LLM.ModelName
		if service.Kserve.LLM.RuntimeImage != "" {
			runtimeImage = service.Kserve.LLM.RuntimeImage
		}
	}
	// TO DO: consider if we want to inject root path for LLM services as well, and if so, how to handle the case when the framework is vllm that expects the prefix to be preserved for routing
	//injectRootPath(service)

	// Build container spec
	container := map[string]any{
		"name":  "main",
		"image": runtimeImage,
		"securityContext": map[string]any{
			"runAsNonRoot": false,
		},
	}
	resources, err := createKserveResources(service.Kserve)
	if err != nil {
		return nil, err
	}
	container["resources"] = resources
	container["args"] = service.Kserve.Args
	container["env"] = types.ConvertEnvVars(service.Kserve.Env)

	minScale, _ := normalizeScaleFromKserveService(service.Kserve)

	router, err := buildKserveLLMServiceRouter(service, knSvc, cfg)
	if err != nil {
		return nil, err
	}

	labels := map[string]any{
		kserveKeyLabelApp:           prefixLabelApp + service.Name,
		types.OscarUserServiceLabel: "true",
	}
	if cfg.KueueEnable {
		labels["kueue.x-k8s.io/queue-name"] = BuildLocalQueueName(service.Name)
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": llmInferenceServiceGVR.Group + "/" + llmInferenceServiceGVR.Version,
		"kind":       "LLMInferenceService",
		"metadata": map[string]any{
			"name":            buildKserveName(service.Name),
			"namespace":       knSvc.Namespace,
			"ownerReferences": getOwnerReference(knSvc),
		},
		"spec": map[string]any{
			"model": map[string]any{
				"uri":  service.Kserve.StorageUri,
				"name": modelName,
			},
			"replicas": minScale,
			"labels":   labels,
			"template": map[string]any{
				"containers": []any{container},
			},
			"router": router,
		},
	}}, nil
}

// UpdateKserveLLMInferenceServiceDefinition updates the spec fields of an existing
// unstructured LLMInferenceService object in place, preserving metadata (including resourceVersion).
func UpdateKserveLLMInferenceServiceDefinition(service *types.Service, oldLLMIsvc *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if err := ValidateKserveService(service); err != nil {
		return nil, err
	}

	runtimeImage := defaultLLMCPUimage
	if service.Kserve.EnableGPU {
		runtimeImage = defaultLLMGPUimage
	}

	modelName := service.Name
	if service.Kserve.LLM != nil {
		modelName = service.Kserve.LLM.ModelName
		if service.Kserve.LLM.RuntimeImage != "" {
			runtimeImage = service.Kserve.LLM.RuntimeImage
		}
	}

	container := map[string]any{
		"name":  "main",
		"image": runtimeImage,
		"securityContext": map[string]any{
			"runAsNonRoot": false,
		},
	}
	resources, err := createKserveResources(service.Kserve)
	if err != nil {
		return nil, err
	}
	container["resources"] = resources
	container["args"] = service.Kserve.Args
	container["env"] = types.ConvertEnvVars(service.Kserve.Env)

	minScale, _ := normalizeScaleFromKserveService(service.Kserve)

	oldLLMIsvc.Object["spec"] = map[string]any{
		"model": map[string]any{
			"uri":  service.Kserve.StorageUri,
			"name": modelName,
		},
		"replicas": minScale,
		"labels":   oldLLMIsvc.Object["spec"].(map[string]any)["labels"],
		"template": map[string]any{
			"containers": []any{container},
		},
	}
	return oldLLMIsvc, nil
}

func GetKserveLabelSelector(serviceName string) string {
	return fmt.Sprintf("%s=%s", kserveKeyLabelApp, prefixLabelApp+serviceName)
}

func GetKserveSvcName(serviceNamne, kserveModelFormat string) string {
	if serviceNamne == "" {
		return ""
	}

	switch getKserveType(kserveModelFormat) {
	case "predictor":
		return serviceNamne + "-predictor"
	case "llm":
		return serviceNamne + "-kserve-workload-svc"
	default:
		return ""
	}
}

func GetKservePodAndDplName(serviceNamne, kserveModelFormat string) string {
	if serviceNamne == "" {
		return ""
	}

	switch getKserveType(kserveModelFormat) {
	case "predictor":
		return serviceNamne + "-predictor"
	case "llm":
		return serviceNamne + "-kserve"
	default:
		return ""
	}
}

func getKserveType(kserveModelFormat string) string {
	modelFormat := strings.ToLower(strings.TrimSpace(kserveModelFormat))
	if modelFormat == "" {
		return ""
	}

	if kserveType, ok := kserveTypeByFormat[modelFormat]; ok {
		return kserveType.kserveType
	}

	return ""
}

func getKserveFramework(kserveModelFormat string) string {
	modelFormat := strings.ToLower(strings.TrimSpace(kserveModelFormat))
	if modelFormat == "" {
		return ""
	}

	if kserveType, ok := kserveTypeByFormat[modelFormat]; ok {
		return kserveType.framework
	}

	return ""
}

func validModelFormat(service *types.Service) bool {
	KserveDef := service.Kserve
	switch getKserveType(KserveDef.ModelFormat) {
	case "predictor":
		return true
	case "llm": // TO DO: add more validation for LLM services
		return true
	default:
		return false
	}
}

func exposeKserveInferenceService(service *types.Service, knSvc *knv1.Service, cfg *types.Config) error {
	gatewayClientset, err := getDynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	if service.Kserve.SetAuth {
		if err = createTraefikAuthMiddleware(gatewayClientset, service, knSvc); err != nil {
			return fmt.Errorf("failed to create OIDC middleware: %v", err)
		}
		// Create OIDC forwardAuth Traefik Middleware
		/*
			if err = createTraefikOIDCMiddleware(gatewayClientset, service, knSvc, cfg); err != nil {
				return fmt.Errorf("failed to create OIDC middleware: %v", err)
			}
		*/
	}

	// Create HTTPRoute
	if err = createHTTPRoute(gatewayClientset, service, knSvc, cfg); err != nil {
		return fmt.Errorf("failed to create HTTPRoute: %v", err)
	}

	return nil
}

func checkKserveUpdate(oldService *types.Service, newService *types.Service) error {
	if oldService.Token != newService.Token {
		return fmt.Errorf("unexpected error")
	}
	oldKserve := oldService.Kserve
	newKserve := newService.Kserve
	// If both old and new KServe configurations are nil,
	// we consider it valid (no change)
	if oldKserve == nil && newKserve == nil {
		return nil
	}
	// If one of them is nil and the other is not,
	// it's a not alloved change in KServe configuration
	if oldKserve == nil || newKserve == nil {
		return fmt.Errorf("cannot add or remove KServe configuration")

	}
	// ModelFormat is the only field we allow to be set at creation
	// and not changed afterwards, so if it changes we return false
	// We also check StorageUri and SetAuth because changing them would require changes to the InferenceService spec
	// that are not supported in an update, and we want to prevent users from making changes that would lead to an
	// inconsistent state where the spec does not match the service configuration
	if oldKserve.ModelFormat != newKserve.ModelFormat {
		return fmt.Errorf("cannot update model format for KServe")
	}
	if oldKserve.StorageUri != newKserve.StorageUri {
		return fmt.Errorf("cannot update model storage configuration for KServe")
	}
	if oldKserve.SetAuth != newKserve.SetAuth {
		return fmt.Errorf("cannot update authentication configuration for KServe")
	}
	return nil
}

func getDynamicClient() (*dynamic.DynamicClient, error) {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %v", err)
	}
	dynClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %v", err)
	}
	return dynClient, nil
}

func normalizeScaleFromKserveService(service *types.Kserve) (int32, int32) {
	var minScale int32 = 0
	var maxScale int32 = 1
	if service.MinScale >= 1 {
		minScale = service.MinScale
	}
	if service.MaxScale >= 1 {
		maxScale = service.MaxScale
	}
	if minScale > maxScale {
		maxScale = minScale
	}
	return minScale, maxScale
}

func buildKserveName(serviceName string) string {
	// TODO
	return serviceName
}

// Helper function to determine the protocol version for KServe based on service configuration
// Defaults to "v1" if not specified or invalid
func protocolVersion(service *types.Service) string {
	modelFormat := strings.ToLower(strings.TrimSpace(service.Kserve.ModelFormat))
	switch {
	case service.Kserve.APIVersion == "v1" && kserveTypeByFormat[modelFormat].protocolV1:
		return "v1"
	case service.Kserve.APIVersion == "v2" && kserveTypeByFormat[modelFormat].protocolV2:
		return "v2"
	// For model formats that do not support v1, default to v2
	case (service.Kserve.APIVersion == "" || service.Kserve.APIVersion == "v1") && !kserveTypeByFormat[modelFormat].protocolV1:
		return "v2"
	default:
		return "v1"
	}
}

// createTraefikOIDCMiddleware creates a Traefik Middleware of type ForwardAuth for OIDC authentication,
// which will be used in the HTTPRoute to protect the KServe service.
// TO DO: change implementation when decided how to handle authentication for KServe services
func createTraefikOIDCMiddleware(gatewayClientset dynamic.Interface, service *types.Service, knSvc *knv1.Service, cfg *types.Config) error {
	authEndpointAddress := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d/system/config", cfg.Name, cfg.Namespace, cfg.ServicePort)
	middleware := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "traefik.io/v1alpha1",
		"kind":       "Middleware",
		"metadata": map[string]any{
			"name":            getTraefikCORSMiddlewareName(knSvc.Name),
			"namespace":       knSvc.Namespace,
			"ownerReferences": getOwnerReference(knSvc),
		},
		"spec": map[string]any{
			"forwardAuth": map[string]any{
				"address":            authEndpointAddress,
				"trustForwardHeader": true,
			},
		},
	}}

	_, err := gatewayClientset.Resource(kserveTraefikMiddlewareGVR).Namespace(knSvc.Namespace).Create(context.Background(), middleware, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create OIDC middleware: %v", err)
	}
	return nil
}

func createTraefikAuthMiddleware(gatewayClientset dynamic.Interface, service *types.Service, knSvc *knv1.Service) error {
	err := createTraefikAuthSecret(gatewayClientset, service, knSvc)
	if err != nil {
		return fmt.Errorf("failed to create auth secret: %v", err)
	}

	middleware := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "traefik.io/v1alpha1",
		"kind":       "Middleware",
		"metadata": map[string]any{
			"name":            getTraefikAuthMiddlewareName(service.Name),
			"namespace":       knSvc.Namespace,
			"ownerReferences": getOwnerReference(knSvc),
		},
		"spec": map[string]any{
			"basicAuth": map[string]any{
				"secret": getTraefikAuthSecretName(service.Name),
			},
		},
	}}
	_, err = gatewayClientset.Resource(kserveTraefikMiddlewareGVR).Namespace(knSvc.Namespace).Create(context.TODO(), middleware, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create basic auth middleware: %v", err)
	}
	return nil
}

func createTraefikAuthSecret(gatewayClientset dynamic.Interface, service *types.Service, knSvc *knv1.Service) error {
	hash := make(htpasswd.HashedPasswords)
	err := hash.SetPassword(service.Name, service.Token, htpasswd.HashAPR1)
	if err != nil {
		kserveLogger.Print(err.Error())
		return fmt.Errorf("failed to hash password: %v", err)
	}

	secret := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":            getTraefikAuthSecretName(service.Name),
			"namespace":       knSvc.Namespace,
			"ownerReferences": getOwnerReference(knSvc),
		},
		"immutable": true,
		"stringData": map[string]any{
			"users": service.Name + ":" + hash[service.Name],
		},
		"type": "Opaque",
	}}
	_, err = gatewayClientset.Resource(kserveSecretGVR).Namespace(knSvc.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	return err
}

func getHTTPRouteName(serviceName string) string {
	return serviceName + "-route"
}

func getTraefikCORSMiddlewareName(serviceName string) string {
	return serviceName + "-cors-mdw"
}

func getTraefikAuthMiddlewareName(serviceName string) string {
	return serviceName + "-auth-mdw"
}

func getTraefikAuthSecretName(serviceName string) string {
	return serviceName + "-auth-traefik"
}

// createHTTPRoute creates a Gateway API HTTPRoute to expose the KServe InferenceService.
func createHTTPRoute(gatewayClientset dynamic.Interface, service *types.Service, knSvc *knv1.Service, cfg *types.Config) error {
	isvcName := service.Name
	httpRouteName := isvcName + httpRouteSuffix
	namespace := knSvc.Namespace
	apiPath := getAPIPath(isvcName)
	svcName := GetKserveSvcName(isvcName, service.Kserve.ModelFormat)

	filters := []any{
		map[string]any{
			"type": "RequestHeaderModifier",
			"requestHeaderModifier": map[string]any{
				"set": []any{
					map[string]any{"name": "KServe-Isvc-Name", "value": isvcName},
					map[string]any{"name": "KServe-Isvc-Namespace", "value": namespace},
				},
			},
		},
	}
	if service.Kserve.SetAuth {
		filters = append(filters, map[string]any{
			"type": "ExtensionRef",
			"extensionRef": map[string]any{
				"group": "traefik.io",
				"kind":  "Middleware",
				"name":  getTraefikAuthMiddlewareName(service.Name),
			},
		})
	}
	// TO DO: vLLM framework support root path change
	filters = append(filters, map[string]any{
		"type": "URLRewrite",
		"urlRewrite": map[string]any{
			"path": map[string]any{
				"type":               "ReplacePrefixMatch",
				"replacePrefixMatch": "/",
			},
		},
	})

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
				"name":      svcName,
				"namespace": namespace,
				"port":      int64(80),
			},
		},
	}

	spec := map[string]any{
		"rules": []any{rule},
	}

	if host := strings.TrimSpace(cfg.IngressHost); host != "" {
		spec["hostnames"] = []any{host}
	}

	parentRef := map[string]any{
		"group": "gateway.networking.k8s.io",
		"kind":  "Gateway",
		"name":  strings.TrimSpace(cfg.HTTPRouteGatewayName),
	}
	if gwNamespace := strings.TrimSpace(cfg.HTTPRouteGatewayNamespace); gwNamespace != "" {
		parentRef["namespace"] = gwNamespace
	}
	spec["parentRefs"] = []any{parentRef}

	httpRoute := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]any{
			"name":            httpRouteName,
			"namespace":       namespace,
			"ownerReferences": getOwnerReference(knSvc),
		},
		"spec": spec,
	}}

	_, err := gatewayClientset.Resource(kserveHTTPRouteGVR).Namespace(namespace).Create(context.Background(), httpRoute, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create HTTPRoute: %v", err)
	}
	return nil
}

func createKserveResources(service *types.Kserve) (v1.ResourceRequirements, error) {
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
		resources.Limits[corev1.ResourceCPU] = cpu
		resources.Requests[corev1.ResourceCPU] = cpu
	}

	if len(service.Memory) > 0 {
		memory, err := resource.ParseQuantity(service.Memory)
		if err != nil {
			return resources, err
		}
		resources.Limits[corev1.ResourceMemory] = memory
		resources.Requests[corev1.ResourceMemory] = memory
	}

	if service.EnableGPU {
		gpu, err := resource.ParseQuantity("1")
		if err != nil {
			return resources, err
		}
		resources.Limits["nvidia.com/gpu"] = gpu
	}

	return resources, nil
}

func buildKserveLLMServiceRouter(service *types.Service, knSvc *knv1.Service, cfg *types.Config) (map[string]any, error) {
	gatewayClientset, err := getDynamicClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %v", err)
	}

	if service.Kserve.SetAuth {
		//err := createTraefikOIDCMiddleware(gatewayClientset, service, knSvc, cfg)
		err := createTraefikAuthMiddleware(gatewayClientset, service, knSvc)
		if err != nil {
			return nil, fmt.Errorf("failed to create Auth middleware: %v", err)
		}
	}
	return getKserveLLMServiceRouterSpec(service, knSvc.Namespace), nil
}

// getKserveLLMServiceRouterSpec returns a router configuration for LLM InferenceServices
// to route requests based on the service name (use inference Pool).
func getKserveLLMServiceRouterSpec(service *types.Service, namespace string) map[string]any {
	filters := []any{
		map[string]any{
			"type": "RequestHeaderModifier",
			"requestHeaderModifier": map[string]any{
				"set": []any{
					map[string]any{"name": "KServe-Isvc-Name", "value": service.Name},
					map[string]any{"name": "KServe-Isvc-Namespace", "value": namespace},
				},
			},
		},
	}
	if service.Kserve.SetAuth {
		filters = append(filters, map[string]any{
			"type": "ExtensionRef",
			"extensionRef": map[string]any{
				"group": "traefik.io",
				"kind":  "Middleware",
				"name":  getTraefikAuthMiddlewareName(service.Name),
			},
		})
	}

	// TO DO: LLM use vLLM that supports root path change
	filters = append(filters, map[string]any{
		"type": "URLRewrite",
		"urlRewrite": map[string]any{
			"path": map[string]any{
				"type":               "ReplacePrefixMatch",
				"replacePrefixMatch": "/",
			},
		},
	})

	return map[string]any{
		"route": map[string]any{
			"http": map[string]any{
				"spec": map[string]any{
					"rules": []any{
						map[string]any{
							"matches": []any{
								map[string]any{
									"path": map[string]any{
										"type":  "PathPrefix",
										"value": getAPIPath(service.Name),
									},
								},
							},
							"filters": filters,
						},
					},
				},
			},
		},
	}
}

func getOwnerReference(knSvc *knv1.Service) []metav1.OwnerReference {
	controller := false
	blockOwnerDeletion := true

	return []metav1.OwnerReference{
		metav1.OwnerReference{
			APIVersion:         "serving.knative.dev/v1",
			Kind:               "Service",
			Name:               knSvc.Name,
			UID:                knSvc.UID,
			Controller:         &controller,
			BlockOwnerDeletion: &blockOwnerDeletion,
		},
	}
}

func getAPIPath(serviceName string) string {
	return fmt.Sprintf("/system/services/%s/exposed", serviceName)
}

func formatUID(uid string) string {
	uidr, _ := regexp.Compile("[0-9a-z]+@")
	idx := uidr.FindStringIndex(uid)
	// If the regex is not matched assume it is not an EGI uid
	// and return the original string
	if idx == nil {
		return uid
	}

	uid = uid[0 : idx[1]-1]
	if len(uid) > 62 {
		uid = uid[:62]
	}
	return uid
}

func injectRootPath(service *types.Service) {
	kserveServiceFramework := getKserveFramework(service.Kserve.ModelFormat)
	if kserveServiceFramework != "triton" {
		if kserveServiceFramework == "mlserver" {
			if service.Kserve.Env == nil {
				service.Kserve.Env = make(map[string]string)
			}
			service.Kserve.Env["MLSERVER_ROOT_PATH"] = getAPIPath(service.Name)
		} else if kserveServiceFramework == "vllm" {
			service.Kserve.Args = append(service.Kserve.Args, fmt.Sprintf("--root-path=%s", getAPIPath(service.Name)))
		}
	}
}

/*
if service.Expose.SetAuth {
		if err := createTraefikAuthSecret(service, namespace, kubeClientset); err != nil {
			return err
		}
		if err := createTraefikAuthMiddleware(service, namespace); err != nil {
			return err
		}
	}*/
