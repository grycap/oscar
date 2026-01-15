FROM vllm/vllm-openai:v0.12.0

RUN pip install "huggingface_hub[cli]"
RUN huggingface-cli download unsloth/Llama-3.2-1B-Instruct --revision 5a8abab4a5d6f164389b1079fb721cfab8d7126c

RUN pip cache purge

RUN rm -rf /root/.cache/pip/* && \
    rm -rf /tmp/* && \
    rm -rf /var/lib/apt/lists/*

ENV HF_HUB_OFFLINE=1