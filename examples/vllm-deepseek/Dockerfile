FROM vllm/vllm-openai:latest

RUN pip install "huggingface_hub[cli]"
RUN huggingface-cli download deepseek-ai/deepseek-coder-1.3b-instruct

RUN pip cache purge

RUN rm -rf /root/.cache/pip/* && \
    rm -rf /tmp/* && \
    rm -rf /var/lib/apt/lists/*