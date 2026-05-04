package utils

import (
	"testing"

	oscarType "github.com/grycap/oscar/v3/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types" // for UID
	knv1 "knative.dev/serving/pkg/apis/serving/v1"
)

// knativeServiceWithUID returns a minimal Knative service with the given UID.
func knativeServiceWithUID(uid types.UID) *knv1.Service {
	return &knv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-kn-svc",
			Namespace: "oscar-svc",
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
		Kserve: &oscarType.Kserve{
			ModelFormat: "sklearn",
			StorageUri:  "s3://my-bucket/model",
			MinScale:    minScale,
			MaxScale:    maxScale,
			APIVersion:  "v1",
			CPU:         "1.0",
			Memory:      "2Gi",
		},
	}
}

// ─── helpers for unstructured field access ────────────────────────────────────

func getNestedString(t *testing.T, obj *unstructured.Unstructured, fields ...string) string {
	t.Helper()
	val, found, err := unstructured.NestedString(obj.Object, fields...)
	if err != nil || !found {
		t.Errorf("field %v not found or error: %v", fields, err)
	}
	return val
}

func getNestedMap(t *testing.T, obj *unstructured.Unstructured, fields ...string) map[string]any {
	t.Helper()
	val, found, err := unstructured.NestedMap(obj.Object, fields...)
	if err != nil || !found {
		t.Errorf("field %v not found or error: %v", fields, err)
	}
	return val
}

func getNestedInt64(t *testing.T, obj *unstructured.Unstructured, fields ...string) int64 {
	t.Helper()
	val, found, err := unstructured.NestedInt64(obj.Object, fields...)
	if err != nil || !found {
		t.Errorf("field %v not found or error: %v", fields, err)
	}
	return val
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

// ─── NewKserveInferenceServiceDefinition ─────────────────────────────────────

func TestNewKserveInferenceServiceDefinition_Success(t *testing.T) {
	svc := kserveService()
	uid := types.UID("test-uid-1234")
	knSvc := knativeServiceWithUID(uid)

	isvc, err := NewKserveInferenceServiceDefinition(svc, knSvc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// apiVersion / kind
	if v := getNestedString(t, isvc, "apiVersion"); v != "serving.kserve.io/v1beta1" {
		t.Errorf("apiVersion = %q, want serving.kserve.io/v1beta1", v)
	}
	if v := getNestedString(t, isvc, "kind"); v != "InferenceService" {
		t.Errorf("kind = %q, want InferenceService", v)
	}

	// name / namespace
	if v := getNestedString(t, isvc, "metadata", "name"); v != buildKserveName(svc.Name) {
		t.Errorf("metadata.name = %q, want %q", v, buildKserveName(svc.Name))
	}
	if v := getNestedString(t, isvc, "metadata", "namespace"); v != knSvc.Namespace {
		t.Errorf("metadata.namespace = %q, want %q", v, knSvc.Namespace)
	}

	// ownerReferences
	ownerRefs, found, err := unstructured.NestedSlice(isvc.Object, "metadata", "ownerReferences")
	if err != nil || !found || len(ownerRefs) != 1 {
		t.Fatalf("expected 1 ownerReference, got %v (found=%v, err=%v)", len(ownerRefs), found, err)
	}
	ownerRef := ownerRefs[0].(map[string]any)
	if ownerRef["uid"] != string(uid) {
		t.Errorf("ownerReference.uid = %v, want %v", ownerRef["uid"], uid)
	}
	if ownerRef["kind"] != "Service" {
		t.Errorf("ownerReference.kind = %v, want Service", ownerRef["kind"])
	}

	// predictor model fields
	if v := getNestedString(t, isvc, "spec", "predictor", "model", "modelFormat", "name"); v != svc.Kserve.ModelFormat {
		t.Errorf("modelFormat.name = %q, want %q", v, svc.Kserve.ModelFormat)
	}
	if v := getNestedString(t, isvc, "spec", "predictor", "model", "storageUri"); v != svc.Kserve.StorageUri {
		t.Errorf("storageUri = %q, want %q", v, svc.Kserve.StorageUri)
	}
	if v := getNestedString(t, isvc, "spec", "predictor", "model", "protocolVersion"); v != "v1" {
		t.Errorf("protocolVersion = %q, want %q", v, "v1")
	}

	// scale
	if v := getNestedInt64(t, isvc, "spec", "predictor", "minReplicas"); int32(v) != svc.Kserve.MinScale {
		t.Errorf("minReplicas = %d, want %d", v, svc.Kserve.MinScale)
	}
	if v := getNestedInt64(t, isvc, "spec", "predictor", "maxReplicas"); int32(v) != svc.Kserve.MaxScale {
		t.Errorf("maxReplicas = %d, want %d", v, svc.Kserve.MaxScale)
	}

	// resources present
	getNestedMap(t, isvc, "spec", "predictor", "model", "resources")
}

func TestNewKserveInferenceServiceDefinition_ProtocolVersion(t *testing.T) {
	knSvc := knativeServiceWithUID("uid")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"v1 explicit", "v1", "v1"},
		{"default to v1", "", "v1"},
		{"v2 explicit", "v2", "v2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := kserveService()
			svc.Kserve.APIVersion = tt.input
			isvc, err := NewKserveInferenceServiceDefinition(svc, knSvc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := getNestedString(t, isvc, "spec", "predictor", "model", "protocolVersion")
			if got != tt.expected {
				t.Errorf("protocolVersion = %q, want %q", got, tt.expected)
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
	svc.Kserve.CPU = "not-valid-cpu"
	knSvc := knativeServiceWithUID("uid")

	_, err := NewKserveInferenceServiceDefinition(svc, knSvc)
	if err == nil {
		t.Error("expected error due to invalid CPU quantity, got nil")
	}
}

func TestNewKserveInferenceServiceDefinition_InvalidMemory(t *testing.T) {
	svc := kserveService()
	svc.Kserve.Memory = "bad-mem"
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

	oldIsvc, err := NewKserveInferenceServiceDefinition(original, knSvc)
	if err != nil {
		t.Fatalf("setup error: %v", err)
	}

	updated := kserveService()
	updated.Kserve.ModelFormat = "tensorflow"
	updated.Kserve.StorageUri = "s3://new-bucket/model"
	updated.Kserve.MinScale = 2
	updated.Kserve.MaxScale = 5
	updated.Kserve.CPU = "1"
	updated.Kserve.Memory = "2Gi"
	updated.Kserve.APIVersion = "v2"

	result, err := UpdateKserveInferenceServiceDefinition(updated, oldIsvc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v := getNestedString(t, result, "spec", "predictor", "model", "modelFormat", "name"); v != "tensorflow" {
		t.Errorf("modelFormat.name = %q, want tensorflow", v)
	}
	if v := getNestedString(t, result, "spec", "predictor", "model", "storageUri"); v != "s3://new-bucket/model" {
		t.Errorf("storageUri = %q, want s3://new-bucket/model", v)
	}
	if v := getNestedInt64(t, result, "spec", "predictor", "minReplicas"); int32(v) != 2 {
		t.Errorf("minReplicas = %d, want 2", v)
	}
	if v := getNestedInt64(t, result, "spec", "predictor", "maxReplicas"); int32(v) != 5 {
		t.Errorf("maxReplicas = %d, want 5", v)
	}
	if v := getNestedString(t, result, "spec", "predictor", "model", "protocolVersion"); v != "v2" {
		t.Errorf("protocolVersion = %q, want %q", v, "v2")
	}
	getNestedMap(t, result, "spec", "predictor", "model", "resources")
}

func TestUpdateKserveInferenceServiceDefinition_ProtocolVersion(t *testing.T) {
	original := kserveService()
	knSvc := knativeServiceWithUID("uid-update")

	oldIsvc, err := NewKserveInferenceServiceDefinition(original, knSvc)
	if err != nil {
		t.Fatalf("setup error: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"v1 explicit", "v1", "v1"},
		{"default to v1", "", "v1"},
		{"v2 explicit", "v2", "v2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := kserveService()
			svc.Kserve.APIVersion = tt.input
			isvc, err := UpdateKserveInferenceServiceDefinition(svc, oldIsvc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := getNestedString(t, isvc, "spec", "predictor", "model", "protocolVersion")
			if got != tt.expected {
				t.Errorf("protocolVersion = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestUpdateKserveInferenceServiceDefinition_NoKserveConfig(t *testing.T) {
	svc := &oscarType.Service{Name: "no-kserve"}
	oldIsvc := &unstructured.Unstructured{Object: map[string]any{}}

	_, err := UpdateKserveInferenceServiceDefinition(svc, oldIsvc)
	if err == nil {
		t.Error("expected error when service has no KServe configuration, got nil")
	}
}

func TestUpdateKserveInferenceServiceDefinition_InvalidCPU(t *testing.T) {
	svc := kserveService()
	svc.Kserve.CPU = "not-valid-cpu"
	oldIsvc := &unstructured.Unstructured{Object: map[string]any{}}

	_, err := UpdateKserveInferenceServiceDefinition(svc, oldIsvc)
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
		service *oscarType.Service
		valid   bool
	}{
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: "onnx"}}, true},
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: "sklearn"}}, true},
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: "xgboost"}}, true},
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: "pytorch"}}, true},
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: "tensorflow"}}, true},
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: "triton"}}, true},
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: ""}}, false},
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: "unknown"}}, false},
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: "SKLEARN"}}, false},
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: "torch"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.service.Kserve.ModelFormat, func(t *testing.T) {
			got := validModelFormat(tt.service)
			if got != tt.valid {
				t.Errorf("validModelFormat(%q) = %v, want %v", tt.service.Kserve.ModelFormat, got, tt.valid)
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
		expected    string
	}{
		{"v1 explicit", "sklearn", "v1", "v1"},
		{"v2 explicit", "sklearn", "v2", "v2"},
		{"default to v1 when empty", "sklearn", "", "v1"},
		{"onnx forces v2 regardless of apiVersion", "onnx", "v1", "v2"},
		{"onnx forces v2 when apiVersion empty", "onnx", "", "v2"},
		{"invalid apiVersion defaults to v1", "sklearn", "v3", "v1"},
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

// ─── ValidateKserveService ───────────────────────────────────────────────────

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
