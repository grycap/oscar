#!/bin/bash

if [ -z "$API_KEY" ]; then
    python3 -m vllm.entrypoints.openai.api_server --root-path $OPENAI_BASE_URL --model deepseek-ai/deepseek-coder-1.3b-instruct --max_model_len $MAX_MODEL_LEN --enforce-eager --gpu-memory-utilization $GPU_MEMORY_UTILIZATION 
else
    python3 -m vllm.entrypoints.openai.api_server --root-path $OPENAI_BASE_URL --model deepseek-ai/deepseek-coder-1.3b-instruct --max_model_len $MAX_MODEL_LEN --enforce-eager --gpu-memory-utilization $GPU_MEMORY_UTILIZATION --api-key $API_KEY
fi