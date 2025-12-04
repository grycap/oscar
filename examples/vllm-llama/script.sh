#!/bin/bash

if [ -z "$API_KEY" ]; then
  python3 -m vllm.entrypoints.openai.api_server --root-path $OPENAI_BASE_URL --model unsloth/Llama-3.2-1B-Instruct --revision 5a8abab4a5d6f164389b1079fb721cfab8d7126c --max_model_len $MAX_MODEL_LEN --enforce-eager --gpu-memory-utilization $GPU_MEMORY_UTILIZATION --dtype auto
else
  python3 -m vllm.entrypoints.openai.api_server --root-path $OPENAI_BASE_URL --model unsloth/Llama-3.2-1B-Instruct --revision 5a8abab4a5d6f164389b1079fb721cfab8d7126c --max_model_len $MAX_MODEL_LEN --enforce-eager --gpu-memory-utilization $GPU_MEMORY_UTILIZATION --api-key $API_KEY --dtype auto
fi
