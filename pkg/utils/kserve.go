package utils

import (
	"context"
	"fmt"

	"github.com/grycap/oscar/v3/pkg/types"
	servingv1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	kserveclient "github.com/kserve/kserve/pkg/client/clientset/versioned"
	"github.com/kserve/kserve/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knv1 "knative.dev/serving/pkg/apis/serving/v1"
)

const (
	OSCAR_KSERVE_SERVICE_IMAGE  = "docker.io/curlimages/curl:8.18.0"
	OSCAR_KSERVE_SERVICE_SCRIPT = ""
)

func ValidateKserveService(service *types.Service) error {
	if !IsKserveService(service) {
		return fmt.Errorf("service does not have KServe configuration")
	}
	if !validModelFormat(service.Kserve.ModelFormat) {
		return fmt.Errorf("invalid ModelFormat: %s", service.Kserve.ModelFormat)
	}
	if service.Kserve.APIVersion != "" && service.Kserve.APIVersion != string(protocolVersion(service)) {
		return fmt.Errorf("invalid APIVersion: %s for ModelFormat: %s", service.Kserve.APIVersion, service.Kserve.ModelFormat)
	}
	return nil
}

func NewKserveInferenceServiceDefinition(service *types.Service, knSvc *knv1.Service) (*servingv1beta1.InferenceService, error) {

	if err := ValidateKserveService(service); err != nil {
		return nil, err
	}

	resources, err := types.CreateResources(service)
	if err != nil {
		return nil, err
	}

	// Determine protocol version based on service configuration, default to v1 if not specified or invalid
	protocolV := protocolVersion(service)
	controller := false
	blockOwnerDeletion := true

	// Define InferenceService
	return &servingv1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildKserveName(service.Name),
			Namespace: service.Namespace,

			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion:         "serving.knative.dev/v1",
					Kind:               "Service",
					Name:               service.Name,
					UID:                knSvc.UID,
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
		Spec: servingv1beta1.InferenceServiceSpec{
			Predictor: servingv1beta1.PredictorSpec{
				Model: &servingv1beta1.ModelSpec{
					ModelFormat: servingv1beta1.ModelFormat{
						Name: service.Kserve.ModelFormat,
					},
					PredictorExtensionSpec: servingv1beta1.PredictorExtensionSpec{
						StorageURI:      &service.Kserve.StorageUri,
						ProtocolVersion: &protocolV,
					},
				},
				ComponentExtensionSpec: servingv1beta1.ComponentExtensionSpec{
					MinReplicas: &service.Kserve.MinScale,
					MaxReplicas: service.Kserve.MaxScale,
				},
				PodSpec: servingv1beta1.PodSpec{
					Resources: &resources,
				},
			},
		},
	}, nil
}

func UpdateKserveInferenceServiceDefinition(service *types.Service, updatedKnSvc *knv1.Service, oldIsvc *servingv1beta1.InferenceService) (*servingv1beta1.InferenceService, error) {

	if err := ValidateKserveService(service); err != nil {
		return nil, err
	}

	resources, err := types.CreateResources(service)
	if err != nil {
		return nil, err
	}

	// Determine protocol version based on service configuration, default to v1 if not specified or invalid
	protocolV := protocolVersion(service)

	// Revise InferenceService
	oldIsvc.Spec.Predictor.Model.ModelFormat.Name = service.Kserve.ModelFormat
	oldIsvc.Spec.Predictor.Model.StorageURI = &service.Kserve.StorageUri
	oldIsvc.Spec.Predictor.Model.ProtocolVersion = &protocolV
	oldIsvc.Spec.Predictor.ComponentExtensionSpec.MinReplicas = &service.Kserve.MinScale
	oldIsvc.Spec.Predictor.ComponentExtensionSpec.MaxReplicas = service.Kserve.MaxScale
	oldIsvc.Spec.Predictor.PodSpec.Resources = &resources
	return oldIsvc, nil
}

// CreateKserveInferenceService creates a KServe InferenceService based on the provided service and Knative service.
// It set an OwnerReference to the Knative service, so if the Knative service is deleted the KServe InferenceService will be automatically deleted by Kubernetes garbage collection.
// It returns the created InferenceService or an error if the creation fails.
func CreateKserveInferenceService(kserveclient *kserveclient.Clientset, service *types.Service, knativeService *knv1.Service) (*servingv1beta1.InferenceService, error) {

	isvc, err := NewKserveInferenceServiceDefinition(service, knativeService)
	if err != nil {
		return nil, err
	}
	// Create InferenceService
	isvc, err = kserveclient.ServingV1beta1().InferenceServices(isvc.Namespace).Create(context.Background(), isvc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create InferenceService: %v", err)
	}
	return isvc, nil
}

func UpdateKserveInferenceService(kserveclient *kserveclient.Clientset, service *types.Service, knativeService *knv1.Service, oldIsvc *servingv1beta1.InferenceService) (*servingv1beta1.InferenceService, error) {
	revisedIsvc, err := UpdateKserveInferenceServiceDefinition(service, knativeService, oldIsvc)
	if err != nil {
		return nil, err
	}
	// Update InferenceService
	updatedIsvc, err := kserveclient.ServingV1beta1().InferenceServices(revisedIsvc.Namespace).Update(context.Background(), revisedIsvc, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update InferenceService: %v", err)
	}
	return updatedIsvc, nil
}

func GetKserveInferenceService(kserveclient *kserveclient.Clientset, service *types.Service, namespace string) (*servingv1beta1.InferenceService, error) {
	// Get InferenceService
	isvc, err := kserveclient.ServingV1beta1().InferenceServices(namespace).Get(context.Background(), buildKserveName(service.Name), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get InferenceService: %v", err)
	}
	return isvc, nil
}

func DeleteKserveInferenceService(kserveclient *kserveclient.Clientset, serviceName, namespace string) error {
	name := buildKserveName(serviceName)
	// Delete InferenceService
	return kserveclient.ServingV1beta1().InferenceServices(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

func IsKserveService(service *types.Service) bool {
	// If the service has KServe configuration
	if service.Kserve.ModelFormat == "" || service.Kserve.StorageUri == "" {
		return false
	}
	return true
}

func buildKserveName(serviceName string) string {
	// TO DO
	return serviceName
}

func KservePredictor(serviceName string) string {
	if serviceName == "" {
		return ""
	}
	return serviceName + "-predictor"
}

// Helper function to determine the protocol version for KServe based on service configuration
// Defaults to "v1" if not specified or invalid
func protocolVersion(service *types.Service) constants.InferenceServiceProtocol {
	switch {
	case service.Kserve.APIVersion == "v1" && !onlyProtocolV2(service):
		return constants.ProtocolV1
	case service.Kserve.APIVersion == "v2" || onlyProtocolV2(service):
		return constants.ProtocolV2
	default:
		return constants.ProtocolV1
	}
}

// onlyProtocolV2 checks if the service runtime supports only Protocol V2
func onlyProtocolV2(service *types.Service) bool {
	return service.Kserve.ModelFormat == "onnx"
}

func validModelFormat(format string) bool {
	switch format {
	case "onnx", "sklearn", "xgboost", "pytorch", "tensorflow", "triton":
		return true
	default:
		return false
	}
}
