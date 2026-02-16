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

func NewKserveInferenceService(service *types.Service, knSvc *knv1.Service) (*servingv1beta1.InferenceService, error) {

	if !IsKserveService(service) {
		return nil, fmt.Errorf("service does not have KServe configuration")
	}

	resources, err := types.CreateResources(service)
	if err != nil {
		return nil, err
	}

	protocolV := constants.ProtocolV1
	/* Disabled at the moment
	switch service.Kserve.APIVersion {
	case "v2":
		protocolV = constants.ProtocolV2
	}
	*/
	controller := false
	blockOwnerDeletion := true

	// Define InferenceService
	return &servingv1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deriveKserveName(service.Name),
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

// CreateKserveInferenceService creates a KServe InferenceService based on the provided service and Knative service.
// It set an OwnerReference to the Knative service, so if the Knative service is deleted the KServe InferenceService will be automatically deleted by Kubernetes garbage collection.
// It returns the created InferenceService or an error if the creation fails.
func CreateKserveInferenceService(kserveclient *kserveclient.Clientset, service *types.Service, knativeService *knv1.Service) (*servingv1beta1.InferenceService, error) {

	isvc, err := NewKserveInferenceService(service, knativeService)
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

func DeleteKserveInferenceService(kserveclient *kserveclient.Clientset, serviceName, namespace string) error {
	name := deriveKserveName(serviceName)
	// Create InferenceService
	return kserveclient.ServingV1beta1().InferenceServices(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

func IsKserveService(service *types.Service) bool {
	// If the service has KServe configuration
	if service.Kserve.ModelFormat == "" || service.Kserve.StorageUri == "" {
		return false
	}
	return true
}

func deriveKserveName(serviceName string) string {
	// TO DO
	return serviceName
}

func KservePredictor(serviceName string) string {
	return serviceName + "-predictor"
}
