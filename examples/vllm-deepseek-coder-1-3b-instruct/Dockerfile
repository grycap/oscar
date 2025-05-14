FROM vllm/vllm-openai:latest

RUN pip install "huggingface_hub[cli]"
RUN huggingface-cli download deepseek-ai/deepseek-coder-1.3b-instruct

RUN pip cache purge

RUN rm -rf /root/.cache/pip/* && \
    rm -rf /tmp/* && \
    rm -rf /var/lib/apt/lists/*

ENTRYPOINT ["vllm", "serve", "deepseek-ai/deepseek-coder-1.3b-instruct", "--max_model_len=2048", "--gpu-memory-utilization=0.6"]