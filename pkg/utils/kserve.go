package utils

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/foomo/htpasswd"
	"github.com/grycap/oscar/v4/pkg/types"
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
	httpRouteSuffix               = "-route"
	authMiddlewareSuffix          = "-auth-mdw"
	authSecretSuffix              = "-auth-traefik" // #nosec G101
	corsMiddlewareSuffix          = "-cors-mdw"
	defaultLLMCPUimage            = "vllm/vllm-openai-cpu:latest"
	defaultLLMGPUimage            = "vllm/vllm-openai:latest"
	kserveKeyLabelApp             = "oscar-app"
	prefixLabelApp                = "oscar-svc-ksv-"
	kserveIsvcSuffix              = "-predictor"
	kserveIsvcPodDplSuffix        = "-predictor"
	kserveLLMIsvcSuffix           = "-kserve-workload-svc"
	kserveLLMIsvcPodDplSuffix     = "-kserve"
	KserveTypeInferenceService    = "inference"
	KserveTypeLLMInferenceService = "llm_inference"
)

var (
	llmInferenceServiceGVR     = schema.GroupVersionResource{Group: "serving.kserve.io", Version: "v1alpha1", Resource: "llminferenceservices"}
	kserveIsvcGVR              = schema.GroupVersionResource{Group: "serving.kserve.io", Version: "v1beta1", Resource: "inferenceservices"}
	kserveHTTPRouteGVR         = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
	kserveTraefikMiddlewareGVR = schema.GroupVersionResource{Group: "traefik.io", Version: "v1alpha1", Resource: "middlewares"}
	kserveSecretGVR            = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	defaultKserveCpuRequest    = resource.MustParse("0.2")
	defaultKserveMemoryRequest = resource.MustParse("256Mi")
)

type dynamicClientFactory func() (*dynamic.DynamicClient, error)

var newDynamicClient dynamicClientFactory = func() (*dynamic.DynamicClient, error) {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(restCfg)
}

var kserveLogger = log.New(os.Stdout, "[KSERVE-SERVICE] ", log.Flags())

func IsKserveService(service *types.Service) bool {
	// If the service has KServe configuration
	if service.Kserve == nil /* || service.Kserve.StorageUri == "" || service.Kserve.Type == ""*/ {
		return false
	}
	/*if service.Kserve.Type == KserveTypeInferenceService && (service.Kserve.Inference == nil || service.Kserve.Inference.ModelFormat == "") {
		return false
	}*/
	return true
}

func IsKserveSupported(cfg *types.Config) bool {
	return cfg.KserveEnable && cfg.ExposedServicesRouteKind == types.HTTPROUTE
}

// ValidateKserveService checks if the provided service has valid KServe configuration.
func ValidateKserveService(service *types.Service) error {
	if !IsKserveService(service) {
		return fmt.Errorf("service does not have KServe configuration")
	}

	if service.Kserve.Type == "" {
		return fmt.Errorf("missing KServe service type %s | %s", KserveTypeInferenceService, KserveTypeLLMInferenceService)
	}

	if service.Kserve.StorageUri == "" {
		return fmt.Errorf("missing model storage URI in KServe configuration")
	}

	if service.Kserve.Type == KserveTypeInferenceService {
		if service.Kserve.Inference == nil {
			return fmt.Errorf("missing Inference configuration for KServe service")
		}
		if service.Kserve.Inference.ModelFormat == "" {
			return fmt.Errorf("missing model format in KServe configuration")
		}
		if service.Kserve.LLMInference != nil {
			return fmt.Errorf("LLMInference configuration should be nil for Inference type")
		}
	}

	if service.Kserve.Type == KserveTypeLLMInferenceService {
		if service.Kserve.Inference != nil {
			return fmt.Errorf("Inference configuration should be nil for LLMInference type")
		}
	}

	if service.Kserve.APIVersion != "" && !(service.Kserve.APIVersion == "v1" || service.Kserve.APIVersion == "v2") {
		return fmt.Errorf("invalid APIVersion: %s", service.Kserve.APIVersion)
	}
	return nil
}

// CreateKserveService creates a KServe service based on the provided service and Knative service.
func CreateKserveService(service *types.Service, knativeService *knv1.Service, cfg *types.Config) error {
	if err := ValidateKserveService(service); err != nil {
		return err
	}

	ok, err := existsKserveHTTPRouteByServiceName(service.Name, service.Namespace)
	if err != nil {
		return fmt.Errorf("failed to check HTTPRoute availability for service %s: %v", service.Name, err)
	}
	if ok {
		return fmt.Errorf("HTTPRoute for service %s already taken, change the service name", service.Name)
	}

	dynClient, err := getDynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}
	//kserveLogger.Printf("Creating KServe service '%s' for user '%s' with model format %s", service.Name, service.Owner, service.Kserve.ModelFormat)

	if service.Kserve.Type == KserveTypeLLMInferenceService {
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
	if err := CheckKserveUpdate(oldService, service); err != nil {
		return err
	}

	dynClient, err := getDynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	if service.Kserve.Type == KserveTypeLLMInferenceService {
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

	apiVersion := "v1"
	if service.Kserve.APIVersion != "" {
		apiVersion = service.Kserve.APIVersion
	}

	modelSpec := map[string]any{
		"modelFormat":     map[string]any{"name": service.Kserve.Inference.ModelFormat},
		"storageUri":      service.Kserve.StorageUri,
		"protocolVersion": apiVersion,
	}

	if service.Kserve.Inference.Runtime != "" {
		modelSpec["runtime"] = service.Kserve.Inference.Runtime
	}
	// TO DO: consider if we want to inject root path for LLM services as well, and if so, how to handle the case when the framework is vllm that expects the prefix to be preserved for routing
	//injectRootPath(service)

	modelSpec["resources"] = resources
	modelSpec["args"] = service.Kserve.Args
	modelSpec["env"] = types.ConvertEnvVars(service.Kserve.Env)

	predictor := map[string]any{
		"model": modelSpec,
	}
	minScale, maxScale := normalizeScaleFromKserveService(service.Kserve)
	predictor["minReplicas"] = minScale
	predictor["maxReplicas"] = maxScale

	labels := map[string]any{
		kserveKeyLabelApp:           prefixLabelApp + service.Name,
		types.OscarUserServiceLabel: "true",
	}

	if cfg.KueueEnable {
		localQueueName, ok := service.Labels["kueue.x-k8s.io/queue-name"]
		if ok && localQueueName != "" {
			labels["kueue.x-k8s.io/queue-name"] = localQueueName
		} else if service.Owner != types.DefaultOwner {
			return nil, fmt.Errorf("missing required label 'kueue.x-k8s.io/queue-name' for KServe service with Kueue enabled")
		}
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

	apiVersion := service.Kserve.APIVersion
	if apiVersion == "" {
		oldApiVersion, ok := oldIsvc.Object["spec"].(map[string]any)["predictor"].(map[string]any)["model"].(map[string]any)["protocolVersion"]
		if !ok {
			apiVersion = "v1"
		} else {
			apiVersion = oldApiVersion.(string)
		}
	}

	modelSpec := map[string]any{
		"modelFormat":     map[string]any{"name": service.Kserve.Inference.ModelFormat},
		"storageUri":      service.Kserve.StorageUri,
		"protocolVersion": apiVersion,
	}
	if service.Kserve.Inference.Runtime != "" {
		modelSpec["runtime"] = service.Kserve.Inference.Runtime
	}

	modelSpec["resources"] = resources
	modelSpec["args"] = service.Kserve.Args
	modelSpec["env"] = types.ConvertEnvVars(service.Kserve.Env)

	predictor := map[string]any{
		"model": modelSpec,
	}
	minScale, maxScale := normalizeScaleFromKserveService(service.Kserve)
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
	dynClient, err := getDynamicClient()
	if err != nil {
		return err
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

	runtimeImage := defaultLLMCPUimage
	if service.Kserve.EnableGPU {
		runtimeImage = defaultLLMGPUimage
	}
	if service.Kserve.LLMInference != nil && service.Kserve.LLMInference.RuntimeImage != "" {
		runtimeImage = service.Kserve.LLMInference.RuntimeImage
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
		localQueueName, ok := service.Labels["kueue.x-k8s.io/queue-name"]
		if ok && localQueueName != "" {
			labels["kueue.x-k8s.io/queue-name"] = localQueueName
		} else if service.Owner != types.DefaultOwner {
			return nil, fmt.Errorf("missing required label 'kueue.x-k8s.io/queue-name' for KServe service with Kueue enabled")
		}
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
				"name": service.Name,
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
	if service.Kserve.LLMInference != nil && service.Kserve.LLMInference.RuntimeImage != "" {
		runtimeImage = service.Kserve.LLMInference.RuntimeImage
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

	labels := map[string]any{}
	if oldlabels, ok := oldLLMIsvc.Object["spec"].(map[string]any)["labels"]; ok {
		labels = oldlabels.(map[string]any)
	}

	oldLLMIsvc.Object["spec"] = map[string]any{
		"model": map[string]any{
			"uri":  service.Kserve.StorageUri,
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

func GetKserveLabelSelector(serviceName string) string {
	return fmt.Sprintf("%s=%s", kserveKeyLabelApp, prefixLabelApp+serviceName)
}

func GetKserveSvcName(serviceName, kserveType string) string {
	if serviceName == "" {
		return ""
	}

	switch kserveType {
	case KserveTypeInferenceService:
		return serviceName + kserveIsvcSuffix
	case KserveTypeLLMInferenceService:
		return serviceName + kserveLLMIsvcSuffix
	default:
		return ""
	}
}

func GetKservePodAndDplName(serviceName, kserveType string) string {
	if serviceName == "" {
		return ""
	}

	switch kserveType {
	case KserveTypeInferenceService:
		return serviceName + kserveIsvcPodDplSuffix
	case KserveTypeLLMInferenceService:
		return serviceName + kserveLLMIsvcPodDplSuffix
	default:
		return ""
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
		// TO DO: Create OIDC forwardAuth Traefik Middleware
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

func CheckKserveUpdate(oldService *types.Service, newService *types.Service) error {
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
	// it's a not allowed change in KServe configuration
	if oldKserve == nil || newKserve == nil {
		return fmt.Errorf("cannot add or remove KServe configuration")
	}
	if oldKserve.StorageUri != newKserve.StorageUri {
		return fmt.Errorf("cannot update model storage configuration for KServe")
	}
	if oldKserve.Inference.Runtime != newKserve.Inference.Runtime {
		return fmt.Errorf("cannot update runtime for KServe")
	}
	if oldKserve.SetAuth != newKserve.SetAuth {
		return fmt.Errorf("cannot update authentication configuration for KServe")
	}
	if oldKserve.Type != newKserve.Type {
		return fmt.Errorf("cannot change KServe service type")

	} else if newKserve.Type == KserveTypeInferenceService {
		if oldKserve.Inference == nil || newKserve.Inference == nil {
			return fmt.Errorf("inference configuration cannot be nil for KServe service")

		} else if oldKserve.Inference.ModelFormat != newKserve.Inference.ModelFormat {
			return fmt.Errorf("cannot update model format for KServe")
		}
	}

	return nil
}

func getDynamicClient() (*dynamic.DynamicClient, error) {
	dynClient, err := newDynamicClient()
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
	if service.Type == KserveTypeLLMInferenceService && minScale == 0 {
		minScale = 1
	}
	return minScale, maxScale
}

func buildKserveName(serviceName string) string {
	// TODO
	return serviceName
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
	svcName := GetKserveSvcName(isvcName, service.Kserve.Type)
	gwName := strings.TrimSpace(cfg.HTTPRouteGatewayName)
	gwNamespace := strings.TrimSpace(cfg.HTTPRouteGatewayNamespace)
	if gwNamespace == "" || gwName == "" {
		return fmt.Errorf("gateway namespace and name must be provided in config to create HTTPRoute for KServe service")
	}

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
		"group":     "gateway.networking.k8s.io",
		"kind":      "Gateway",
		"name":      gwName,
		"namespace": gwNamespace,
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
	if err != nil {
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
		resources.Requests["nvidia.com/gpu"] = gpu
	}

	return resources, nil
}

func buildKserveLLMServiceRouter(service *types.Service, knSvc *knv1.Service, cfg *types.Config) (map[string]any, error) {
	gwName := strings.TrimSpace(cfg.HTTPRouteGatewayName)
	gwNamespace := strings.TrimSpace(cfg.HTTPRouteGatewayNamespace)
	if gwNamespace == "" || gwName == "" {
		return nil, fmt.Errorf("gateway namespace and name must be provided in config to create HTTPRoute for KServe service")
	}

	if service.Kserve.SetAuth {
		gatewayClientset, err := getDynamicClient()
		if err != nil {
			return nil, fmt.Errorf("failed to create dynamic client: %v", err)
		}

		//err := createTraefikOIDCMiddleware(gatewayClientset, service, knSvc, cfg)
		err = createTraefikAuthMiddleware(gatewayClientset, service, knSvc)
		if err != nil {
			return nil, fmt.Errorf("failed to create Auth middleware: %v", err)
		}
	}
	return getKserveLLMServiceRouterSpec(service, knSvc.Namespace, cfg), nil
}

// getKserveLLMServiceRouterSpec returns a router configuration for LLM InferenceServices
// to route requests based on the service name (use inference Pool).
func getKserveLLMServiceRouterSpec(service *types.Service, namespace string, cfg *types.Config) map[string]any {
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

	// TO DO: Evaluate the use of InferencePool
	// Traefik do not support InferencePool
	// https://gateway-api-inference-extension.sigs.k8s.io/implementations/gateways/
	backendRefs := []any{
		map[string]any{
			"group": "",
			"kind":  "Service",
			"name":  GetKserveSvcName(service.Name, service.Kserve.Type),
			"port":  8000,
		},
	}

	spec := map[string]any{
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
				"filters":     filters,
				"backendRefs": backendRefs,
			},
		},
	}

	if cfg != nil {
		if host := strings.TrimSpace(cfg.IngressHost); host != "" {
			spec["hostnames"] = []any{host}
		}
	}

	return map[string]any{
		"gateway": map[string]any{
			"name":      strings.TrimSpace(cfg.HTTPRouteGatewayName),
			"namespace": strings.TrimSpace(cfg.HTTPRouteGatewayNamespace),
		},
		"route": map[string]any{
			"http": map[string]any{
				"spec": spec,
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

/*
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
*/
// existsKserveHTTPRouteByServiceName checks whether there is any HTTPRoute in the
// cluster that matches the expected name and API path for the given service name, and returns an error if there are any conflicts (e.g. same name but different path, or same path but different name). It returns true if a matching HTTPRoute exists in the same namespace, false if no matching HTTPRoute exists, and an error if there is a conflict.
func existsKserveHTTPRouteByServiceName(serviceName, namespace string) (bool, error) {
	routeName := getHTTPRouteName(serviceName)

	dynClient, err := getDynamicClient()
	if err != nil {
		return false, fmt.Errorf("failed to create dynamic client: %v", err)
	}

	routeList, err := dynClient.Resource(kserveHTTPRouteGVR).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{FieldSelector: "metadata.name=" + routeName})
	if err != nil {
		return false, fmt.Errorf("failed to list HTTPRoutes: %v", err)
	}

	return (len(routeList.Items) > 0), nil
}
