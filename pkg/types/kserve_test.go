package types

import (
	"encoding/json"
	"testing"
)

func TestKserveUnmarshalJSON(t *testing.T) {
	// Test default unmarshaling
	data := []byte(`{
	"type": "inference",
	"storage_uri": "s3://bucket/model",
	"inference": {
		"model_format": "onnx"
	}
}`)
	var kserve Kserve
	err := json.Unmarshal(data, &kserve)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if kserve.MinScale != 0 {
		t.Errorf("Expected default MinScale 0, got %d", kserve.MinScale)
	}
	if kserve.MaxScale != 1 {
		t.Errorf("Expected default MaxScale 1, got %d", kserve.MaxScale)
	}
	if kserve.CPU != "0.2" {
		t.Errorf("Expected default CPU '0.2', got %q", kserve.CPU)
	}
	if kserve.Memory != "256Mi" {
		t.Errorf("Expected default Memory '256Mi', got %q", kserve.Memory)
	}
	if kserve.EnableGPU != false {
		t.Errorf("Expected default EnableGPU false, got %v", kserve.EnableGPU)
	}
	if kserve.SetAuth != true {
		t.Errorf("Expected default SetAuth true, got %v", kserve.SetAuth)
	}

	// Test invalid unmarshaling (missing required StorageUri)
	invalidData := []byte(`{"type": "inference", "inference": {"model_format": "onnx"}, "storage_uri": ""}`)
	var kserveInvalid Kserve
	err = json.Unmarshal(invalidData, &kserveInvalid)
	if err != nil {
		t.Fatal("Unmarshal should not fail for missing StorageUri, validation is done separately")
	}
	err = kserveInvalid.Validate()
	if err != nil {
		t.Fatalf("Expected validation error for missing StorageUri, got: %v", err)
	}
	expectedErr := "missing model storage URI in KServe configuration"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message:\n%s\nGot:\n%s", expectedErr, err.Error())
	}
}

func TestKserveValidateInvalidType(t *testing.T) {
	// Test unknown type
	kserve := Kserve{
		Type:       "unknown_type",
		StorageUri: "some_uri",
	}
	err := kserve.Validate()
	if err == nil {
		t.Fatal("Expected validation error for unknown type, got nil")
	}
	expectedErr := "invalid KServe service type \"unknown_type\", expected \"inference\" or \"llm_inference\""
	if err.Error() != expectedErr {
		t.Errorf("Expected error message:\n%s\nGot:\n%s", expectedErr, err.Error())
	}
}

func TestKserveValidateMissingStorageUri(t *testing.T) {
	kserve := Kserve{
		Type: "inference",
		Inference: &KserveInference{
			ModelFormat: "onnx",
			APIVersion:  "v1",
		},
		StorageUri: "", // Missing
	}
	err := kserve.Validate()
	if err == nil {
		t.Fatal("Expected validation error for missing StorageUri, got nil")
	}
	expectedErr := "missing model storage URI in KServe configuration"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message:\n%s\nGot:\n%s", expectedErr, err.Error())
	}
}

func TestKserveValidateInferenceServiceMissingInferenceConfig(t *testing.T) {
	kserve := Kserve{
		Type:       KserveTypeInferenceService,
		StorageUri: "some_uri",
		Inference:  nil, // Missing
	}
	err := kserve.Validate()
	if err == nil {
		t.Fatal("Expected validation error for missing Inference configuration, got nil")
	}
	expectedErr := "missing Inference configuration for KServe service"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message:\n%s\nGot:\n%s", expectedErr, err.Error())
	}
}

func TestKserveValidateInferenceServiceHasLLMInferenceConfig(t *testing.T) {
	kserve := Kserve{
		Type: KserveTypeInferenceService,
		Inference: &KserveInference{
			ModelFormat: "onnx",
		},
		LLMInference: &KserveLLMInference{
			RuntimeImage: "vLLM",
		},
		StorageUri: "some_uri",
	}
	err := kserve.Validate()
	if err == nil {
		t.Fatal("Expected validation error for having LLMInference config in InferenceService, got nil")
	}
	expectedErr := "can't have LLMInference configuration for InferenceService"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message:\n%s\nGot:\n%s", expectedErr, err.Error())
	}
}

func TestKserveValidateLLMInferenceServiceHasInferenceConfig(t *testing.T) {
	kserve := Kserve{
		Type: KserveTypeLLMInferenceService,
		Inference: &KserveInference{
			ModelFormat: "pytorch",
		},
		StorageUri: "some_uri",
	}
	err := kserve.Validate()
	if err == nil {
		t.Fatal("Expected validation error for having Inference config in LLMInferenceService, got nil")
	}
	expectedErr := "can't have Inference configuration for LLMInferenceService"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message:\n%s\nGot:\n%s", expectedErr, err.Error())
	}
}

func TestKserveValidateEqual(t *testing.T) {
	kserve1 := Kserve{
		Type:       "inference",
		StorageUri: "uri1",
		MinScale:   1,
		MaxScale:   2,
		CPU:        "0.5",
		Memory:     "512Mi",
		Inference: &KserveInference{
			ModelFormat: "onnx",
			APIVersion:  "v2",
		},
		SetAuth: true,
		Args:    []string{"arg1"},
		Env:     map[string]string{"KEY": "VAL1"},
	}

	kserve2 := Kserve{
		Type:       "inference",
		StorageUri: "uri1",
		MinScale:   1,
		MaxScale:   2,
		CPU:        "0.5",
		Memory:     "512Mi",
		Inference: &KserveInference{
			ModelFormat: "onnx",
			APIVersion:  "v2",
		},
		SetAuth: true,
		Args:    []string{"arg1"},
		Env:     map[string]string{"KEY": "VAL1"},
	}

	kserve3 := Kserve{
		Type:       "inference",
		StorageUri: "uri2", // Different URI
		MinScale:   1,
		MaxScale:   2,
		CPU:        "0.5",
		Memory:     "512Mi",
		Inference: &KserveInference{
			ModelFormat: "onnx",
			APIVersion:  "v2",
		},
		SetAuth: true,
		Args:    []string{"arg1"},
		Env:     map[string]string{"KEY": "VAL1"},
	}

	kserve4 := Kserve{
		Type:       "llm_inference", // Different Type
		StorageUri: "uri1",
		MinScale:   1,
		MaxScale:   2,
		CPU:        "0.5",
		Memory:     "512Mi",
		Inference:  nil,
		LLMInference: &KserveLLMInference{
			RuntimeImage: "vLLM",
		},
		SetAuth: true,
		Args:    []string{"arg1"},
		Env:     map[string]string{"KEY": "VAL1"},
	}

	kserve5 := Kserve{
		Type:       "inference",
		StorageUri: "uri1",
		MinScale:   2, // Different MinScale
		MaxScale:   2,
		CPU:        "0.5",
		Memory:     "512Mi",
		Inference: &KserveInference{
			ModelFormat: "onnx",
			APIVersion:  "v2",
		},
		SetAuth: true,
		Args:    []string{"arg1"},
		Env:     map[string]string{"KEY": "VAL1"},
	}

	t.Run("Equal_True", func(t *testing.T) {
		if !kserve1.Equal(kserve2) {
			t.Error("kserve1 and kserve2 should be equal")
		}
	})

	t.Run("Equal_False_StorageUri", func(t *testing.T) {
		if kserve1.Equal(kserve3) {
			t.Error("kserve1 and kserve3 should NOT be equal due to different StorageUri")
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

	// Test invalid API Version
	dataInvalid := []byte(`{"model_format": "onnx", "api_version": "v3"}`)
	var kserveInferenceInvalid KserveInference
	err = json.Unmarshal(dataInvalid, &kserveInferenceInvalid)
	if err == nil {
		t.Fatal("Expected unmarshal to fail due to invalid API version, but it succeeded.")
	}
}

func TestKserveInferenceValidate(t *testing.T) {
	// Test missing ModelFormat
	kserve := KserveInference{
		ModelFormat: "",
		APIVersion:  "v1",
	}
	err := kserve.Validate()
	if err == nil {
		t.Fatal("Expected validation error for missing ModelFormat, got nil")
	}
	expectedErr := "Kserve Inference ModelFormat is required"
	if err.Error() != expectedErr {
		t.Errorf("Expected error message:\n%s\nGot:\n%s", expectedErr, err.Error())
	}

	// Test invalid API Version
	kserve = KserveInference{
		ModelFormat: "onnx",
		APIVersion:  "v3",
	}
	err = kserve.Validate()
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
	}
	err = kserveValid.Validate()
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
