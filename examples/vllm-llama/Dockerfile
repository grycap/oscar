FROM vllm/vllm-openai:latest

RUN huggingface-cli download unsloth/Llama-3.2-1B-Instruct

RUN pip cache purge

RUN rm -rf /root/.cache/pip/* && \
    rm -rf /tmp/* && \
    rm -rf /var/lib/apt/lists/*