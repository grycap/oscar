package utils

import (
	"fmt"
	"strings"
	"testing"

	oscarType "github.com/grycap/oscar/v3/pkg/types"
	corev1 "k8s.io/api/core/v1"
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
		Token:     "test-token",
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

func llmKserveService() *oscarType.Service {
	return &oscarType.Service{
		Name:      "my-llm-service",
		Namespace: "oscar-svc",
		Token:     "llm-token",
		Kserve: &oscarType.Kserve{
			ModelFormat: "llm",
			StorageUri:  "s3://my-bucket/llm-model",
			MinScale:    2,
			MaxScale:    4,
			CPU:         "1",
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

// getRawNested walks the object map along fields and returns the raw value.
func getRawNested(t *testing.T, obj *unstructured.Unstructured, fields ...string) any {
	t.Helper()
	var cur any = obj.Object
	for _, f := range fields {
		m, ok := cur.(map[string]any)
		if !ok {
			t.Errorf("field %v: unexpected type at %q", fields, f)
			return nil
		}
		cur, ok = m[f]
		if !ok {
			t.Errorf("field %v not found at %q", fields, f)
			return nil
		}
	}
	return cur
}

func getNestedMap(t *testing.T, obj *unstructured.Unstructured, fields ...string) map[string]any {
	t.Helper()
	val, found, err := unstructured.NestedMap(obj.Object, fields...)
	if err != nil || !found {
		t.Errorf("field %v not found or error: %v", fields, err)
	}
	return val
}

func getNestedInt32(t *testing.T, obj *unstructured.Unstructured, fields ...string) int32 {
	t.Helper()
	v := getRawNested(t, obj, fields...)
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int32:
		return n
	case int64:
		return int32(n)
	}
	t.Errorf("field %v: unexpected type %T", fields, v)
	return 0
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

	isvc, err := NewKserveInferenceServiceDefinition(svc, knSvc, &oscarType.Config{})
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

	// ownerReferences — stored as typed []metav1.OwnerReference (not []interface{}),
	// so read directly from the raw map via type assertion.
	rawMeta := isvc.Object["metadata"].(map[string]any)
	ownerRefs := rawMeta["ownerReferences"].([]metav1.OwnerReference)
	if len(ownerRefs) != 1 {
		t.Fatalf("expected 1 ownerReference, got %d", len(ownerRefs))
	}
	if ownerRefs[0].UID != uid {
		t.Errorf("ownerReference.uid = %v, want %v", ownerRefs[0].UID, uid)
	}
	if ownerRefs[0].Kind != "Service" {
		t.Errorf("ownerReference.kind = %v, want Service", ownerRefs[0].Kind)
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
	if v := getNestedInt32(t, isvc, "spec", "predictor", "minReplicas"); v != svc.Kserve.MinScale {
		t.Errorf("minReplicas = %d, want %d", v, svc.Kserve.MinScale)
	}
	if v := getNestedInt32(t, isvc, "spec", "predictor", "maxReplicas"); v != svc.Kserve.MaxScale {
		t.Errorf("maxReplicas = %d, want %d", v, svc.Kserve.MaxScale)
	}

	// resources present
	if getRawNested(t, isvc, "spec", "predictor", "model", "resources") == nil {
		t.Error("expected resources to be set")
	}
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
			isvc, err := NewKserveInferenceServiceDefinition(svc, knSvc, &oscarType.Config{})
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

	_, err := NewKserveInferenceServiceDefinition(svc, knSvc, &oscarType.Config{})
	if err == nil {
		t.Error("expected error when service has no KServe configuration, got nil")
	}
}

func TestNewKserveInferenceServiceDefinition_InvalidCPU(t *testing.T) {
	svc := kserveService()
	svc.Kserve.CPU = "not-valid-cpu"
	knSvc := knativeServiceWithUID("uid")

	_, err := NewKserveInferenceServiceDefinition(svc, knSvc, &oscarType.Config{})
	if err == nil {
		t.Error("expected error due to invalid CPU quantity, got nil")
	}
}

func TestNewKserveInferenceServiceDefinition_InvalidMemory(t *testing.T) {
	svc := kserveService()
	svc.Kserve.Memory = "bad-mem"
	knSvc := knativeServiceWithUID("uid")

	_, err := NewKserveInferenceServiceDefinition(svc, knSvc, &oscarType.Config{})
	if err == nil {
		t.Error("expected error due to invalid memory quantity, got nil")
	}
}

// ─── UpdateKserveInferenceServiceDefinition ───────────────────────────────────

func TestUpdateKserveInferenceServiceDefinition_Success(t *testing.T) {
	original := kserveService()
	knSvc := knativeServiceWithUID("uid-update")

	oldIsvc, err := NewKserveInferenceServiceDefinition(original, knSvc, &oscarType.Config{})
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
	if v := getNestedInt32(t, result, "spec", "predictor", "minReplicas"); v != 2 {
		t.Errorf("minReplicas = %d, want 2", v)
	}
	if v := getNestedInt32(t, result, "spec", "predictor", "maxReplicas"); v != 5 {
		t.Errorf("maxReplicas = %d, want 5", v)
	}
	if v := getNestedString(t, result, "spec", "predictor", "model", "protocolVersion"); v != "v2" {
		t.Errorf("protocolVersion = %q, want %q", v, "v2")
	}
	if getRawNested(t, result, "spec", "predictor", "model", "resources") == nil {
		t.Error("expected resources to be set")
	}
}

func TestUpdateKserveInferenceServiceDefinition_ProtocolVersion(t *testing.T) {
	original := kserveService()
	knSvc := knativeServiceWithUID("uid-update")

	oldIsvc, err := NewKserveInferenceServiceDefinition(original, knSvc, &oscarType.Config{})
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
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: "huggingface"}}, true},
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: ""}}, false}, // empty string should be invalid
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: "unknown"}}, false},
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: "SKLEARN"}}, true}, // case-insensitive
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: "torch"}}, false},
		{&oscarType.Service{Kserve: &oscarType.Kserve{ModelFormat: "llm"}}, true},
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

// ─── getKserveType ────────────────────────────────────────────────────────

func TestGetKserveType(t *testing.T) {
	tests := []struct {
		modelFormat string
		expected    string
	}{
		{"onnx", "predictor"},
		{"sklearn", "predictor"},
		{"xgboost", "predictor"},
		{"pytorch", "predictor"},
		{"tensorflow", "predictor"},
		{"triton", "predictor"},
		{"huggingface", "predictor"},
		{"llm", "llm"},
		{" SKLEARN ", "predictor"},
		{"", ""},
		{"unknown", ""},
	}
	for _, tt := range tests {
		t.Run(tt.modelFormat, func(t *testing.T) {
			got := getKserveType(tt.modelFormat)
			if got != tt.expected {
				t.Errorf("getKserveType(%q) = %v, want %v", tt.modelFormat, got, tt.expected)
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

func TestIsKserveSupported(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		route    string
		expected bool
	}{
		{name: "enabled with httproute", enabled: true, route: "httproute", expected: true},
		{name: "disabled with httproute", enabled: false, route: "httproute", expected: false},
		{name: "enabled with ingress", enabled: true, route: "ingress", expected: false},
		{name: "enabled with mixed-case route", enabled: true, route: "HTTPRoute", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &oscarType.Config{KserveEnable: tt.enabled, ExposedServicesRouteKind: tt.route}
			got := IsKserveSupported(cfg)
			if got != tt.expected {
				t.Errorf("IsKserveSupported() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetKserveFramework(t *testing.T) {
	tests := []struct {
		modelFormat string
		expected    string
	}{
		{modelFormat: "onnx", expected: "triton"},
		{modelFormat: "sklearn", expected: "mlserver"},
		{modelFormat: " triton ", expected: "triton"},
		{modelFormat: "HUGGINGFACE", expected: "vllm"},
		{modelFormat: "llm", expected: "vllm"},
		{modelFormat: "unknown", expected: ""},
		{modelFormat: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.modelFormat, func(t *testing.T) {
			got := getKserveFramework(tt.modelFormat)
			if got != tt.expected {
				t.Errorf("getKserveFramework(%q) = %q, want %q", tt.modelFormat, got, tt.expected)
			}
		})
	}
}

func TestKserveNamingHelpers(t *testing.T) {
	if got := GetKserveLabelSelector("svc"); got != "oscar-app=oscar-svc-ksv-svc" {
		t.Errorf("GetKserveLabelSelector() = %q, want %q", got, "oscar-app=oscar-svc-ksv-svc")
	}

	type nameCase struct {
		name        string
		serviceName string
		modelFormat string
		wantSvcName string
		wantPodName string
	}

	tests := []nameCase{
		{name: "predictor service", serviceName: "demo", modelFormat: "sklearn", wantSvcName: "demo-predictor", wantPodName: "demo-predictor"},
		{name: "llm service", serviceName: "demo", modelFormat: "llm", wantSvcName: "demo-kserve-workload-svc", wantPodName: "demo-kserve"},
		{name: "unknown format", serviceName: "demo", modelFormat: "unknown", wantSvcName: "", wantPodName: ""},
		{name: "empty service name", serviceName: "", modelFormat: "llm", wantSvcName: "", wantPodName: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetKserveSvcName(tt.serviceName, tt.modelFormat); got != tt.wantSvcName {
				t.Errorf("GetKserveSvcName() = %q, want %q", got, tt.wantSvcName)
			}
			if got := GetKservePodAndDplName(tt.serviceName, tt.modelFormat); got != tt.wantPodName {
				t.Errorf("GetKservePodAndDplName() = %q, want %q", got, tt.wantPodName)
			}
		})
	}

	if got := getHTTPRouteName("demo"); got != "demo-route" {
		t.Errorf("getHTTPRouteName() = %q, want %q", got, "demo-route")
	}
	if got := getTraefikCORSMiddlewareName("demo"); got != "demo-cors-mdw" {
		t.Errorf("getTraefikCORSMiddlewareName() = %q, want %q", got, "demo-cors-mdw")
	}
	if got := getTraefikAuthMiddlewareName("demo"); got != "demo-auth-mdw" {
		t.Errorf("getTraefikAuthMiddlewareName() = %q, want %q", got, "demo-auth-mdw")
	}
	if got := getTraefikAuthSecretName("demo"); got != "demo-auth-traefik" {
		t.Errorf("getTraefikAuthSecretName() = %q, want %q", got, "demo-auth-traefik")
	}
}

func TestNormalizeScaleFromKserveService(t *testing.T) {
	tests := []struct {
		name    string
		input   *oscarType.Kserve
		wantMin int32
		wantMax int32
	}{
		{name: "defaults when both zero", input: &oscarType.Kserve{}, wantMin: 0, wantMax: 1},
		{name: "min promoted when max unset", input: &oscarType.Kserve{MinScale: 2}, wantMin: 2, wantMax: 2},
		{name: "max respected", input: &oscarType.Kserve{MaxScale: 5}, wantMin: 0, wantMax: 5},
		{name: "min greater than max", input: &oscarType.Kserve{MinScale: 4, MaxScale: 2}, wantMin: 4, wantMax: 4},
		{name: "both set", input: &oscarType.Kserve{MinScale: 1, MaxScale: 3}, wantMin: 1, wantMax: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax := normalizeScaleFromKserveService(tt.input)
			if gotMin != tt.wantMin || gotMax != tt.wantMax {
				t.Errorf("normalizeScaleFromKserveService() = (%d,%d), want (%d,%d)", gotMin, gotMax, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCreateKserveResources(t *testing.T) {
	tests := []struct {
		name      string
		kserveCfg *oscarType.Kserve
		wantCPU   string
		wantMem   string
		wantGPU   bool
		wantErr   bool
	}{
		{
			name:      "defaults",
			kserveCfg: &oscarType.Kserve{},
			wantCPU:   defaultKserveCpuRequest.String(),
			wantMem:   defaultKserveMemoryRequest.String(),
			wantGPU:   false,
		},
		{
			name:      "custom cpu and memory",
			kserveCfg: &oscarType.Kserve{CPU: "1", Memory: "2Gi"},
			wantCPU:   "1",
			wantMem:   "2Gi",
			wantGPU:   false,
		},
		{
			name:      "gpu enabled",
			kserveCfg: &oscarType.Kserve{EnableGPU: true},
			wantCPU:   defaultKserveCpuRequest.String(),
			wantMem:   defaultKserveMemoryRequest.String(),
			wantGPU:   true,
		},
		{
			name:      "invalid cpu",
			kserveCfg: &oscarType.Kserve{CPU: "bad-cpu"},
			wantErr:   true,
		},
		{
			name:      "invalid memory",
			kserveCfg: &oscarType.Kserve{Memory: "bad-memory"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources, err := createKserveResources(tt.kserveCfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			cpuLimit := resources.Limits[corev1.ResourceCPU]
			if got := cpuLimit.String(); got != tt.wantCPU {
				t.Errorf("limits.cpu = %q, want %q", got, tt.wantCPU)
			}
			cpuRequest := resources.Requests[corev1.ResourceCPU]
			if got := cpuRequest.String(); got != tt.wantCPU {
				t.Errorf("requests.cpu = %q, want %q", got, tt.wantCPU)
			}

			memoryLimit := resources.Limits[corev1.ResourceMemory]
			if got := memoryLimit.String(); got != tt.wantMem {
				t.Errorf("limits.memory = %q, want %q", got, tt.wantMem)
			}
			memoryRequest := resources.Requests[corev1.ResourceMemory]
			if got := memoryRequest.String(); got != tt.wantMem {
				t.Errorf("requests.memory = %q, want %q", got, tt.wantMem)
			}

			_, hasGPU := resources.Limits[corev1.ResourceName("nvidia.com/gpu")]
			if hasGPU != tt.wantGPU {
				t.Errorf("gpu limit present = %v, want %v", hasGPU, tt.wantGPU)
			}
		})
	}
}

func TestCheckKserveUpdate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(oldSvc, newSvc *oscarType.Service)
		wantErr bool
	}{
		{
			name: "same config",
			mutate: func(oldSvc, newSvc *oscarType.Service) {
			},
			wantErr: false,
		},
		{
			name: "token changed",
			mutate: func(oldSvc, newSvc *oscarType.Service) {
				newSvc.Token = "new-token"
			},
			wantErr: true,
		},
		{
			name: "cannot change model format",
			mutate: func(oldSvc, newSvc *oscarType.Service) {
				newSvc.Kserve.ModelFormat = "tensorflow"
			},
			wantErr: true,
		},
		{
			name: "cannot change storage uri",
			mutate: func(oldSvc, newSvc *oscarType.Service) {
				newSvc.Kserve.StorageUri = "s3://another-bucket/model"
			},
			wantErr: true,
		},
		{
			name: "cannot change auth",
			mutate: func(oldSvc, newSvc *oscarType.Service) {
				newSvc.Kserve.SetAuth = true
			},
			wantErr: true,
		},
		{
			name: "both nil kserve config",
			mutate: func(oldSvc, newSvc *oscarType.Service) {
				oldSvc.Kserve = nil
				newSvc.Kserve = nil
			},
			wantErr: false,
		},
		{
			name: "cannot add kserve config",
			mutate: func(oldSvc, newSvc *oscarType.Service) {
				oldSvc.Kserve = nil
			},
			wantErr: true,
		},
		{
			name: "cannot remove kserve config",
			mutate: func(oldSvc, newSvc *oscarType.Service) {
				newSvc.Kserve = nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldSvc := kserveService()
			newSvc := kserveService()
			if tt.mutate != nil {
				tt.mutate(oldSvc, newSvc)
			}

			err := checkKserveUpdate(oldSvc, newSvc)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestUpdateKserveLLMInferenceServiceDefinition(t *testing.T) {
	tests := []struct {
		name          string
		mutateService func(svc *oscarType.Service)
		wantImage     string
		wantModelName string
	}{
		{
			name:          "default cpu image",
			mutateService: func(svc *oscarType.Service) {},
			wantImage:     defaultLLMCPUimage,
			wantModelName: "my-llm-service",
		},
		{
			name: "default gpu image",
			mutateService: func(svc *oscarType.Service) {
				svc.Kserve.EnableGPU = true
			},
			wantImage:     defaultLLMGPUimage,
			wantModelName: "my-llm-service",
		},
		{
			name: "custom llm runtime and model name",
			mutateService: func(svc *oscarType.Service) {
				svc.Kserve.LLM = &oscarType.LLMConfig{ModelName: "custom-model", RuntimeImage: "repo/custom:v1"}
			},
			wantImage:     "repo/custom:v1",
			wantModelName: "custom-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := llmKserveService()
			tt.mutateService(svc)

			oldLLMIsvc := &unstructured.Unstructured{Object: map[string]any{
				"spec": map[string]any{
					"labels": map[string]any{"preserve": "yes"},
				},
			}}

			updated, err := UpdateKserveLLMInferenceServiceDefinition(svc, oldLLMIsvc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if v := getNestedString(t, updated, "spec", "model", "uri"); v != svc.Kserve.StorageUri {
				t.Errorf("model.uri = %q, want %q", v, svc.Kserve.StorageUri)
			}
			if v := getNestedString(t, updated, "spec", "model", "name"); v != tt.wantModelName {
				t.Errorf("model.name = %q, want %q", v, tt.wantModelName)
			}
			if v := getNestedInt32(t, updated, "spec", "replicas"); v != svc.Kserve.MinScale {
				t.Errorf("replicas = %d, want %d", v, svc.Kserve.MinScale)
			}

			labels := getNestedMap(t, updated, "spec", "labels")
			if labels["preserve"] != "yes" {
				t.Errorf("labels.preserve = %v, want %q", labels["preserve"], "yes")
			}

			containersAny := getRawNested(t, updated, "spec", "template", "containers")
			containers, ok := containersAny.([]any)
			if !ok || len(containers) != 1 {
				t.Fatalf("expected one container, got %T (%v)", containersAny, containersAny)
			}
			container, ok := containers[0].(map[string]any)
			if !ok {
				t.Fatalf("expected container map, got %T", containers[0])
			}

			if v, ok := container["image"].(string); !ok || v != tt.wantImage {
				t.Errorf("container.image = %v, want %q", container["image"], tt.wantImage)
			}
			if v, ok := container["name"].(string); !ok || v != KserveLLMISVCContainerName {
				t.Errorf("container.name = %v, want %q", container["name"], KserveLLMISVCContainerName)
			}
			if container["resources"] == nil {
				t.Error("expected container resources to be set")
			}
		})
	}
}

func TestUpdateKserveLLMInferenceServiceDefinition_InvalidCPU(t *testing.T) {
	svc := llmKserveService()
	svc.Kserve.CPU = "invalid-cpu"
	oldLLMIsvc := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"labels": map[string]any{},
		},
	}}

	_, err := UpdateKserveLLMInferenceServiceDefinition(svc, oldLLMIsvc)
	if err == nil {
		t.Fatal("expected error for invalid CPU quantity, got nil")
	}
}

func TestGetKserveLLMServiceRouterSpec(t *testing.T) {
	tests := []struct {
		name     string
		setAuth  bool
		wantAuth bool
	}{
		{name: "without auth", setAuth: false, wantAuth: false},
		{name: "with auth", setAuth: true, wantAuth: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := llmKserveService()
			svc.Name = "router-service"
			svc.Kserve.SetAuth = tt.setAuth

			routerSpec := getKserveLLMServiceRouterSpec(svc, "router-ns")
			routerObj := &unstructured.Unstructured{Object: routerSpec}

			rulesAny := getRawNested(t, routerObj, "route", "http", "spec", "rules")
			rules, ok := rulesAny.([]any)
			if !ok || len(rules) != 1 {
				t.Fatalf("expected one rule, got %T (%v)", rulesAny, rulesAny)
			}

			rule, ok := rules[0].(map[string]any)
			if !ok {
				t.Fatalf("expected rule map, got %T", rules[0])
			}

			matches, ok := rule["matches"].([]any)
			if !ok || len(matches) != 1 {
				t.Fatalf("expected one match, got %T (%v)", rule["matches"], rule["matches"])
			}
			match, ok := matches[0].(map[string]any)
			if !ok {
				t.Fatalf("expected match map, got %T", matches[0])
			}
			path, ok := match["path"].(map[string]any)
			if !ok {
				t.Fatalf("expected path map, got %T", match["path"])
			}
			if path["value"] != getAPIPath(svc.Name) {
				t.Errorf("path.value = %v, want %q", path["value"], getAPIPath(svc.Name))
			}

			filters, ok := rule["filters"].([]any)
			if !ok {
				t.Fatalf("expected filters list, got %T", rule["filters"])
			}

			hasAuthFilter := false
			hasURLRewriteFilter := false
			for _, rawFilter := range filters {
				filter, ok := rawFilter.(map[string]any)
				if !ok {
					continue
				}
				typeName, _ := filter["type"].(string)
				switch typeName {
				case "ExtensionRef":
					hasAuthFilter = true
				case "URLRewrite":
					hasURLRewriteFilter = true
				}
			}

			if hasAuthFilter != tt.wantAuth {
				t.Errorf("auth filter present = %v, want %v", hasAuthFilter, tt.wantAuth)
			}
			if !hasURLRewriteFilter {
				t.Error("expected URLRewrite filter")
			}
		})
	}
}

func TestKserveGetAPIPath(t *testing.T) {
	if got := getAPIPath("service-a"); got != "/system/services/service-a/exposed" {
		t.Errorf("getAPIPath() = %q, want %q", got, "/system/services/service-a/exposed")
	}
}

func TestKserveFormatUID(t *testing.T) {
	longUID := strings.Repeat("a", 70) + "@example.org"

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "egi uid", input: "abcdef123@egi.eu", expected: "abcdef123"},
		{name: "non matching uppercase", input: "USER@EGI.EU", expected: "USER@EGI.EU"},
		{name: "without at-sign", input: "user-id", expected: "user-id"},
		{name: "truncate long uid", input: longUID, expected: strings.Repeat("a", 62)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatUID(tt.input)
			if got != tt.expected {
				t.Errorf("formatUID(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestKserveInjectRootPath(t *testing.T) {
	pathForService := func(serviceName string) string {
		return fmt.Sprintf("--root-path=%s", getAPIPath(serviceName))
	}

	tests := []struct {
		name  string
		svc   *oscarType.Service
		check func(t *testing.T, svc *oscarType.Service)
	}{
		{
			name: "mlserver env var injected",
			svc: &oscarType.Service{
				Name: "ml-service",
				Kserve: &oscarType.Kserve{
					ModelFormat: "sklearn",
				},
			},
			check: func(t *testing.T, svc *oscarType.Service) {
				t.Helper()
				if svc.Kserve.Env == nil {
					t.Fatal("expected env map to be initialized")
				}
				if got := svc.Kserve.Env["MLSERVER_ROOT_PATH"]; got != getAPIPath(svc.Name) {
					t.Errorf("MLSERVER_ROOT_PATH = %q, want %q", got, getAPIPath(svc.Name))
				}
			},
		},
		{
			name: "vllm argument injected",
			svc: &oscarType.Service{
				Name: "vllm-service",
				Kserve: &oscarType.Kserve{
					ModelFormat: "huggingface",
					Args:        []string{"--port=8080"},
				},
			},
			check: func(t *testing.T, svc *oscarType.Service) {
				t.Helper()
				expectedArg := pathForService(svc.Name)
				found := false
				for _, arg := range svc.Kserve.Args {
					if arg == expectedArg {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected args to contain %q, got %v", expectedArg, svc.Kserve.Args)
				}
			},
		},
		{
			name: "triton unchanged",
			svc: &oscarType.Service{
				Name: "triton-service",
				Kserve: &oscarType.Kserve{
					ModelFormat: "triton",
					Args:        []string{"--strict=true"},
				},
			},
			check: func(t *testing.T, svc *oscarType.Service) {
				t.Helper()
				if len(svc.Kserve.Args) != 1 || svc.Kserve.Args[0] != "--strict=true" {
					t.Errorf("expected args unchanged, got %v", svc.Kserve.Args)
				}
				if svc.Kserve.Env != nil {
					if _, exists := svc.Kserve.Env["MLSERVER_ROOT_PATH"]; exists {
						t.Error("did not expect MLSERVER_ROOT_PATH for triton")
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			injectRootPath(tt.svc)
			tt.check(t, tt.svc)
		})
	}
}
