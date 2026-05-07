package utils

import (
	"context"
	"fmt"
	"regexp"
	"strings"

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
	authMiddlewareSuffix = "-mdw-auth"
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
	// Mapping of supported model formats to their corresponding KServe runtime types and frameworks
	kserveTypeByFormat = map[string]kserveRuntime{
		"onnx":        {kserveType: "predictor", framework: "mlserver"},
		"sklearn":     {kserveType: "predictor", framework: "mlserver"},
		"xgboost":     {kserveType: "predictor", framework: "mlserver"},
		"pytorch":     {kserveType: "predictor", framework: "mlserver"},
		"tensorflow":  {kserveType: "predictor", framework: "mlserver"},
		"triton":      {kserveType: "predictor", framework: "triton"},
		"huggingface": {kserveType: "predictor", framework: "vllm"},
		"llm":         {kserveType: "llm", framework: "vllm"},
	}

	defaultKserveCpuRequest    = resource.MustParse("0.2")
	defaultKserveMemoryRequest = resource.MustParse("256Mi")
)

type kserveRuntime struct {
	kserveType string
	framework  string
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
	dynClient, err := getDynamicClient()

	if err := ValidateKserveService(service); err != nil {
		return err
	}

	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	if getKserveType(service.Kserve.ModelFormat) == "llm" {
		// For LLM services, we use a different InferenceService definition (LLMInferenceService)
		llmIsvc, err := NewKserveLLMInferenceServiceDefinition(service, knativeService, cfg)
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
	if checkKserveUpdate(service.Kserve, oldService.Kserve) {
		return fmt.Errorf("model format changes or adding/removing KServe configuration are not supported after service creation")
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
	minScale, maxScale := normalizeScaleFromKserveService(service.Kserve)

	predictor := map[string]any{
		"model":       modelSpec,
		"minReplicas": minScale,
		"maxReplicas": maxScale,
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

func exposeKserveInferenceService(service *types.Service, knSvc *knv1.Service, cfg *types.Config) error {
	gatewayClientset, err := getDynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	var authMiddlewareName string = ""
	if service.Kserve.SetAuth {
		authMiddlewareName = service.Name + authMiddlewareSuffix
		// Create OIDC forwardAuth Traefik Middleware
		if err = createOIDCMiddleware(gatewayClientset, cfg, knSvc, authMiddlewareName); err != nil {
			return fmt.Errorf("failed to create OIDC middleware: %v", err)
		}
	}

	// Create HTTPRoute
	if err = createHTTPRoute(gatewayClientset, service, knSvc, cfg, authMiddlewareName); err != nil {
		return fmt.Errorf("failed to create HTTPRoute: %v", err)
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

func checkKserveUpdate(old *types.Kserve, new *types.Kserve) bool {
	// If both old and new KServe configurations are nil,
	// we consider it valid (no change)
	if old == nil && new == nil {
		return true
	}
	// If one of them is nil and the other is not,
	// it's a not alloved change in KServe configuration
	if old == nil || new == nil {
		return false
	}
	// ModelFormat is the only field we allow to be set at creation
	// and not changed afterwards, so if it changes we return false
	return old.ModelFormat != new.ModelFormat
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

// Helper function to determine the protocol version for KServe based on service configuration
// Defaults to "v1" if not specified or invalid
func protocolVersion(service *types.Service) string {
	switch {
	case service.Kserve.APIVersion == "v1" && !onlyProtocolV2(service):
		return "v1"
	case service.Kserve.APIVersion == "v2" || onlyProtocolV2(service):
		return "v2"
	default:
		return "v1"
	}
}

// onlyProtocolV2 checks if the service runtime supports only Protocol V2
func onlyProtocolV2(service *types.Service) bool {
	return service.Kserve.ModelFormat == "onnx"
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

// createOIDCMiddleware creates a Traefik Middleware of type ForwardAuth for OIDC authentication,
// which will be used in the HTTPRoute to protect the KServe service.
// TO DO: change implementation when decided how to handle authentication for KServe services
func createOIDCMiddleware(gatewayClientset *dynamic.DynamicClient, cfg *types.Config, knSvc *knv1.Service, middlewareName string) error {
	if middlewareName == "" {
		return fmt.Errorf("middleware name cannot be empty when creating HTTPRoute")
	}
	authEndpointAddress := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d/system/config", cfg.Name, cfg.Namespace, cfg.ServicePort)
	/*if gatewayNamespace := strings.TrimSpace(cfg.HTTPRouteGatewayNamespace); gatewayNamespace != "" {
		parentRef["namespace"] = gatewayNamespace
	}*/
	middleware := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "traefik.io/v1alpha1",
		"kind":       "Middleware",
		"metadata": map[string]any{
			"name":            middlewareName,
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

// createHTTPRoute creates a Gateway API HTTPRoute to expose the KServe InferenceService.
func createHTTPRoute(gatewayClientset *dynamic.DynamicClient, service *types.Service, knSvc *knv1.Service, cfg *types.Config, authMiddlewareName string) error {
	isvcName := service.Name
	httpRouteName := isvcName + httpRouteSuffix
	namespace := knSvc.Namespace
	apiPath := getApiPath(isvcName)
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
	if service.Kserve.SetAuth && authMiddlewareName != "" {
		filters = append(filters, map[string]any{
			"type": "ExtensionRef",
			"extensionRef": map[string]any{
				"group": "traefik.io",
				"kind":  "Middleware",
				"name":  authMiddlewareName,
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

	var authMiddlewareName string = ""
	if service.Kserve.SetAuth {
		authMiddlewareName = service.Name + authMiddlewareSuffix
		err := createOIDCMiddleware(gatewayClientset, cfg, knSvc, authMiddlewareName)
		if err != nil {
			return nil, fmt.Errorf("failed to create OIDC middleware: %v", err)
		}
	}
	return getKserveLLMServiceRouter(service.Name, knSvc.Namespace, authMiddlewareName), nil
}

// getKserveLLMServiceRouter returns a router configuration for LLM InferenceServices
// to route requests based on the service name (use inference Pool).
func getKserveLLMServiceRouter(serviceName, namespace string, authMiddlewareName string) map[string]any {
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
	if authMiddlewareName != "" {
		filters = append(filters, map[string]any{
			"type": "ExtensionRef",
			"extensionRef": map[string]any{
				"group": "traefik.io",
				"kind":  "Middleware",
				"name":  authMiddlewareName,
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
										"value": getApiPath(serviceName),
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

func getApiPath(serviceName string) string {
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
			service.Kserve.Env["MLSERVER_ROOT_PATH"] = getApiPath(service.Name)
		} else if kserveServiceFramework == "vllm" {
			service.Kserve.Args = append(service.Kserve.Args, fmt.Sprintf("--root-path=%s", getApiPath(service.Name)))
		}
	}
}
