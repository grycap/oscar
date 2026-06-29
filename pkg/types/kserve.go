package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	KserveTypeInferenceService    = "inference"
	KserveTypeLLMInferenceService = "llm_inference"
)

type Kserve struct {
	// Type the type of KServe service to deploy
	// Required. Set the type of KServe service, either "inference" for a standard InferenceService
	// or "llm_inference" for an LLMInferenceService
	Type string `json:"type,omitempty" default:"inference"`

	// Inference configuration for KServe InferenceService.
	// It is required when Type is set to "inference"
	Inference *KserveInference `json:"inference,omitempty"`

	// LLMInference configuration for KServe LLMInferenceService.
	// It is required when Type is set to "llm_inference"
	LLMInference *KserveLLMInference `json:"llm_inference,omitempty"`

	// StorageUri the URI of the model storage for KServe
	// Required. It should follow the format expected by KServe, for example:
	StorageUri string `json:"storage_uri"`

	// MinScale minimum number of active replicas (pods) for the service
	// Optional. (default: 0)
	MinScale int32 `json:"min_scale,omitempty" default:"0"`

	// MaxScale maximum number of active replicas (pods) for the service
	// Optional. (default: 1)
	MaxScale int32 `json:"max_scale,omitempty" default:"1"`

	// CPU cpu limit for the service following the kubernetes format
	// https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu
	// Optional. (default: 0.2)
	CPU string `json:"cpu" default:"0.2"`

	// Memory memory limit for the service following the kubernetes format
	// https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-memory
	// Optional. (default: 256Mi)
	Memory string `json:"memory" default:"256Mi"`

	// Args command-line arguments to be passed to the container
	// Optional
	Args []string `json:"args,omitempty"`

	// Environment variables to be passed to the container
	// Optional
	Env map[string]string `json:"env,omitempty"`

	// EnableGPU parameter to request gpu usage in KServe InferenceService
	// Optional. (default: false)
	EnableGPU bool `json:"enable_gpu,omitempty" default:"false"`

	// SetAuth parameter to set the authentication for the KServe InferenceService
	// Optional. (default: true)
	SetAuth bool `json:"set_auth,omitempty" default:"true"`
}

// UnmarshalJSON sets KServe defaults for fields that may be omitted in API requests.
func (k *Kserve) UnmarshalJSON(data []byte) error {
	type Alias Kserve

	aux := Alias{
		CPU:       "0.2",
		Memory:    "256Mi",
		SetAuth:   true,
		MinScale:  0,
		MaxScale:  1,
		EnableGPU: false,
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	ks := Kserve(aux)
	if err := ks.Validate(); err != nil {
		return err
	}

	*k = ks

	return nil
}

func (k Kserve) IsInferenceService() bool {
	return k.Type == KserveTypeInferenceService
}

func (k Kserve) IsLLMInferenceService() bool {
	return k.Type == KserveTypeLLMInferenceService
}

func (k Kserve) Validate() error {
	switch k.Type {
	case KserveTypeInferenceService:
		if err := k.validateInferenceService(); err != nil {
			return err
		}

	case KserveTypeLLMInferenceService:
		if err := k.validateLLMInferenceService(); err != nil {
			return err
		}

	default:
		return fmt.Errorf(
			"invalid KServe service type %q, expected %q or %q",
			k.Type,
			KserveTypeInferenceService,
			KserveTypeLLMInferenceService,
		)
	}

	if k.StorageUri == "" {
		return fmt.Errorf("missing model storage URI in KServe configuration")
	}

	return nil
}

func (k Kserve) ValidateUpdate(old Kserve) error {
	if old.StorageUri != k.StorageUri {
		return fmt.Errorf("cannot update model storage configuration for KServe")
	}
	// TODO
	if old.SetAuth != k.SetAuth {
		return fmt.Errorf("cannot update authentication configuration for KServe")
	}
	if old.Type != k.Type {
		return fmt.Errorf("cannot change KServe service type")
	} else if k.IsInferenceService() {
		if old.Inference == nil || k.Inference == nil {
			return fmt.Errorf("inference configuration cannot be nil for KServe service")
		}
		if old.Inference.Runtime != k.Inference.Runtime {
			return fmt.Errorf("cannot update runtime for KServe")
		}
		if old.Inference.ModelFormat != k.Inference.ModelFormat {
			return fmt.Errorf("cannot update model format for KServe")
		}
	}

	return nil
}

func (k Kserve) validateInferenceService() error {
	if k.Inference == nil {
		return fmt.Errorf("missing Inference configuration for KServe service")
	}

	if k.LLMInference != nil {
		return fmt.Errorf("can't have LLMInference configuration for InferenceService")
	}
	return nil
}

func (k Kserve) validateLLMInferenceService() error {
	if k.Inference != nil {
		return fmt.Errorf("Inference configuration must be nil for LLMInferenceService")
	}

	return nil
}

type KserveInference struct {
	// ModelFormat the model format to use for KServe InferenceService
	// ("onnx", "sklearn", "xgboost", "pytorch", "tensorflow", "triton", "huggingface").
	ModelFormat string `json:"model_format,omitempty"`
	// Runtime the KServe runtime to use
	// Ref: https://kserve.github.io/website/docs/concepts/resources/servingruntime
	// Optional.
	Runtime string `json:"runtime,omitempty"`
	// Can be used to specify the protocol version for KServe (e.g., "v1", "v2").
	// Optional. (default: "v1")
	APIVersion string `json:"api_version,omitempty" default:"v1"`
}

func (k *KserveInference) UnmarshalJSON(data []byte) error {
	type Alias KserveInference

	aux := Alias{
		APIVersion: "v1",
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	kaux := KserveInference(aux)
	err := kaux.Validate()
	if err != nil {
		return err
	}

	*k = kaux
	return nil
}

func (k KserveInference) Validate() error {
	if strings.TrimSpace(k.ModelFormat) == "" {
		return fmt.Errorf("Kserve Inference ModelFormat is required")
	}

	switch k.APIVersion {
	case "v1", "v2":
		return nil
	default:
		return fmt.Errorf("invalid API version %q, expected v1 or v2", k.APIVersion)
	}
}

type KserveLLMInference struct {
	// At the moment only supported for LLMInferenceService,
	// the runtime image to use for KServe when IsLLM is true
	// Optional. (default: a custom image based on vLLM for CPU and another one with GPU support)
	RuntimeImage string `json:"runtime_image,omitempty"`
}
