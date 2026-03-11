package utils

import (
	"testing"

	oscarType "github.com/grycap/oscar/v3/pkg/types"
	servingv1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	"github.com/kserve/kserve/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types" // for UID
	knv1 "knative.dev/serving/pkg/apis/serving/v1"
)

// knativeServiceWithUID returns a minimal Knative service with the given UID.
func knativeServiceWithUID(uid types.UID) *knv1.Service {
	return &knv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-kn-svc",
			Namespace: "default",
			UID:       uid,
		},
	}
}

// kserveService returns a types.Service with valid KServe configuration.
func kserveService() *oscarType.Service {
	minScale := int32(1)
	maxScale := int32(3)
	return &oscarType.Service{
		Name:      "my-service",
		Namespace: "oscar-svc",
		CPU:       "500m",
		Memory:    "1Gi",
		Kserve: oscarType.Kserve{
			ModelFormat: "sklearn",
			StorageUri:  "s3://my-bucket/model",
			MinScale:    minScale,
			MaxScale:    maxScale,
			APIVersion:  "v1",
		},
	}
}

// ─── IsKserveService ─────────────────────────────────────────────────────────

func TestIsKserveService_ValidConfig(t *testing.T) {
	svc := kserveService()
	if !IsKserveService(svc) {
		t.Error("expected IsKserveService to return true for a service with ModelFormat and StorageUri set")
	}
}

func TestIsKserveService_MissingModelFormat(t *testing.T) {
	svc := kserveService()
	svc.Kserve.ModelFormat = ""
	if IsKserveService(svc) {
		t.Error("expected IsKserveService to return false when ModelFormat is empty")
	}
}

func TestIsKserveService_MissingStorageUri(t *testing.T) {
	svc := kserveService()
	svc.Kserve.StorageUri = ""
	if IsKserveService(svc) {
		t.Error("expected IsKserveService to return false when StorageUri is empty")
	}
}

func TestIsKserveService_BothMissing(t *testing.T) {
	svc := &oscarType.Service{}
	if IsKserveService(svc) {
		t.Error("expected IsKserveService to return false when both ModelFormat and StorageUri are empty")
	}
}

// ─── KservePredictor ─────────────────────────────────────────────────────────

func TestKservePredictor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple name", "my-service", "my-service-predictor"},
		{"empty name", "", ""},
		{"name with numbers", "svc123", "svc123-predictor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KservePredictor(tt.input)
			if got != tt.expected {
				t.Errorf("KservePredictor(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// ─── NewKserveInferenceServiceDefinition ─────────────────────────────────────

func TestNewKserveInferenceServiceDefinition_Success(t *testing.T) {
	svc := kserveService()
	uid := types.UID("test-uid-1234")
	knSvc := knativeServiceWithUID(uid)

	isvc, err := NewKserveInferenceServiceDefinition(svc, knSvc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Name / namespace
	if isvc.Name != svc.Name {
		t.Errorf("isvc.Name = %q, want %q", isvc.Name, svc.Name)
	}
	if isvc.Namespace != svc.Namespace {
		t.Errorf("isvc.Namespace = %q, want %q", isvc.Namespace, svc.Namespace)
	}

	// OwnerReference points to the Knative service
	if len(isvc.OwnerReferences) != 1 {
		t.Fatalf("expected 1 OwnerReference, got %d", len(isvc.OwnerReferences))
	}
	ownerRef := isvc.OwnerReferences[0]
	if ownerRef.UID != uid {
		t.Errorf("OwnerReference.UID = %q, want %q", ownerRef.UID, uid)
	}
	if ownerRef.Kind != "Service" {
		t.Errorf("OwnerReference.Kind = %q, want Service", ownerRef.Kind)
	}
	if *ownerRef.Controller {
		t.Error("OwnerReference.Controller should be false")
	}
	if !*ownerRef.BlockOwnerDeletion {
		t.Error("OwnerReference.BlockOwnerDeletion should be true")
	}

	// Predictor model format
	if isvc.Spec.Predictor.Model.ModelFormat.Name != svc.Kserve.ModelFormat {
		t.Errorf("ModelFormat = %q, want %q", isvc.Spec.Predictor.Model.ModelFormat.Name, svc.Kserve.ModelFormat)
	}

	// StorageURI
	if *isvc.Spec.Predictor.Model.StorageURI != svc.Kserve.StorageUri {
		t.Errorf("StorageURI = %q, want %q", *isvc.Spec.Predictor.Model.StorageURI, svc.Kserve.StorageUri)
	}

	// Protocol version defaults to V1
	if *isvc.Spec.Predictor.Model.ProtocolVersion != constants.ProtocolV1 {
		t.Errorf("ProtocolVersion = %v, want %v", *isvc.Spec.Predictor.Model.ProtocolVersion, constants.ProtocolV1)
	}

	// Scale settings
	if *isvc.Spec.Predictor.MinReplicas != svc.Kserve.MinScale {
		t.Errorf("MinReplicas = %d, want %d", *isvc.Spec.Predictor.MinReplicas, svc.Kserve.MinScale)
	}
	if isvc.Spec.Predictor.MaxReplicas != svc.Kserve.MaxScale {
		t.Errorf("MaxReplicas = %d, want %d", isvc.Spec.Predictor.MaxReplicas, svc.Kserve.MaxScale)
	}

	// Resources present
	if isvc.Spec.Predictor.PodSpec.Resources == nil {
		t.Error("PodSpec.Resources should not be nil")
	}
}

func TestNewKserveInferenceServiceDefinition_ProtocolVersion(t *testing.T) {
	knSvc := knativeServiceWithUID("uid")

	tests := []struct {
		protocolVersion string
		input           string
		expected        string
	}{
		{"v1", "v1", "v1"},
		{"v1", "", "v1"},
		{"v2", "v2", "v2"},
	}
	for _, tt := range tests {
		t.Run(tt.protocolVersion, func(t *testing.T) {
			svc := kserveService()
			svc.Kserve.APIVersion = tt.protocolVersion
			isvc, err := NewKserveInferenceServiceDefinition(svc, knSvc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(*isvc.Spec.Predictor.Model.ProtocolVersion) != tt.expected {
				t.Errorf("APIVersion = %q, want %q", *isvc.Spec.Predictor.Model.ProtocolVersion, tt.expected)
			}
		})
	}
}

func TestNewKserveInferenceServiceDefinition_NoKserveConfig(t *testing.T) {
	svc := &oscarType.Service{Name: "no-kserve"}
	knSvc := knativeServiceWithUID("uid")

	_, err := NewKserveInferenceServiceDefinition(svc, knSvc)
	if err == nil {
		t.Error("expected error when service has no KServe configuration, got nil")
	}
}

func TestNewKserveInferenceServiceDefinition_InvalidCPU(t *testing.T) {
	svc := kserveService()
	svc.CPU = "not-valid-cpu"
	knSvc := knativeServiceWithUID("uid")

	_, err := NewKserveInferenceServiceDefinition(svc, knSvc)
	if err == nil {
		t.Error("expected error due to invalid CPU quantity, got nil")
	}
}

func TestNewKserveInferenceServiceDefinition_InvalidMemory(t *testing.T) {
	svc := kserveService()
	svc.Memory = "bad-mem"
	knSvc := knativeServiceWithUID("uid")

	_, err := NewKserveInferenceServiceDefinition(svc, knSvc)
	if err == nil {
		t.Error("expected error due to invalid memory quantity, got nil")
	}
}

// ─── UpdateKserveInferenceServiceDefinition ───────────────────────────────────

func TestUpdateKserveInferenceServiceDefinition_Success(t *testing.T) {
	original := kserveService()
	knSvc := knativeServiceWithUID("uid-update")

	// Create an initial isvc from the original service
	oldIsvc, err := NewKserveInferenceServiceDefinition(original, knSvc)
	if err != nil {
		t.Fatalf("setup error: %v", err)
	}

	// Build an updated service
	updated := kserveService()
	updated.Kserve.ModelFormat = "tensorflow"
	updated.Kserve.StorageUri = "s3://new-bucket/model"
	updated.Kserve.MinScale = 2
	updated.Kserve.MaxScale = 5
	updated.CPU = "1"
	updated.Memory = "2Gi"
	updated.Kserve.APIVersion = "v2"

	result, err := UpdateKserveInferenceServiceDefinition(updated, knSvc, oldIsvc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Spec.Predictor.Model.ModelFormat.Name != "tensorflow" {
		t.Errorf("ModelFormat = %q, want tensorflow", result.Spec.Predictor.Model.ModelFormat.Name)
	}
	if *result.Spec.Predictor.Model.StorageURI != "s3://new-bucket/model" {
		t.Errorf("StorageURI = %q, want s3://new-bucket/model", *result.Spec.Predictor.Model.StorageURI)
	}
	if *result.Spec.Predictor.MinReplicas != 2 {
		t.Errorf("MinReplicas = %d, want 2", *result.Spec.Predictor.MinReplicas)
	}
	if result.Spec.Predictor.MaxReplicas != 5 {
		t.Errorf("MaxReplicas = %d, want 5", result.Spec.Predictor.MaxReplicas)
	}
	if *result.Spec.Predictor.Model.ProtocolVersion != constants.ProtocolV2 {
		t.Errorf("ProtocolVersion = %v, want %v", *result.Spec.Predictor.Model.ProtocolVersion, constants.ProtocolV2)
	}
	if result.Spec.Predictor.PodSpec.Resources == nil {
		t.Error("PodSpec.Resources should not be nil after update")
	}
}

func TestUpdateKserveInferenceServiceDefinition_ProtocolVersion(t *testing.T) {
	original := kserveService()
	knSvc := knativeServiceWithUID("uid-update")
	// Create an initial isvc from the original service
	oldIsvc, err := NewKserveInferenceServiceDefinition(original, knSvc)
	if err != nil {
		t.Fatalf("setup error: %v", err)
	}

	tests := []struct {
		protocolVersion string
		input           string
		expected        string
	}{
		{"v1", "v1", "v1"},
		{"v1", "", "v1"},
		{"v2", "v2", "v2"},
	}
	for _, tt := range tests {
		t.Run(tt.protocolVersion, func(t *testing.T) {
			svc := kserveService()
			svc.Kserve.APIVersion = tt.protocolVersion
			isvc, err := UpdateKserveInferenceServiceDefinition(svc, knSvc, oldIsvc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(*isvc.Spec.Predictor.Model.ProtocolVersion) != tt.expected {
				t.Errorf("APIVersion = %q, want %q", *isvc.Spec.Predictor.Model.ProtocolVersion, tt.expected)
			}
		})
	}
}

func TestUpdateKserveInferenceServiceDefinition_NoKserveConfig(t *testing.T) {
	svc := &oscarType.Service{Name: "no-kserve"}
	knSvc := knativeServiceWithUID("uid")
	oldIsvc := &servingv1beta1.InferenceService{}

	_, err := UpdateKserveInferenceServiceDefinition(svc, knSvc, oldIsvc)
	if err == nil {
		t.Error("expected error when service has no KServe configuration, got nil")
	}
}

func TestUpdateKserveInferenceServiceDefinition_InvalidCPU(t *testing.T) {
	svc := kserveService()
	svc.CPU = "not-valid-cpu"
	knSvc := knativeServiceWithUID("uid")
	oldIsvc := &servingv1beta1.InferenceService{}

	_, err := UpdateKserveInferenceServiceDefinition(svc, knSvc, oldIsvc)
	if err == nil {
		t.Error("expected error due to invalid CPU quantity, got nil")
	}
}

// ─── onlyProtocolV2 ──────────────────────────────────────────────────────────

func TestOnlyProtocolV2_Onnx(t *testing.T) {
	svc := kserveService()
	svc.Kserve.ModelFormat = "onnx"
	if !onlyProtocolV2(svc) {
		t.Error("expected onlyProtocolV2 to return true for onnx model format")
	}
}

func TestOnlyProtocolV2_OtherFormats(t *testing.T) {
	for _, format := range []string{"sklearn", "xgboost", "pytorch", "tensorflow", "triton"} {
		svc := kserveService()
		svc.Kserve.ModelFormat = format
		if onlyProtocolV2(svc) {
			t.Errorf("expected onlyProtocolV2 to return false for model format %q", format)
		}
	}
}

// ─── validModelFormat ────────────────────────────────────────────────────────

func TestValidModelFormat(t *testing.T) {
	tests := []struct {
		format string
		valid  bool
	}{
		{"onnx", true},
		{"sklearn", true},
		{"xgboost", true},
		{"pytorch", true},
		{"tensorflow", true},
		{"triton", true},
		{"", false},
		{"unknown", false},
		{"SKLEARN", false},
		{"torch", false},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			got := validModelFormat(tt.format)
			if got != tt.valid {
				t.Errorf("validModelFormat(%q) = %v, want %v", tt.format, got, tt.valid)
			}
		})
	}
}

// ─── protocolVersion ─────────────────────────────────────────────────────────

func TestProtocolVersion(t *testing.T) {
	tests := []struct {
		name        string
		modelFormat string
		apiVersion  string
		expected    constants.InferenceServiceProtocol
	}{
		{"v1 explicit", "sklearn", "v1", constants.ProtocolV1},
		{"v2 explicit", "sklearn", "v2", constants.ProtocolV2},
		{"default to v1 when empty", "sklearn", "", constants.ProtocolV1},
		{"onnx forces v2 regardless of apiVersion", "onnx", "v1", constants.ProtocolV2},
		{"onnx forces v2 when apiVersion empty", "onnx", "", constants.ProtocolV2},
		{"invalid apiVersion defaults to v1", "sklearn", "v3", constants.ProtocolV1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := kserveService()
			svc.Kserve.ModelFormat = tt.modelFormat
			svc.Kserve.APIVersion = tt.apiVersion
			got := protocolVersion(svc)
			if got != tt.expected {
				t.Errorf("protocolVersion() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ─── validateKserveService ───────────────────────────────────────────────────

func TestValidateKserveService_Valid(t *testing.T) {
	svc := kserveService()
	if err := ValidateKserveService(svc); err != nil {
		t.Errorf("unexpected error for valid service: %v", err)
	}
}

func TestValidateKserveService_NoKserveConfig(t *testing.T) {
	svc := &oscarType.Service{Name: "bare"}
	if err := ValidateKserveService(svc); err == nil {
		t.Error("expected error when service has no KServe configuration, got nil")
	}
}

func TestValidateKserveService_InvalidModelFormat(t *testing.T) {
	svc := kserveService()
	svc.Kserve.ModelFormat = "unsupported-format"
	if err := ValidateKserveService(svc); err == nil {
		t.Error("expected error for invalid model format, got nil")
	}
}

func TestValidateKserveService_MissingStorageUri(t *testing.T) {
	svc := kserveService()
	svc.Kserve.StorageUri = ""
	if err := ValidateKserveService(svc); err == nil {
		t.Error("expected error when StorageUri is empty, got nil")
	}
}
