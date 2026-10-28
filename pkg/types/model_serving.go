package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"

	corev1 "k8s.io/api/core/v1"
	knv1 "knative.dev/serving/pkg/apis/serving/v1"
)

// List of supported ModelServing service types. Each type is registered with RegisterModelServingSpec.
const (
	kserveTypeInferenceService    = "inference"
	kserveTypeLLMInferenceService = "llm_inference"
)

var (
	ErrModelServingTypeConflict  = errors.New("conflicting ModelServing service type, you cannot have more than one service type configured at the same time")
	ErrModelServingTypeInvalid   = errors.New("invalid ModelServing service type")
	ErrModelServingSourceMissing = errors.New("missing model storage URI in ModelServing configuration")
	ErrModelServingSourceUpdate  = errors.New("cannot update model storage URI for ModelServing")
)

// ModelServingSpec is the type-specific part of a ModelServing configuration.
// Each implementation is registered with RegisterModelServingSpec and is
// exposed in the JSON document under the key returned by Type.
type ModelServingSpec interface {
	// Type returns the model serving type identifier and its JSON key.
	Type() string
	// validate validates the type-specific configuration.
	validate() error
	// validateUpdate checks that moving from old to the receiver is allowed.
	validateUpdate(old ModelServingSpec) error
	// svcName returns the name of the Kubernetes Service backing the serviceName service.
	svcName(serviceName string) string
	// podAndDplName returns the name of the Deployment and pods backing the serviceName service.
	podAndDplName(serviceName string) string
	// labelSelector returns the label selector that matches the pods of the serviceName service.
	labelSelector(serviceName string) string
	// getLabelSelectorKey returns the label key used for the label selector that matches the pods of the serviceName service.
	getLabelSelectorKey() string
	// getLabelSelectorValue returns the label value used for the label selector that matches the pods of the serviceName service.
	getLabelSelectorValue(serviceName string) string
	// getClusterInternalURI returns the internal path/uri used to find the service inside the cluster.
	getClusterInternalURI(serviceName string, namespace string) string
	// getConteinerName returns the name of the container inside the pod
	getConteinerName() string

	/*
	* This methods are used to manage the lifecycle of the model serving service.
	* Implementations should provide the logic to create, update, and delete the service as needed.
	 */

	// GetKueueResourceList returns the resource list and minScale for the Kueue queue, or an error if the spec is invalid.
	getKueueResourceList(service *Service) (request corev1.ResourceList, minScale int32, err error)

	// UpdateModelServingService updates the model serving service based on the provided service and oldService specifications.
	updateModelServingService(service *Service, oldService *Service, namespace string) error

	// CreateModelServingService creates the model serving service based on the provided service specification.
	createModelServingService(service *Service, knativeService *knv1.Service, cfg *Config) error

	// DeleteModelServingService deletes the model serving service based on the provided service specification.
	deleteModelServingService(service *Service, namespace string) error
}

// ModelServingScalableSpec is an optional interface for ModelServingSpec implementations that support scaling.
type ModelServingScalableSpec interface {
	GetScaling() ModelServingScalable
}

type ModelServingScalable struct {
	MinScale int32
	MaxScale int32
}

// ModelServingAuthSpec is an optional interface for ModelServingSpec implementations that support authentication.
type ModelServingAuthSpec interface {
	GetAuth() bool
}

type modelServingSpecDecoder func(raw json.RawMessage) (ModelServingSpec, error)

var modelServingSpecDecoders = map[string]modelServingSpecDecoder{}

// RegisterModelServingSpec registers a new model serving type. It is meant to be
// called from init functions, adding a type does not require modifying ModelServing.
func RegisterModelServingSpec(modelType string, decode modelServingSpecDecoder) {
	if _, exists := modelServingSpecDecoders[modelType]; exists {
		panic(fmt.Sprintf("ModelServing type %q is already registered", modelType))
	}
	modelServingSpecDecoders[modelType] = decode
}

func registeredModelServingTypes() []string {
	names := make([]string, 0, len(modelServingSpecDecoders))
	for modelType := range modelServingSpecDecoders {
		names = append(names, modelType)
	}
	sort.Strings(names)
	return names
}

type ModelServing struct {
	// Spec is the type-specific configuration (e.g. *KserveInference or *KserveLLMInference).
	// It is serialized under the key of its Type, and the service type is always derived from it.
	Spec ModelServingSpec `json:"-"`
}

// modelServingAlias drops the ModelServing methods to avoid recursion in (Un)MarshalJSON.
type modelServingAlias ModelServing

// UnmarshalJSON decodes the type-specific configuration with the decoder
// registered for the key found in the document. Defaults for omitted fields are
// set by each decoder.
func (m *ModelServing) UnmarshalJSON(data []byte) error {
	aux := modelServingAlias{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	for _, modelType := range registeredModelServingTypes() {
		body, ok := raw[modelType]
		if !ok || string(body) == "null" {
			continue
		}
		if aux.Spec != nil {
			return ErrModelServingTypeConflict
		}
		spec, err := modelServingSpecDecoders[modelType](body)
		if err != nil {
			return err
		}
		aux.Spec = spec
	}

	*m = ModelServing(aux)
	return nil
}

// MarshalJSON writes the type-specific configuration under the key of its Type.
func (m ModelServing) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(modelServingAlias(m))
	if err != nil || m.Spec == nil {
		return data, err
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	spec, err := json.Marshal(m.Spec)
	if err != nil {
		return nil, err
	}
	fields[m.Spec.Type()] = spec
	return json.Marshal(fields)
}

// MarshalYAML implements yaml.InterfaceMarshaler.
func (m ModelServing) MarshalYAML() (interface{}, error) {
	data, err := m.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	return out, json.Unmarshal(data, &out)
}

// UnmarshalYAML implements yaml.BytesUnmarshaler.
func (m *ModelServing) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw map[string]interface{}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return m.UnmarshalJSON(data)
}

// GetType returns the model serving type, or an empty string if no type is configured.
func (m ModelServing) GetType() string {
	if m.Spec == nil {
		return ""
	}
	return m.Spec.Type()
}

func (m ModelServing) GetModelServingType() string {
	return m.GetType()
}

func (m ModelServing) IsInferenceService() bool {
	return m.Inference() != nil
}

func (m ModelServing) IsLLMInferenceService() bool {
	return m.LLMInference() != nil
}

// Inference returns the KServe InferenceService configuration, or nil for other types.
func (m ModelServing) Inference() *KserveInference {
	spec, _ := m.Spec.(*KserveInference)
	return spec
}

// LLMInference returns the KServe LLMInferenceService configuration, or nil for other types.
func (m ModelServing) LLMInference() *KserveLLMInference {
	spec, _ := m.Spec.(*KserveLLMInference)
	return spec
}

func (m ModelServing) GetScaling() (ModelServingScalable, bool) {
	s, ok := m.Spec.(ModelServingScalableSpec)
	if !ok {
		return ModelServingScalable{}, false
	}
	return s.GetScaling(), true
}

func (m ModelServing) GetAuth() (bool, bool) {
	a, ok := m.Spec.(ModelServingAuthSpec)
	if !ok {
		return false, false
	}
	return a.GetAuth(), true
}

func (m ModelServing) Validate() error {
	if m.Spec == nil {
		return ErrModelServingTypeInvalid
	}
	return m.Spec.validate()
}

func (m ModelServing) ValidateUpdate(old ModelServing) error {
	newType := m.GetType()
	oldType := old.GetType()
	if oldType != newType {
		return fmt.Errorf("cannot change ModelServing service type from %q to %q", oldType, newType)
	}
	if m.Spec == nil {
		return ErrModelServingTypeInvalid
	}
	if err := m.Validate(); err != nil {
		return fmt.Errorf("invalid ModelServing service: %v", err)
	}

	return m.Spec.validateUpdate(old.Spec)
}

// GetSvcName returns the Kubernetes Service name, or an empty string if no type is configured.
func (m ModelServing) GetSvcName(serviceName string) string {
	if m.Spec == nil {
		return ""
	}
	return m.Spec.svcName(serviceName)
}

// GetPodAndDplName returns the Deployment and pod name, or an empty string if no type is configured.
func (m ModelServing) GetPodAndDplName(serviceName string) string {
	if m.Spec == nil {
		return ""
	}
	return m.Spec.podAndDplName(serviceName)
}

// GetLabelSelector returns the pod label selector, or an empty string if no type is configured.
func (m ModelServing) GetLabelSelector(serviceName string) string {
	if m.Spec == nil {
		return ""
	}
	return m.Spec.labelSelector(serviceName)
}

func (m ModelServing) GetLabelSelectorKey() string {
	if m.Spec == nil {
		return ""
	}
	return m.Spec.getLabelSelectorKey()
}

func (m ModelServing) GetLabelSelectorValue(serviceName string) string {
	if m.Spec == nil {
		return ""
	}
	return m.Spec.getLabelSelectorValue(serviceName)
}

func (m ModelServing) GetClusterInternalURI(modelServingServiceName string, namespace string) string {
	if m.Spec == nil {
		return ""
	}
	return m.Spec.getClusterInternalURI(modelServingServiceName, namespace)
}

func (m ModelServing) GetContainerName() string {
	if m.Spec == nil {
		return ""
	}
	return m.Spec.getConteinerName()
}

func (m ModelServing) GetKueueResourceList(service *Service) (corev1.ResourceList, int32, error) {
	if m.Spec == nil {
		return nil, 0, fmt.Errorf("model serving backend is nil")
	}
	return m.Spec.getKueueResourceList(service)
}

func (m ModelServing) UpdateModelServingService(service *Service, oldService *Service, namespace string) error {
	if m.Spec == nil {
		return fmt.Errorf("model serving backend is nil")
	}
	m.checkIfIsPartOfService(service)

	if oldService == nil || service == nil {
		panic("unexpected error in ModelServing service update validation: oldService or newService is nil")
	}
	if oldService.Token != service.Token {
		panic("unexpected error in ModelServing service update validation: service tokens do not match")
	}

	if err := m.ValidateUpdate(*oldService.ModelServing); err != nil {
		return fmt.Errorf("invalid ModelServing service update: %v", err)
	}

	return m.Spec.updateModelServingService(service, oldService, namespace)
}

func (m ModelServing) CreateModelServingService(service *Service, knativeService *knv1.Service, cfg *Config) error {
	if m.Spec == nil {
		return fmt.Errorf("model serving backend is nil")
	}
	m.checkIfIsPartOfService(service)

	if err := m.Validate(); err != nil {
		return fmt.Errorf("invalid ModelServing service: %v", err)
	}

	return m.Spec.createModelServingService(service, knativeService, cfg)
}

func (m ModelServing) DeleteModelServingService(service *Service, namespace string) error {
	if m.Spec == nil {
		return fmt.Errorf("model serving backend is nil")
	}
	m.checkIfIsPartOfService(service)
	return m.Spec.deleteModelServingService(service, namespace)
}

func (m ModelServing) Equal(other ModelServing) bool {
	return reflect.DeepEqual(m.Spec, other.Spec)
}

// checkIfIsPartOfService checks if the ModelServing service is part of the provided Service object.
// It panics if the service is nil, the ModelServing field is nil, or the ModelServing service does not match.
func (m *ModelServing) checkIfIsPartOfService(service *Service) {
	if service == nil || service.ModelServing == nil || service.ModelServing != m {
		panic("the ModelServing service is not part of the provided Service object")
	}
}

// IsTypeSupported checks if the ModelServing service is supported in the current OSCAR deployment configuration.
func (m ModelServing) IsTypeSupportedByOSCAR(cfg *Config) bool {
	typeName := m.GetType()
	switch {
	// KServe Backends
	case typeName == kserveTypeInferenceService || typeName == kserveTypeLLMInferenceService:
		return cfg.KserveEnable && cfg.ExposedServicesRouteKind == HTTPROUTE
	default:
		return false
	}
}
