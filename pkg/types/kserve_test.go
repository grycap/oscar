package types

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

func TestKserveUnmarshalJSON(t *testing.T) {
	// Test default unmarshaling
	data := []byte(`{
	"inference": {
		"model_format": "onnx",
		"source": "s3://bucket/model"
	}
}`)
	var kserve ModelServing
	err := json.Unmarshal(data, &kserve)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if kserve.GetType() != kserveTypeInferenceService {
		t.Errorf("Expected type %q, got %q", kserveTypeInferenceService, kserve.GetType())
	}
	if kserve.Inference().Source != "s3://bucket/model" {
		t.Errorf("Unexpected Source %q", kserve.Inference().Source)
	}
	if kserve.Inference().MinScale != 1 {
		t.Errorf("Expected default MinScale 1, got %d", kserve.Inference().MinScale)
	}
	if kserve.Inference().MaxScale != 1 {
		t.Errorf("Expected default MaxScale 1, got %d", kserve.Inference().MaxScale)
	}
	if kserve.Inference().SetAuth != true {
		t.Errorf("Expected default SetAuth true, got %v", kserve.Inference().SetAuth)
	}

	// Missing Source: unmarshal succeeds, validation fails
	invalidData := []byte(`{"inference": {"model_format": "onnx"}}`)
	var kserveInvalid ModelServing
	err = json.Unmarshal(invalidData, &kserveInvalid)
	if err != nil {
		t.Fatal("Unmarshal should not fail for missing Source, validation is done separately")
	}
	err = kserveInvalid.Validate()
	if err == nil {
		t.Fatalf("Expected validation error for missing Source, got: %v", err)
	}

	// LLM inference
	var llm ModelServing
	if err := json.Unmarshal([]byte(`{"llm_inference": {"source": "hf://org/model"}}`), &llm); err != nil {
		t.Fatalf("Unmarshal LLM failed: %v", err)
	}
	if !llm.IsLLMInferenceService() || llm.IsInferenceService() {
		t.Errorf("Expected an LLM inference service, got type %q", llm.GetType())
	}
	if err := llm.Validate(); err != nil {
		t.Errorf("Expected valid LLM service, got: %v", err)
	}

	// Empty document: no Spec
	var empty ModelServing
	if err := json.Unmarshal([]byte(`{}`), &empty); err != nil {
		t.Fatalf("Unmarshal empty failed: %v", err)
	}
	if empty.Spec != nil {
		t.Errorf("Expected nil Spec, got %v", empty.Spec)
	}
}

func TestKserveValidateInvalidType(t *testing.T) {
	// Test unknown type
	kserve := ModelServing{
		Spec: nil,
	}
	err := kserve.Validate()
	if err == nil {
		t.Fatal("Expected validation error for unknown type, got nil")
	}
	if err != ErrModelServingTypeInvalid {
		t.Errorf("Expected %v, got %v", ErrModelServingTypeInvalid, err)
	}
	expectedErr := "invalid ModelServing service type"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message:\n%s\nGot:\n%s", expectedErr, err.Error())
	}
}

func TestKserveValidateMissingSource(t *testing.T) {
	kserve := ModelServing{
		Spec: &KserveInference{
			ModelFormat: "onnx",
			APIVersion:  "v1",
			Source:      "", // Missing
		},
	}
	err := kserve.Validate()
	if err == nil {
		t.Fatal("Expected validation error for missing Source, got nil")
	}
	expectedErr := "model Source is required"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message:\n%s\nGot:\n%s", expectedErr, err.Error())
	}
}

func TestKserveUnmarshalConflict(t *testing.T) {
	data := []byte(`{"inference": {"model_format": "onnx", "source": "s"}, "llm_inference": {"source": "s"}}`)
	var kserve ModelServing
	err := json.Unmarshal(data, &kserve)
	if err != ErrModelServingTypeConflict {
		t.Fatalf("Expected %v, got %v", ErrModelServingTypeConflict, err)
	}
}

func TestKserveValidateInferenceMissingModelFormat(t *testing.T) {
	kserve := ModelServing{Spec: &KserveInference{Source: "s3://bucket/model", APIVersion: "v1"}}
	err := kserve.Validate()
	if err == nil || err.Error() != "ModelFormat is required" {
		t.Fatalf("Expected ModelFormat error, got %v", err)
	}
}

func TestKserveValidateLLMInference(t *testing.T) {
	if err := (ModelServing{Spec: &KserveLLMInference{Source: "hf://org/model"}}).Validate(); err != nil {
		t.Errorf("Expected valid LLM service, got %v", err)
	}
	err := (ModelServing{Spec: &KserveLLMInference{}}).Validate()
	if err == nil || err.Error() != "model Source is required" {
		t.Fatalf("Expected Source error, got %v", err)
	}
}

func TestKserveValidateEqual(t *testing.T) {
	newInf := func(source string, minScale int32) ModelServing {
		return ModelServing{Spec: &KserveInference{
			ModelFormat: "onnx",
			APIVersion:  "v2",
			Source:      source,
			MinScale:    minScale,
			MaxScale:    2,
			SetAuth:     true,
			Args:        []string{"arg1"},
		}}
	}
	kserve1 := newInf("uri1", 1)
	kserve2 := newInf("uri1", 1)
	kserve3 := newInf("uri2", 1)
	kserve4 := ModelServing{Spec: &KserveLLMInference{RuntimeImage: "vLLM", Source: "uri1"}}
	kserve5 := newInf("uri1", 2)

	t.Run("Equal_True", func(t *testing.T) {
		if !kserve1.Equal(kserve2) {
			t.Error("kserve1 and kserve2 should be equal")
		}
	})

	t.Run("Equal_False_Source", func(t *testing.T) {
		if kserve1.Equal(kserve3) {
			t.Error("kserve1 and kserve3 should NOT be equal due to different Source")
		}
	})

	t.Run("Equal_False_Type", func(t *testing.T) {
		if kserve1.Equal(kserve4) {
			t.Error("kserve1 and kserve4 should NOT be equal due to different Type")
		}
	})

	t.Run("Equal_False_MinScale", func(t *testing.T) {
		if kserve1.Equal(kserve5) {
			t.Error("kserve1 and kserve5 should NOT be equal due to different MinScale")
		}
	})

	t.Run("Equal_NilSpec", func(t *testing.T) {
		if !(ModelServing{}).Equal(ModelServing{}) {
			t.Error("two empty ModelServing should be equal")
		}
		if kserve1.Equal(ModelServing{}) {
			t.Error("a ModelServing with Spec should differ from an empty one")
		}
	})
}
func TestKserveInferenceUnmarshalJSON(t *testing.T) {
	// Test default API Version
	data := []byte(`{"model_format": "onnx"}`)
	var kserveInference KserveInference
	err := json.Unmarshal(data, &kserveInference)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if kserveInference.APIVersion != "v1" {
		t.Errorf("Expected default APIVersion v1, got %s", kserveInference.APIVersion)
	}

	// Test custom API Version
	dataV2 := []byte(`{"model_format": "pytorch", "api_version": "v2"}`)
	var kserveInferenceV2 KserveInference
	err = json.Unmarshal(dataV2, &kserveInferenceV2)
	if err != nil {
		t.Fatalf("Unmarshal V2 failed: %v", err)
	}
	if kserveInferenceV2.APIVersion != "v2" {
		t.Errorf("Expected APIVersion v2, got %s", kserveInferenceV2.APIVersion)
	}

	// An invalid API version is only rejected by validate()
	dataInvalid := []byte(`{"model_format": "onnx", "api_version": "v3", "source": "s"}`)
	var kserveInferenceInvalid KserveInference
	if err = json.Unmarshal(dataInvalid, &kserveInferenceInvalid); err != nil {
		t.Fatalf("Unmarshal should not fail for invalid API version: %v", err)
	}
	if err = kserveInferenceInvalid.validate(); err == nil {
		t.Fatal("Expected validation to fail due to invalid API version, but it succeeded.")
	}
}

func TestKserveInferenceValidate(t *testing.T) {
	// Test missing ModelFormat
	kserve := KserveInference{
		ModelFormat: "",
		APIVersion:  "v1",
	}
	err := kserve.validate()
	if err == nil {
		t.Fatal("Expected validation error for missing ModelFormat, got nil")
	}
	expectedErr := "ModelFormat is required"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message:\n%s\nGot:\n%s", expectedErr, err.Error())
	}

	// Test missing Source
	kserve = KserveInference{
		ModelFormat: "onnx",
		APIVersion:  "v1",
	}
	err = kserve.validate()
	if err == nil || err.Error() != "model Source is required" {
		t.Fatalf("Expected Source error, got %v", err)
	}

	// Test invalid API Version
	kserve = KserveInference{
		ModelFormat: "onnx",
		APIVersion:  "v3",
		Source:      "s3://bucket/model",
	}
	err = kserve.validate()
	if err == nil {
		t.Fatal("Expected validation error for invalid API version, got nil")
	}
	expectedErr = "invalid API version \"v3\", expected v1 or v2"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message:\n%s\nGot:\n%s", expectedErr, err.Error())
	}

	// Test valid API Version
	kserveValid := KserveInference{
		ModelFormat: "triton",
		APIVersion:  "v1",
		Source:      "s3://bucket/model",
	}
	err = kserveValid.validate()
	if err != nil {
		t.Errorf("Expected valid inference config to pass validation, got error: %v", err)
	}
}

func TestKserveLLMInferenceEqual(t *testing.T) {
	llm1 := KserveLLMInference{RuntimeImage: "vLLM"}
	llm2 := KserveLLMInference{RuntimeImage: "vLLM"}
	llm3 := KserveLLMInference{RuntimeImage: "other_image"}

	if !llm1.Equal(llm2) {
		t.Error("llm1 and llm2 should be equal")
	}
	if llm1.Equal(llm3) {
		t.Error("llm1 and llm3 should NOT be equal")
	}
}

func TestKserveEqualInferenceConfig(t *testing.T) {
	inf1 := KserveInference{
		ModelFormat: "onnx",
		APIVersion:  "v1",
	}
	inf2 := KserveInference{
		ModelFormat: "onnx",
		APIVersion:  "v1",
	}
	inf3 := KserveInference{
		ModelFormat: "onnx",
		APIVersion:  "v2", // Different version
	}

	if !inf1.Equal(inf2) {
		t.Error("inf1 and inf2 should be equal")
	}
	if inf1.Equal(inf3) {
		t.Error("inf1 and inf3 should NOT be equal due to different API Version")
	}
}

func nestedString(obj map[string]any, fields ...string) (string, bool, error) {
	return unstructured.NestedString(obj, fields...)
}

func newInferenceService(name string) *Service {
	svc := &Service{
		Name:      name,
		Namespace: "oscar-svc",
		Owner:     "user1",
		Labels:    map[string]string{},
	}
	svc.ModelServing = &ModelServing{Spec: &KserveInference{
		ModelFormat: "onnx",
		APIVersion:  "v1",
		Source:      "s3://bucket/model",
		MinScale:    1,
		MaxScale:    2,
		SetAuth:     true,
	}}
	return svc
}

func newLLMService(name string) *Service {
	svc := &Service{
		Name:      name,
		Namespace: "oscar-svc",
		Owner:     "user1",
		Labels:    map[string]string{},
	}
	svc.ModelServing = &ModelServing{Spec: &KserveLLMInference{
		Source:   "hf://org/model",
		MinScale: 0,
		MaxScale: 1,
		SetAuth:  true,
	}}
	return svc
}

func TestKserveNamingHelpers(t *testing.T) {
	inf := ModelServing{Spec: &KserveInference{}}
	llm := ModelServing{Spec: &KserveLLMInference{}}

	tests := []struct {
		name                                     string
		ms                                       ModelServing
		svcName, podName, container, selectorKey string
	}{
		{"inference", inf, "svc-predictor", "svc-predictor", kserveISVCContainerName, kserveKeyLabelApp},
		{"llm", llm, "svc-kserve-workload-svc", "svc-kserve", kserveLLMISVCContainerName, kserveKeyLabelApp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ms.GetSvcName("svc"); got != tt.svcName {
				t.Errorf("GetSvcName = %q, want %q", got, tt.svcName)
			}
			if got := tt.ms.GetPodAndDplName("svc"); got != tt.podName {
				t.Errorf("GetPodAndDplName = %q, want %q", got, tt.podName)
			}
			if got := tt.ms.GetContainerName(); got != tt.container {
				t.Errorf("GetContainerName = %q, want %q", got, tt.container)
			}
			if got := tt.ms.GetLabelSelectorKey(); got != tt.selectorKey {
				t.Errorf("GetLabelSelectorKey = %q, want %q", got, tt.selectorKey)
			}
			want := "svc." + "ns" + ".svc.cluster.local"
			if got := tt.ms.GetClusterInternalURI("svc", "ns"); !strings.HasSuffix(got, ".ns.svc.cluster.local") || !strings.HasPrefix(got, "svc") {
				t.Errorf("GetClusterInternalURI = %q, want something like %q", got, want)
			}
		})
	}

	if got := inf.GetLabelSelector("svc"); got != "oscar-app=oscar-svc-ksv-svc" {
		t.Errorf("GetLabelSelector = %q", got)
	}
	if got := inf.GetLabelSelectorValue("svc"); got != "oscar-svc-ksv-svc" {
		t.Errorf("GetLabelSelectorValue = %q", got)
	}

	empty := ModelServing{}
	if empty.GetSvcName("svc") != "" || empty.GetPodAndDplName("svc") != "" || empty.GetType() != "" {
		t.Error("ModelServing without Spec should return empty names")
	}
}

func TestBuildKserveNameAndPaths(t *testing.T) {
	if got := buildKserveName("svc"); got != "svc" {
		t.Errorf("buildKserveName = %q", got)
	}
	if got := getAPIPath("svc"); got != "/system/services/svc/models" {
		t.Errorf("getAPIPath = %q", got)
	}
	if got := getTraefikAuthMiddlewareName("svc"); got != "svc-auth-mdw" {
		t.Errorf("getTraefikAuthMiddlewareName = %q", got)
	}
	if got := getServiceAuthCookieName("my-svc"); got != "oscar_service_my_svc_auth" {
		t.Errorf("getServiceAuthCookieName = %q", got)
	}
}

func TestGetOwnerReference(t *testing.T) {
	if got := getOwnerReference(nil); got != nil {
		t.Errorf("expected nil for nil owner, got %v", got)
	}
	owner := &KserveServiceOwner{APIVersion: "v1", Kind: "Service", Name: "svc", UID: "uid-1"}
	refs := getOwnerReference(owner)
	if len(refs) != 1 {
		t.Fatalf("expected 1 owner reference, got %d", len(refs))
	}
	ref := refs[0]
	if ref.Name != "svc" || ref.Kind != "Service" || ref.APIVersion != "v1" || ref.UID != "uid-1" {
		t.Errorf("unexpected owner reference %+v", ref)
	}
	if ref.Controller == nil || *ref.Controller || ref.BlockOwnerDeletion == nil || !*ref.BlockOwnerDeletion {
		t.Errorf("unexpected controller/blockOwnerDeletion flags: %+v", ref)
	}
}

func TestNormalizeScaleFromKserveService(t *testing.T) {
	tests := []struct {
		name     string
		ms       *ModelServing
		min, max int32
	}{
		{"inference zero values", &ModelServing{Spec: &KserveInference{}}, 0, 1},
		{"inference explicit", &ModelServing{Spec: &KserveInference{MinScale: 2, MaxScale: 4}}, 2, 4},
		{"inference min above max", &ModelServing{Spec: &KserveInference{MinScale: 5, MaxScale: 2}}, 5, 5},
		{"llm zero min becomes 1", &ModelServing{Spec: &KserveLLMInference{}}, 1, 1},
		{"llm explicit", &ModelServing{Spec: &KserveLLMInference{MinScale: 3, MaxScale: 3}}, 3, 3},
		{"no spec", &ModelServing{}, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			min, max := normalizeScaleFromKserveService(tt.ms)
			if min != tt.min || max != tt.max {
				t.Errorf("got (%d, %d), want (%d, %d)", min, max, tt.min, tt.max)
			}
		})
	}
}

func TestCreateKserveResources(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		res, err := createKserveResources(&Service{})
		if err != nil {
			t.Fatal(err)
		}
		if got := res.Requests.Cpu().String(); got != "200m" {
			t.Errorf("cpu = %s, want 200m", got)
		}
		if got := res.Requests.Memory().String(); got != "256Mi" {
			t.Errorf("memory = %s, want 256Mi", got)
		}
		if _, ok := res.Requests["nvidia.com/gpu"]; ok {
			t.Error("unexpected GPU request")
		}
	})
	t.Run("custom with overhead and gpu", func(t *testing.T) {
		res, err := createKserveResources(&Service{CPU: "1", Memory: "2Gi", EnableGPU: true})
		if err != nil {
			t.Fatal(err)
		}
		if got := res.Limits.Cpu().String(); got != "990m" {
			t.Errorf("cpu limit = %s, want 990m", got)
		}
		if got := res.Limits.Memory().String(); got != "2033Mi" {
			t.Errorf("memory limit = %s, want 2033Mi", got)
		}
		gpu := res.Requests["nvidia.com/gpu"]
		if gpu.String() != "1" {
			t.Errorf("gpu = %s, want 1", gpu.String())
		}
	})
	t.Run("invalid values", func(t *testing.T) {
		if _, err := createKserveResources(&Service{CPU: "abc"}); err == nil {
			t.Error("expected error for invalid CPU")
		}
		if _, err := createKserveResources(&Service{Memory: "abc"}); err == nil {
			t.Error("expected error for invalid memory")
		}
	})
}

func TestNewKserveInferenceServiceSpec(t *testing.T) {
	svc := newInferenceService("svc")
	svc.ModelServing.Inference().RuntimeImage = "custom-runtime"
	svc.ModelServing.Inference().Args = []string{"--a"}
	cfg := &Config{}
	owner := &KserveServiceOwner{Namespace: "owner-ns", Name: "svc", Kind: "Service", APIVersion: "v1"}

	u, err := newKserveInferenceServiceSpec(svc, owner, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if u.GetKind() != "InferenceService" || u.GetName() != "svc" || u.GetNamespace() != "owner-ns" {
		t.Errorf("unexpected metadata: %s %s %s", u.GetKind(), u.GetName(), u.GetNamespace())
	}
	if u.GetLabels()[kserveKeyLabelApp] != prefixLabelApp+"svc" {
		t.Errorf("missing selector label, got %v", u.GetLabels())
	}
	if u.GetLabels()[OscarUserServiceLabel] != "true" {
		t.Errorf("missing oscar user service label, got %v", u.GetLabels())
	}
	storageURI, _, _ := nestedString(u.Object, "spec", "predictor", "model", "storageUri")
	if storageURI != "s3://bucket/model" {
		t.Errorf("storageUri = %q", storageURI)
	}
	runtime, _, _ := nestedString(u.Object, "spec", "predictor", "model", "runtime")
	if runtime != "custom-runtime" {
		t.Errorf("runtime = %q", runtime)
	}
	predictor := u.Object["spec"].(map[string]any)["predictor"].(map[string]any)
	if predictor["minReplicas"] != int32(1) || predictor["maxReplicas"] != int32(2) {
		t.Errorf("unexpected replicas %v/%v", predictor["minReplicas"], predictor["maxReplicas"])
	}

	t.Run("without owner uses service namespace", func(t *testing.T) {
		u, err := newKserveInferenceServiceSpec(svc, nil, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if u.GetNamespace() != "oscar-svc" {
			t.Errorf("namespace = %q", u.GetNamespace())
		}
	})

	t.Run("kueue requires queue label", func(t *testing.T) {
		kcfg := &Config{KueueEnable: true}
		if _, err := newKserveInferenceServiceSpec(svc, owner, kcfg); err == nil {
			t.Error("expected error when the kueue queue label is missing")
		}
		svc.Labels["kueue.x-k8s.io/queue-name"] = "queue"
		u, err := newKserveInferenceServiceSpec(svc, owner, kcfg)
		if err != nil {
			t.Fatal(err)
		}
		if u.GetLabels()["kueue.x-k8s.io/queue-name"] != "queue" {
			t.Errorf("missing queue label, got %v", u.GetLabels())
		}
		delete(svc.Labels, "kueue.x-k8s.io/queue-name")
	})
}

func TestUpdateKserveInferenceServiceSpec(t *testing.T) {
	svc := newInferenceService("svc")
	cfg := &Config{}
	old, err := newKserveInferenceServiceSpec(svc, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	old.SetResourceVersion("42")
	old.Object["spec"].(map[string]any)["predictor"].(map[string]any)["labels"] = map[string]any{"keep": "me"}

	svc.ModelServing.Inference().MaxScale = 5
	svc.ModelServing.Inference().Args = []string{"--new"}
	updated, err := updateKserveInferenceServiceSpec(svc, old)
	if err != nil {
		t.Fatal(err)
	}
	if updated.GetResourceVersion() != "42" {
		t.Errorf("resourceVersion not preserved: %q", updated.GetResourceVersion())
	}
	predictor := updated.Object["spec"].(map[string]any)["predictor"].(map[string]any)
	if predictor["maxReplicas"] != int32(5) {
		t.Errorf("maxReplicas = %v", predictor["maxReplicas"])
	}
	if labels, _ := predictor["labels"].(map[string]any); labels["keep"] != "me" {
		t.Errorf("predictor labels not preserved: %v", predictor["labels"])
	}
}

func TestNewKserveLLMInferenceServiceSpec(t *testing.T) {
	cfg := &Config{}
	owner := &KserveServiceOwner{Namespace: "owner-ns", Name: "svc"}

	imageOf := func(t *testing.T, svc *Service) string {
		t.Helper()
		u, err := newKserveLLMInferenceServiceSpec(svc, owner, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if u.GetKind() != "LLMInferenceService" {
			t.Fatalf("kind = %q", u.GetKind())
		}
		containers := u.Object["spec"].(map[string]any)["template"].(map[string]any)["containers"].([]any)
		return containers[0].(map[string]any)["image"].(string)
	}

	svc := newLLMService("svc")
	if got := imageOf(t, svc); got != defaultLLMCPUimage {
		t.Errorf("image = %q, want %q", got, defaultLLMCPUimage)
	}
	svc.EnableGPU = true
	if got := imageOf(t, svc); got != defaultLLMGPUimage {
		t.Errorf("image = %q, want %q", got, defaultLLMGPUimage)
	}
	svc.ModelServing.LLMInference().RuntimeImage = "custom:latest"
	if got := imageOf(t, svc); got != "custom:latest" {
		t.Errorf("image = %q, want custom:latest", got)
	}

	u, err := newKserveLLMInferenceServiceSpec(svc, owner, cfg)
	if err != nil {
		t.Fatal(err)
	}
	spec := u.Object["spec"].(map[string]any)
	if spec["replicas"] != int32(1) {
		t.Errorf("replicas = %v, want 1 (LLM min scale is at least 1)", spec["replicas"])
	}
	if uri, _, _ := nestedString(spec, "model", "uri"); uri != "hf://org/model" {
		t.Errorf("model uri = %q", uri)
	}

	t.Run("kueue requires queue label", func(t *testing.T) {
		if _, err := newKserveLLMInferenceServiceSpec(svc, owner, &Config{KueueEnable: true}); err == nil {
			t.Error("expected error when the kueue queue label is missing")
		}
	})
}

func TestUpdateKserveLLMInferenceServiceSpec(t *testing.T) {
	svc := newLLMService("svc")
	old, err := newKserveLLMInferenceServiceSpec(svc, nil, &Config{})
	if err != nil {
		t.Fatal(err)
	}
	old.SetResourceVersion("7")

	svc.ModelServing.LLMInference().MinScale = 3
	svc.ModelServing.LLMInference().MaxScale = 3
	updated, err := updateKserveLLMInferenceServiceSpec(svc, old)
	if err != nil {
		t.Fatal(err)
	}
	if updated.GetResourceVersion() != "7" {
		t.Errorf("resourceVersion not preserved: %q", updated.GetResourceVersion())
	}
	spec := updated.Object["spec"].(map[string]any)
	if spec["replicas"] != int32(3) {
		t.Errorf("replicas = %v, want 3", spec["replicas"])
	}
	labels, _ := spec["labels"].(map[string]any)
	if labels[kserveKeyLabelApp] != prefixLabelApp+"svc" {
		t.Errorf("labels not preserved: %v", labels)
	}
}

func TestBuildHTTPRouteSpec(t *testing.T) {
	owner := &KserveServiceOwner{Namespace: "ns", Name: "svc", Kind: "InferenceService", APIVersion: "v1"}
	cfg := &Config{
		IngressHost:               " example.org ",
		HTTPRouteGatewayName:      "gw",
		HTTPRouteGatewayNamespace: "gw-ns",
	}

	t.Run("path based inference with auth", func(t *testing.T) {
		route := buildHTTPRouteSpec(newInferenceService("svc"), owner, cfg)
		if route.GetName() != "svc"+httpRouteSuffix || route.GetNamespace() != "ns" {
			t.Errorf("unexpected metadata %s/%s", route.GetNamespace(), route.GetName())
		}
		spec := route.Object["spec"].(map[string]any)
		if hosts := spec["hostnames"].([]any); len(hosts) != 1 || hosts[0] != "example.org" {
			t.Errorf("hostnames = %v", hosts)
		}
		rule := spec["rules"].([]any)[0].(map[string]any)
		filters := rule["filters"].([]any)
		types := []string{}
		for _, f := range filters {
			types = append(types, f.(map[string]any)["type"].(string))
		}
		if strings.Join(types, ",") != "RequestHeaderModifier,ExtensionRef,URLRewrite" {
			t.Errorf("filters = %v", types)
		}
		backend := rule["backendRefs"].([]any)[0].(map[string]any)
		if backend["name"] != "svc-predictor" || backend["port"] != int64(80) {
			t.Errorf("backend = %v", backend)
		}
		path := rule["matches"].([]any)[0].(map[string]any)["path"].(map[string]any)["value"]
		if path != "/system/services/svc/models" {
			t.Errorf("path = %v", path)
		}
		parent := spec["parentRefs"].([]any)[0].(map[string]any)
		if parent["name"] != "gw" || parent["namespace"] != "gw-ns" {
			t.Errorf("parentRef = %v", parent)
		}
	})

	t.Run("subdomain llm without auth", func(t *testing.T) {
		svc := newLLMService("svc")
		svc.ModelServing.LLMInference().SetAuth = false
		scfg := *cfg
		scfg.ExposedServicesUseSubdomainRoute = true
		route := buildHTTPRouteSpec(svc, owner, &scfg)
		spec := route.Object["spec"].(map[string]any)
		if hosts := spec["hostnames"].([]any); hosts[0] != "svc.example.org" {
			t.Errorf("hostnames = %v", hosts)
		}
		rule := spec["rules"].([]any)[0].(map[string]any)
		if n := len(rule["filters"].([]any)); n != 1 {
			t.Errorf("expected only the header filter, got %d filters", n)
		}
		backend := rule["backendRefs"].([]any)[0].(map[string]any)
		if backend["name"] != "svc-kserve-workload-svc" || backend["port"] != int64(8000) {
			t.Errorf("backend = %v", backend)
		}
		if path := rule["matches"].([]any)[0].(map[string]any)["path"].(map[string]any)["value"]; path != "/" {
			t.Errorf("path = %v", path)
		}
	})
}

func TestBuildTraefikOIDCMiddlewareSpec(t *testing.T) {
	owner := &KserveServiceOwner{Namespace: "ns", Name: "svc"}
	cfg := &Config{Name: "oscar", Namespace: "oscar-ns", ServicePort: 8080}
	mw := buildTraefikOIDCMiddlewareSpec(newInferenceService("svc"), owner, cfg)

	if mw.GetKind() != "Middleware" || mw.GetName() != "svc-auth-mdw" || mw.GetNamespace() != "ns" {
		t.Errorf("unexpected metadata %s %s/%s", mw.GetKind(), mw.GetNamespace(), mw.GetName())
	}
	addr, _, _ := nestedString(mw.Object, "spec", "forwardAuth", "address")
	if addr != "http://oscar.oscar-ns.svc.cluster.local:8080/system/services/svc/auth" {
		t.Errorf("address = %q", addr)
	}
}

func TestGetHTTPRouteBackendPort(t *testing.T) {
	if got := getHTTPRouteBackendPort(newInferenceService("svc")); got != 80 {
		t.Errorf("inference port = %d, want 80", got)
	}
	if got := getHTTPRouteBackendPort(newLLMService("svc")); got != 8000 {
		t.Errorf("llm port = %d, want 8000", got)
	}
}

func TestExposeKserveServiceMissingGateway(t *testing.T) {
	err := exposeKserveService(nil, newInferenceService("svc"), &KserveServiceOwner{}, &Config{})
	if err == nil || !strings.Contains(err.Error(), "gateway namespace and name must be provided") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestModelServingValidateUpdate(t *testing.T) {
	newInf := func(format, runtime string) ModelServing {
		return ModelServing{Spec: &KserveInference{ModelFormat: format, RuntimeImage: runtime, APIVersion: "v1", Source: "s"}}
	}
	newLLM := func(runtime string) ModelServing {
		return ModelServing{Spec: &KserveLLMInference{RuntimeImage: runtime, Source: "s"}}
	}

	tests := []struct {
		name    string
		new     ModelServing
		old     ModelServing
		wantErr string
	}{
		{"inference unchanged", newInf("onnx", "rt"), newInf("onnx", "rt"), ""},
		{"inference model format change", newInf("pytorch", "rt"), newInf("onnx", "rt"), "cannot update model format"},
		{"inference runtime change", newInf("onnx", "other"), newInf("onnx", "rt"), "cannot update runtime"},
		{"llm unchanged", newLLM("rt"), newLLM("rt"), ""},
		{"llm runtime change", newLLM("other"), newLLM("rt"), "cannot update runtime"},
		{"type change", newLLM("rt"), newInf("onnx", "rt"), "cannot change ModelServing service type"},
		{"new definition invalid", ModelServing{Spec: &KserveInference{ModelFormat: "onnx", APIVersion: "v1"}}, newInf("onnx", ""), "invalid ModelServing service"},
		{"nil spec on both sides", ModelServing{}, ModelServing{}, ErrModelServingTypeInvalid.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.new.ValidateUpdate(&tt.old)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestModelServingGetScalingAndAuth(t *testing.T) {
	inf := ModelServing{Spec: &KserveInference{MinScale: 1, MaxScale: 3, SetAuth: true}}
	scaling, ok := inf.GetScaling()
	if !ok || scaling.MinScale != 1 || scaling.MaxScale != 3 {
		t.Errorf("GetScaling = %+v, %v", scaling, ok)
	}
	if auth, ok := inf.GetAuth(); !ok || !auth {
		t.Errorf("GetAuth = %v, %v", auth, ok)
	}
	llm := ModelServing{Spec: &KserveLLMInference{MinScale: 2, MaxScale: 2}}
	if scaling, ok := llm.GetScaling(); !ok || scaling.MinScale != 2 {
		t.Errorf("LLM GetScaling = %+v, %v", scaling, ok)
	}
	if auth, ok := llm.GetAuth(); !ok || auth {
		t.Errorf("LLM GetAuth = %v, %v", auth, ok)
	}
	empty := ModelServing{}
	if _, ok := empty.GetScaling(); ok {
		t.Error("GetScaling should report false without Spec")
	}
	if _, ok := empty.GetAuth(); ok {
		t.Error("GetAuth should report false without Spec")
	}
}

func TestModelServingGetKueueResourceList(t *testing.T) {
	svc := newInferenceService("svc")
	svc.CPU = "500m"
	svc.Memory = "1Gi"
	svc.ModelServing.Inference().MinScale = 3

	req, minScale, err := svc.ModelServing.GetKueueResourceList(svc)
	if err != nil {
		t.Fatal(err)
	}
	if minScale != 3 || req.Cpu().String() != "490m" || req.Memory().String() != "1009Mi" {
		t.Errorf("got %s/%s min=%d", req.Cpu(), req.Memory(), minScale)
	}

	llm := newLLMService("llm")
	if _, minScale, err := llm.ModelServing.GetKueueResourceList(llm); err != nil || minScale != 1 {
		t.Errorf("LLM kueue list: min=%d err=%v", minScale, err)
	}

	bad := newInferenceService("bad")
	bad.CPU = "abc"
	if _, _, err := bad.ModelServing.GetKueueResourceList(bad); err == nil || !strings.Contains(err.Error(), "failed to create Kserve resources for Kueue") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestModelServingDynamicClientErrors(t *testing.T) {
	orig := newDynamicClient
	t.Cleanup(func() { newDynamicClient = orig })
	clientErr := errors.New("no cluster")
	newDynamicClient = func() (*dynamic.DynamicClient, error) { return nil, clientErr }

	// The ModelServing wrappers are not exercised here because their ownership check
	// compares the address of the value receiver copy; the specs are tested directly.
	for name, svc := range map[string]*Service{"inference": newInferenceService("svc"), "llm": newLLMService("svc")} {
		t.Run(name, func(t *testing.T) {
			spec := svc.ModelServing.Spec
			if err := spec.createModelServingService(svc, nil, &Config{}); err == nil || !strings.Contains(err.Error(), "failed to create dynamic client") {
				t.Errorf("create: unexpected error %v", err)
			}
			if err := spec.updateModelServingService(svc, svc, "ns"); err == nil || !strings.Contains(err.Error(), "failed to create dynamic client") {
				t.Errorf("update: unexpected error %v", err)
			}
			if err := spec.deleteModelServingService(svc, "ns"); err == nil {
				t.Error("delete: expected error")
			}
		})
	}

	t.Run("nil spec", func(t *testing.T) {
		svc := &Service{Name: "svc", ModelServing: &ModelServing{}}
		if err := svc.ModelServing.CreateModelServingService(svc, nil, &Config{}); err == nil || !strings.Contains(err.Error(), "model serving backend is nil") {
			t.Errorf("create: unexpected error: %v", err)
		}
		if err := svc.ModelServing.UpdateModelServingService(svc, svc, "ns"); err == nil || !strings.Contains(err.Error(), "model serving backend is nil") {
			t.Errorf("update: unexpected error: %v", err)
		}
		if err := svc.ModelServing.DeleteModelServingService(svc, "ns"); err == nil || !strings.Contains(err.Error(), "model serving backend is nil") {
			t.Errorf("delete: unexpected error: %v", err)
		}
	})
}

func TestKserveCreateNilConfigPanics(t *testing.T) {
	for name, svc := range map[string]*Service{"inference": newInferenceService("svc"), "llm": newLLMService("svc")} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("expected panic for nil config")
				}
			}()
			_ = svc.ModelServing.Spec.createModelServingService(svc, nil, nil)
		})
	}
}

func TestModelServingSerializationRoundTrip(t *testing.T) {
	for name, ms := range map[string]ModelServing{
		"inference": {Spec: &KserveInference{ModelFormat: "onnx", APIVersion: "v2", Source: "s", MinScale: 1, MaxScale: 2, SetAuth: true}},
		"llm":       {Spec: &KserveLLMInference{RuntimeImage: "img", Source: "s", MinScale: 1, MaxScale: 2, SetAuth: true}},
	} {
		t.Run(name+" json", func(t *testing.T) {
			data, err := json.Marshal(ms)
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			if _, ok := raw[ms.GetType()]; !ok || len(raw) != 1 {
				t.Errorf("expected only the %q key, got %s", ms.GetType(), data)
			}
			var back ModelServing
			if err := json.Unmarshal(data, &back); err != nil {
				t.Fatal(err)
			}
			if !ms.Equal(back) {
				t.Errorf("round trip mismatch: %s", data)
			}
		})
		t.Run(name+" yaml", func(t *testing.T) {
			data, err := yaml.Marshal(ms)
			if err != nil {
				t.Fatal(err)
			}
			var back ModelServing
			if err := yaml.Unmarshal(data, &back); err != nil {
				t.Fatal(err)
			}
			if !ms.Equal(back) {
				t.Errorf("round trip mismatch: %s", data)
			}
		})
	}

	t.Run("marshal without spec", func(t *testing.T) {
		data, err := json.Marshal(ModelServing{})
		if err != nil || string(data) != "{}" {
			t.Errorf("got %s, %v", data, err)
		}
	})
}

func TestRegisterModelServingSpec(t *testing.T) {
	registered := registeredModelServingTypes()
	want := map[string]bool{kserveTypeInferenceService: true, kserveTypeLLMInferenceService: true}
	for _, name := range registered {
		delete(want, name)
	}
	if len(want) != 0 {
		t.Errorf("types not registered: %v (registered: %v)", want, registered)
	}
}
