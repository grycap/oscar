package utils

import (
	"context"
	"fmt"

	"github.com/grycap/oscar/v3/pkg/types"
	servingv1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	kserveclient "github.com/kserve/kserve/pkg/client/clientset/versioned"
	"github.com/kserve/kserve/pkg/constants"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const {
	OSCAR_KSERVE_SERVICE_IMAGE := "docker.io/curlimages/curl:8.18.0",
	OSCAR_KSERVE_SERVICE_SCRIPT := "",
}

func NewKserveInferenceService(service *types.Service) (*servingv1beta1.InferenceService, error) {

	if service.Kserve.ModelFormat == "" || service.Kserve.StorageUri == "" {
		return nil, fmt.Errorf("service does not have KServe configuration")
	}

	protocolV := constants.ProtocolV1
	/* Disabled at the moment
	switch service.Kserve.APIVersion {
	case "v2":
		protocolV = constants.ProtocolV2
	}
	*/

	// Define InferenceService
	return &servingv1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: service.Namespace,
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

						Container: v1.Container{
							Name: "foo",
							Env: []v1.EnvVar{
								{
									Name:  "STORAGE_URI",
									Value: service.Kserve.StorageUri,
								},
							},
							Resources: v1.ResourceRequirements{},
						},
					},
				},
				ComponentExtensionSpec: servingv1beta1.ComponentExtensionSpec{
					MinReplicas: &service.Kserve.MinScale,
					MaxReplicas: service.Kserve.MaxScale,
				},
				PodSpec: servingv1beta1.PodSpec{
					Resources: &v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse(service.CPU),
							v1.ResourceMemory: resource.MustParse(service.Memory),
						},
					},
				},
			},
		},
	}, nil
}

/*
func NewCustomKserveInferenceService(service *types.Service) (*servingv1beta1.InferenceService, error) {

		if service.Kserve == (types.Kserve{}) {
			return nil, fmt.Errorf("service does not have KServe configuration")
		}
		//protocolV2 := constants.ProtocolV2
		minReplicas := int32(1)

		// Define InferenceService
		return &servingv1beta1.InferenceService{

			ObjectMeta: metav1.ObjectMeta{
				Name:      service.Name,
				Namespace: service.Namespace,
			},
			Spec: servingv1beta1.InferenceServiceSpec{
				Predictor: servingv1beta1.PredictorSpec{
					ComponentExtensionSpec: servingv1beta1.ComponentExtensionSpec{
						MinReplicas: &minReplicas,
					},
					PodSpec: servingv1beta1.PodSpec{
						Containers: []v1.Container{
							{
								Env: []v1.EnvVar{
									{
										Name:  "PROTOCOL",
										Value: "v1",
									},
								},
							},
						},
					},
					WorkerSpec: &servingv1beta1.WorkerSpec{},
				},
			},
		}, nil
	}
*/
func CreateKserveInferenceService(kserveclient *kserveclient.Clientset, service *types.Service) error {

	isvc, err := NewKserveInferenceService(service)
	if err != nil {
		return err
	}
	// Create InferenceService
	_, err = kserveclient.ServingV1beta1().InferenceServices(isvc.Namespace).Create(context.Background(), isvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create InferenceService: %v", err)
	}
	return nil
}

func IsKserveService(service *types.Service) bool {
	// If the service has KServe configuration
	if service.Kserve.ModelFormat == "" {
		return false
	}
	return true
}

// If the service has KServe configuration, set script and image for KServe
func ifKserveService(service *types.Service) {
	// If the service has KServe configuration
	if service.Kserve.ModelFormat == "" {
		return
	}

	// TO DO
	service.Script = ""
	service.Image = "docker.io/curlimages/curl:8.18.0"

}
