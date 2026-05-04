package utils

import (
	"context"
	"fmt"
	"log"
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
	//	defaultAuthMiddlewareName = "oidc-auth"
	httpRouteSuffix      = "-route"
	authMiddlewareSuffix = "-mdw-auth"
	defaultLLMCPUimage   = "vllm/vllm-openai-cpu:latest"
	defaultLLMGPUimage   = "vllm/vllm-openai:latest"
)

var (
	llmInferenceServiceGVR     = schema.GroupVersionResource{Group: "serving.kserve.io", Version: "v1alpha1", Resource: "llminferenceservices"}
	kserveIsvcGVR              = schema.GroupVersionResource{Group: "serving.kserve.io", Version: "v1beta1", Resource: "inferenceservices"}
	kserveHTTPRouteGVR         = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
	kserveTraefikMiddlewareGVR = schema.GroupVersionResource{Group: "traefik.io", Version: "v1alpha1", Resource: "middlewares"}
)

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

// NewKserveInferenceServiceDefinition builds an unstructured InferenceService
// (serving.kserve.io/v1beta1) suitable for use with a dynamic Kubernetes client.
// It is functionally equivalent to NewKserveInferenceServiceDefinition.
func NewKserveInferenceServiceDefinition(service *types.Service, knSvc *knv1.Service) (*unstructured.Unstructured, error) {
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
		"model":       modelSpec,
		"minReplicas": service.Kserve.MinScale,
		"maxReplicas": service.Kserve.MaxScale,
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "serving.kserve.io/v1beta1",
		"kind":       "InferenceService",
		"metadata": map[string]any{
			"name":            buildKserveName(service.Name),
			"namespace":       knSvc.Namespace,
			"ownerReferences": getOwnerReference(knSvc),
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
		"model":       modelSpec,
		"minReplicas": service.Kserve.MinScale,
		"maxReplicas": service.Kserve.MaxScale,
	}

	oldIsvc.Object["spec"] = map[string]any{
		"predictor": predictor,
	}
	return oldIsvc, nil
}

// CreateKserveInferenceService creates a KServe InferenceService based on the provided service and Knative service.
// It set an OwnerReference to the Knative service, so if the Knative service is deleted the KServe InferenceService will be automatically deleted by Kubernetes garbage collection.
// It returns the created InferenceService or an error if the creation fails.
func CreateKserveInferenceService(service *types.Service, knativeService *knv1.Service, cfg *types.Config) error {
	if err := ValidateKserveService(service); err != nil {
		return err
	}

	// Create dynamic client
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %v", err)
	}
	dynClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	if service.Kserve.ModelFormat == "llm" {
		log.Println("LLM Service creation")
		// For LLM services, we use a different InferenceService definition (LLMInferenceService)
		llmIsvc, err := NewKserveLLMInferenceServiceDefinition(service, knativeService, cfg)
		_, err = dynClient.Resource(llmInferenceServiceGVR).Namespace(knativeService.Namespace).Create(context.Background(), llmIsvc, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create InferenceService: %v", err)
		}
		return nil
	}

	rawIsvc, err := NewKserveInferenceServiceDefinition(service, knativeService)
	if err != nil {
		return err
	}
	_, err = dynClient.Resource(kserveIsvcGVR).Namespace(knativeService.Namespace).Create(context.Background(), rawIsvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create InferenceService: %v", err)
	}

	err = ExposeKserveService(service, knativeService, cfg)
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

func UpdateKserveInferenceService(service *types.Service, namespace string) error {
	if err := ValidateKserveService(service); err != nil {
		return err
	}

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %v", err)
	}
	dynClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
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

func GetKserveInferenceService(serviceName, namespace string) (*unstructured.Unstructured, error) {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %v", err)
	}
	dynClient, err := dynamic.NewForConfig(restCfg)
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

	runtimeImage := defaultLLMCPUimage
	if service.Kserve.EnableGPU {
		runtimeImage = defaultLLMGPUimage
	}

	modelName := ""
	if service.Kserve.LLM != nil {
		modelName = service.Kserve.LLM.ModelName
		if service.Kserve.LLM.RuntimeImage != "" {
			runtimeImage = service.Kserve.LLM.RuntimeImage
		}
	}

	// Build container spec
	container := map[string]any{
		"name":  "main",
		"image": runtimeImage,
	}
	resources, err := createKserveResources(service.Kserve)
	if err != nil {
		return nil, err
	}
	container["resources"] = resources
	container["args"] = service.Kserve.Args
	container["env"] = types.ConvertEnvVars(service.Kserve.Env)

	var replicas int32 = 1
	if service.Kserve.MinScale > 1 {
		replicas = service.Kserve.MinScale
	}

	router, err := buildKserveLLMServiceRouter(service, knSvc, cfg)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "serving.kserve.io/v1alpha1",
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
			"replicas": replicas,
			"template": map[string]any{
				"containers": []any{container},
			},
			"router": router,
		},
	}}, nil
}

func IsKserveService(service *types.Service) bool {
	// If the service has KServe configuration
	if service.Kserve == nil || (service.Kserve.ModelFormat == "" || service.Kserve.StorageUri == "") {
		return false
	}
	return true
}

func buildKserveName(serviceName string) string {
	// TODO
	return serviceName
}

func GetKserveSvcName(serviceNamne, kserveModelFormat string) string {
	if serviceNamne == "" {
		return ""
	}

	switch kserveModelFormat {
	case "onnx", "sklearn", "xgboost", "pytorch", "tensorflow", "triton", "huggingface":
		return serviceNamne + "-predictor"
	case "llm":
		return serviceNamne + "-kserve-workload-svc"
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
	switch KserveDef.ModelFormat {
	case "onnx", "sklearn", "xgboost", "pytorch", "tensorflow", "triton", "huggingface":
		return true
	case "llm": // TO DO: add more validation for LLM services
		return true
	default:
		return false
	}
}

func ExposeKserveService(service *types.Service, knSvc *knv1.Service, cfg *types.Config) error {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %v", err)
	}

	gatewayClientset, err := dynamic.NewForConfig(restCfg)
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
	apiPath := "/system/services/" + isvcName + "/exposed"
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
		Limits: corev1.ResourceList{},
	}

	if len(service.CPU) > 0 {
		cpu, err := resource.ParseQuantity(service.CPU)
		if err != nil {
			return resources, err
		}
		resources.Limits[corev1.ResourceCPU] = cpu
	}

	if len(service.Memory) > 0 {
		memory, err := resource.ParseQuantity(service.Memory)
		if err != nil {
			return resources, err
		}
		resources.Limits[corev1.ResourceMemory] = memory
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
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %v", err)
	}

	gatewayClientset, err := dynamic.NewForConfig(restCfg)
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
										"value": "/system/service/" + serviceName + "/",
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
