package types

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const fakeModelServingType = "fake_serving"

// fakeServingSpec is a minimal ModelServingSpec used to test ModelServing
// independently of the KServe implementations. Unimplemented methods panic
// through the embedded nil interface.
type fakeServingSpec struct {
	ModelServingSpec `json:"-"`
	Value            string `json:"value"`
	validateErr      error
	updateErr        error
}

func (f *fakeServingSpec) Type() string                  { return fakeModelServingType }
func (f *fakeServingSpec) validate() error               { return f.validateErr }
func (f *fakeServingSpec) svcName(s string) string       { return s + "-fake-svc" }
func (f *fakeServingSpec) podAndDplName(s string) string { return s + "-fake-pod" }
func (f *fakeServingSpec) labelSelector(s string) string { return "fake=" + s }
func (f *fakeServingSpec) getLabelSelectorKey() string   { return "fake" }
func (f *fakeServingSpec) getConteinerName() string      { return "fake-container" }
func (f *fakeServingSpec) getLabelSelectorValue(s string) string {
	return s
}
func (f *fakeServingSpec) getClusterInternalURI(s, ns string) string {
	return s + "." + ns + ".fake"
}
func (f *fakeServingSpec) validateUpdate(old ModelServingSpec) error { return f.updateErr }

func registerFakeServing(t *testing.T, decode modelServingSpecDecoder) {
	t.Helper()
	RegisterModelServingSpec(fakeModelServingType, decode)
	t.Cleanup(func() { delete(modelServingSpecDecoders, fakeModelServingType) })
}

func decodeFake(raw json.RawMessage) (ModelServingSpec, error) {
	spec := &fakeServingSpec{}
	if err := json.Unmarshal(raw, spec); err != nil {
		return nil, err
	}
	return spec, nil
}

func TestRegisterModelServingSpecDuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic when registering an already registered type")
		}
	}()
	RegisterModelServingSpec(kserveTypeInferenceService, decodeFake)
}

func TestRegisteredModelServingTypesSorted(t *testing.T) {
	registerFakeServing(t, decodeFake)
	names := registeredModelServingTypes()
	if !sort.StringsAreSorted(names) {
		t.Errorf("types not sorted: %v", names)
	}
	found := false
	for _, n := range names {
		found = found || n == fakeModelServingType
	}
	if !found {
		t.Errorf("fake type missing from %v", names)
	}
}

func TestModelServingUnmarshalJSONRegistry(t *testing.T) {
	registerFakeServing(t, decodeFake)

	t.Run("custom registered type", func(t *testing.T) {
		var ms ModelServing
		if err := json.Unmarshal([]byte(`{"fake_serving":{"value":"x"}}`), &ms); err != nil {
			t.Fatal(err)
		}
		spec, ok := ms.Spec.(*fakeServingSpec)
		if !ok || spec.Value != "x" {
			t.Fatalf("unexpected spec %#v", ms.Spec)
		}
		if ms.GetType() != fakeModelServingType || ms.GetModelServingType() != fakeModelServingType {
			t.Errorf("unexpected type %q", ms.GetType())
		}
		if ms.IsInferenceService() || ms.IsLLMInferenceService() {
			t.Error("a custom type must not be reported as KServe")
		}
	})

	t.Run("conflict between two types", func(t *testing.T) {
		var ms ModelServing
		err := json.Unmarshal([]byte(`{"fake_serving":{},"inference":{"source":"s"}}`), &ms)
		if !errors.Is(err, ErrModelServingTypeConflict) {
			t.Errorf("got %v, want %v", err, ErrModelServingTypeConflict)
		}
	})

	t.Run("null body is ignored", func(t *testing.T) {
		var ms ModelServing
		if err := json.Unmarshal([]byte(`{"fake_serving":null,"inference":{"source":"s"}}`), &ms); err != nil {
			t.Fatal(err)
		}
		if ms.GetType() != kserveTypeInferenceService {
			t.Errorf("type = %q", ms.GetType())
		}
	})

	t.Run("unknown keys are ignored", func(t *testing.T) {
		var ms ModelServing
		if err := json.Unmarshal([]byte(`{"other":{"a":1}}`), &ms); err != nil || ms.Spec != nil {
			t.Errorf("spec=%v err=%v", ms.Spec, err)
		}
	})

	t.Run("decoder error is propagated", func(t *testing.T) {
		var ms ModelServing
		if err := json.Unmarshal([]byte(`{"fake_serving":"not an object"}`), &ms); err == nil {
			t.Error("expected decoding error")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		var ms ModelServing
		if err := ms.UnmarshalJSON([]byte(`[1,2]`)); err == nil {
			t.Error("expected error for non-object JSON")
		}
	})
}

func TestModelServingMarshalJSONCustomType(t *testing.T) {
	ms := ModelServing{Spec: &fakeServingSpec{Value: "x"}}
	data, err := json.Marshal(ms)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"fake_serving":{"value":"x"}}` {
		t.Errorf("got %s", data)
	}
}

func TestModelServingYAMLErrors(t *testing.T) {
	var ms ModelServing
	err := ms.UnmarshalYAML(func(v interface{}) error { return errors.New("boom") })
	if err == nil || err.Error() != "boom" {
		t.Errorf("got %v", err)
	}
	err = ms.UnmarshalYAML(func(v interface{}) error {
		*(v.(*map[string]interface{})) = map[string]interface{}{"inference": map[string]interface{}{"source": "s", "model_format": "onnx"}}
		return nil
	})
	if err != nil || ms.Inference() == nil || ms.Inference().Source != "s" {
		t.Errorf("err=%v spec=%v", err, ms.Spec)
	}
}

func TestModelServingDelegationToSpec(t *testing.T) {
	ms := ModelServing{Spec: &fakeServingSpec{}}
	if got := ms.GetSvcName("a"); got != "a-fake-svc" {
		t.Errorf("GetSvcName = %q", got)
	}
	if got := ms.GetPodAndDplName("a"); got != "a-fake-pod" {
		t.Errorf("GetPodAndDplName = %q", got)
	}
	if got := ms.GetLabelSelector("a"); got != "fake=a" {
		t.Errorf("GetLabelSelector = %q", got)
	}
	if got := ms.GetLabelSelectorKey(); got != "fake" {
		t.Errorf("GetLabelSelectorKey = %q", got)
	}
	if got := ms.GetLabelSelectorValue("a"); got != "a" {
		t.Errorf("GetLabelSelectorValue = %q", got)
	}
	if got := ms.GetClusterInternalURI("a", "ns"); got != "a.ns.fake" {
		t.Errorf("GetClusterInternalURI = %q", got)
	}
	if got := ms.GetContainerName(); got != "fake-container" {
		t.Errorf("GetContainerName = %q", got)
	}
}

func TestModelServingNilSpecAccessors(t *testing.T) {
	ms := ModelServing{}
	strs := map[string]string{
		"GetSvcName":            ms.GetSvcName("a"),
		"GetPodAndDplName":      ms.GetPodAndDplName("a"),
		"GetLabelSelector":      ms.GetLabelSelector("a"),
		"GetLabelSelectorKey":   ms.GetLabelSelectorKey(),
		"GetLabelSelectorValue": ms.GetLabelSelectorValue("a"),
		"GetClusterInternalURI": ms.GetClusterInternalURI("a", "ns"),
		"GetContainerName":      ms.GetContainerName(),
	}
	for name, got := range strs {
		if got != "" {
			t.Errorf("%s = %q, want empty", name, got)
		}
	}
	if ms.Inference() != nil || ms.LLMInference() != nil {
		t.Error("typed accessors must return nil without Spec")
	}
	if _, _, err := ms.GetKueueResourceList(&Service{}); err == nil || !strings.Contains(err.Error(), "model serving backend is nil") {
		t.Errorf("GetKueueResourceList error = %v", err)
	}
}

func TestModelServingValidateWithCustomSpec(t *testing.T) {
	want := errors.New("invalid spec")
	if err := (ModelServing{Spec: &fakeServingSpec{validateErr: want}}).Validate(); err != want {
		t.Errorf("got %v, want %v", err, want)
	}
	if err := (ModelServing{Spec: &fakeServingSpec{}}).Validate(); err != nil {
		t.Errorf("unexpected error %v", err)
	}
}

func TestModelServingValidateUpdateWithCustomSpec(t *testing.T) {
	updErr := errors.New("not allowed")
	old := &ModelServing{Spec: &fakeServingSpec{}}

	if err := (ModelServing{Spec: &fakeServingSpec{}}).ValidateUpdate(old); err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if err := (ModelServing{Spec: &fakeServingSpec{updateErr: updErr}}).ValidateUpdate(old); err != updErr {
		t.Errorf("got %v, want %v", err, updErr)
	}
	err := (ModelServing{Spec: &fakeServingSpec{validateErr: errors.New("bad")}}).ValidateUpdate(old)
	if err == nil || !strings.Contains(err.Error(), "invalid ModelServing service: bad") {
		t.Errorf("got %v", err)
	}
	err = (ModelServing{Spec: &fakeServingSpec{}}).ValidateUpdate(&ModelServing{Spec: &KserveInference{}})
	if err == nil || !strings.Contains(err.Error(), `cannot change ModelServing service type from "inference" to "fake_serving"`) {
		t.Errorf("got %v", err)
	}
}

func TestModelServingGetScalingAndAuthUnsupportedSpec(t *testing.T) {
	ms := ModelServing{Spec: &fakeServingSpec{}}
	if _, ok := ms.GetScaling(); ok {
		t.Error("spec without scaling support must report false")
	}
	if _, ok := ms.GetAuth(); ok {
		t.Error("spec without auth support must report false")
	}
}

func TestModelServingEqualDeepComparison(t *testing.T) {
	a := ModelServing{Spec: &fakeServingSpec{Value: "x"}}
	if !a.Equal(ModelServing{Spec: &fakeServingSpec{Value: "x"}}) {
		t.Error("same values should be equal")
	}
	if a.Equal(ModelServing{Spec: &fakeServingSpec{Value: "y"}}) {
		t.Error("different values should differ")
	}
}

func TestModelServingNeedUpdate(t *testing.T) {
	newInferenceService := func() *Service {
		service := &Service{
			CPU:    "1",
			Memory: "1Gi",
			ModelServing: &ModelServing{Spec: &KserveInference{
				ModelFormat: "sklearn",
				RuntimeImage: "runtime:v1",
				APIVersion:   "v1",
				Source:       "s3://models/inference",
				MinScale:     1,
				MaxScale:     2,
				Args:         []string{"--workers=1"},
				SetAuth:      true,
			}},
			Labels: map[string]string{"kueue.x-k8s.io/queue-name": "queue-a"},
		}
		service.Environment.Vars = map[string]string{"MODEL_MODE": "test"}
		service.Environment.Secrets = map[string]string{"TOKEN": "secret"}
		return service
	}
	newLLMService := func() *Service {
		service := &Service{
			CPU:    "1",
			Memory: "1Gi",
			ModelServing: &ModelServing{Spec: &KserveLLMInference{
				RuntimeImage: "runtime:v1",
				Source:       "s3://models/llm",
				MinScale:     1,
				MaxScale:     2,
				Args:         []string{"--workers=1"},
				SetAuth:      true,
			}},
		}
		service.Environment.Vars = map[string]string{"MODEL_MODE": "test"}
		return service
	}

	tests := []struct {
		name       string
		newService func() *Service
		mutate     func(*Service)
		want       bool
	}{
		{name: "unchanged inference service", newService: newInferenceService},
		{name: "CPU change", newService: newInferenceService, mutate: func(s *Service) { s.CPU = "2" }, want: true},
		{name: "memory change", newService: newInferenceService, mutate: func(s *Service) { s.Memory = "2Gi" }, want: true},
		{name: "GPU change", newService: newInferenceService, mutate: func(s *Service) { s.EnableGPU = true }, want: true},
		{name: "environment variable change", newService: newInferenceService, mutate: func(s *Service) { s.Environment.Vars["MODEL_MODE"] = "production" }, want: true},
		{name: "inference model format change", newService: newInferenceService, mutate: func(s *Service) { s.ModelServing.Inference().ModelFormat = "onnx" }, want: true},
		{name: "inference runtime change", newService: newInferenceService, mutate: func(s *Service) { s.ModelServing.Inference().RuntimeImage = "runtime:v2" }, want: true},
		{name: "inference API version change", newService: newInferenceService, mutate: func(s *Service) { s.ModelServing.Inference().APIVersion = "v2" }, want: true},
		{name: "inference source change", newService: newInferenceService, mutate: func(s *Service) { s.ModelServing.Inference().Source = "s3://models/updated" }, want: true},
		{name: "inference minimum scale change", newService: newInferenceService, mutate: func(s *Service) { s.ModelServing.Inference().MinScale = 2 }, want: true},
		{name: "inference maximum scale change", newService: newInferenceService, mutate: func(s *Service) { s.ModelServing.Inference().MaxScale = 3 }, want: true},
		{name: "inference arguments change", newService: newInferenceService, mutate: func(s *Service) { s.ModelServing.Inference().Args[0] = "--workers=2" }, want: true},
		{name: "inference authentication change", newService: newInferenceService, mutate: func(s *Service) { s.ModelServing.Inference().SetAuth = false }, want: true},
		{name: "LLM source change", newService: newLLMService, mutate: func(s *Service) { s.ModelServing.LLMInference().Source = "s3://models/updated" }, want: true},
		{name: "LLM runtime change", newService: newLLMService, mutate: func(s *Service) { s.ModelServing.LLMInference().RuntimeImage = "runtime:v2" }, want: true},
		{name: "LLM minimum scale change", newService: newLLMService, mutate: func(s *Service) { s.ModelServing.LLMInference().MinScale = 2 }, want: true},
		{name: "LLM maximum scale change", newService: newLLMService, mutate: func(s *Service) { s.ModelServing.LLMInference().MaxScale = 3 }, want: true},
		{name: "LLM arguments change", newService: newLLMService, mutate: func(s *Service) { s.ModelServing.LLMInference().Args[0] = "--workers=2" }, want: true},
		{name: "LLM authentication change", newService: newLLMService, mutate: func(s *Service) { s.ModelServing.LLMInference().SetAuth = false }, want: true},
		{name: "environment secret change is not applied to KServe", newService: newInferenceService, mutate: func(s *Service) { s.Environment.Secrets["TOKEN"] = "new-secret" }},
		{name: "queue label change is not applied to KServe", newService: newInferenceService, mutate: func(s *Service) { s.Labels["kueue.x-k8s.io/queue-name"] = "queue-b" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldService := tt.newService()
			newService := tt.newService()
			if tt.mutate != nil {
				tt.mutate(newService)
			}

			if got := modelServingNeedUpdate(oldService, newService); got != tt.want {
				t.Errorf("modelServingNeedUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModelServingIsTypeSupportedByOSCAR(t *testing.T) {
	tests := []struct {
		name string
		ms   ModelServing
		cfg  *Config
		want bool
	}{
		{
			name: "inference supported",
			ms:   ModelServing{Spec: &KserveInference{}},
			cfg:  &Config{KserveEnable: true, ExposedServicesRouteKind: HTTPROUTE},
			want: true,
		},
		{
			name: "llm inference supported",
			ms:   ModelServing{Spec: &KserveLLMInference{}},
			cfg:  &Config{KserveEnable: true, ExposedServicesRouteKind: HTTPROUTE},
			want: true,
		},
		{
			name: "inference unsupported when kserve is disabled",
			ms:   ModelServing{Spec: &KserveInference{}},
			cfg:  &Config{KserveEnable: false, ExposedServicesRouteKind: HTTPROUTE},
		},
		{
			name: "llm inference unsupported when kserve is disabled",
			ms:   ModelServing{Spec: &KserveLLMInference{}},
			cfg:  &Config{KserveEnable: false, ExposedServicesRouteKind: HTTPROUTE},
		},
		{
			name: "inference unsupported with ingress route",
			ms:   ModelServing{Spec: &KserveInference{}},
			cfg:  &Config{KserveEnable: true, ExposedServicesRouteKind: "ingress"},
		},
		{
			name: "llm inference unsupported with ingress route",
			ms:   ModelServing{Spec: &KserveLLMInference{}},
			cfg:  &Config{KserveEnable: true, ExposedServicesRouteKind: "ingress"},
		},
		{
			name: "unsupported type",
			ms:   ModelServing{Spec: &fakeServingSpec{}},
			cfg:  &Config{KserveEnable: true, ExposedServicesRouteKind: HTTPROUTE},
		},
		{
			name: "missing type",
			ms:   ModelServing{},
			cfg:  &Config{KserveEnable: true, ExposedServicesRouteKind: HTTPROUTE},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ms.IsTypeSupportedByOSCAR(tt.cfg); got != tt.want {
				t.Errorf("IsTypeSupportedByOSCAR() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModelServingCheckIfIsPartOfService(t *testing.T) {
	expectPanic := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s: expected panic", name)
			}
		}()
		fn()
	}
	ms := &ModelServing{Spec: &fakeServingSpec{}}
	expectPanic("nil service", func() { ms.checkIfIsPartOfService(nil) })
	expectPanic("service without ModelServing", func() { ms.checkIfIsPartOfService(&Service{}) })
	expectPanic("different ModelServing", func() {
		ms.checkIfIsPartOfService(&Service{ModelServing: &ModelServing{}})
	})

	svc := &Service{ModelServing: ms}
	ms.checkIfIsPartOfService(svc)
	if !reflect.DeepEqual(svc.ModelServing, ms) {
		t.Error("service must not be modified")
	}
}

func TestModelServingServiceMethodsCheckServiceMembership(t *testing.T) {
	tests := []struct {
		name string
		call func(*ModelServing, *Service) error
	}{
		{
			name: "create",
			call: func(ms *ModelServing, service *Service) error {
				return ms.CreateModelServingService(service, nil, &Config{})
			},
		},
		{
			name: "update",
			call: func(ms *ModelServing, service *Service) error {
				return ms.UpdateModelServingService(service, nil, "")
			},
		},
		{
			name: "delete",
			call: func(ms *ModelServing, service *Service) error {
				return ms.DeleteModelServingService(service, "")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &ModelServing{Spec: &fakeServingSpec{}}
			service := &Service{ModelServing: &ModelServing{Spec: &fakeServingSpec{}}}

			defer func() {
				got := recover()
				if got != "the ModelServing service is not part of the provided Service object" {
					t.Errorf("panic = %v, want membership validation panic", got)
				}
			}()
			_ = tt.call(ms, service)
			t.Fatal("expected membership validation panic")
		})
	}
}
