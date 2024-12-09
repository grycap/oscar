FROM nvidia/cuda:11.8.0-cudnn8-runtime-ubuntu22.04

RUN apt update && \
    apt install -y --no-install-recommends git wget python3-pip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

RUN git clone https://github.com/srisco/stable-diffusion-tensorflow.git

WORKDIR stable-diffusion-tensorflow

RUN pip install -r requirements.txt && \
    rm -rf /root/.cache/pip/* && \
    rm -rf /tmp/*

# DOWNLOAD WEIGHTS
RUN mkdir -p /root/.keras/datasets && \
    wget https://huggingface.co/fchollet/stable-diffusion/resolve/main/text_encoder.h5 -O /root/.keras/datasets/text_encoder.h5 && \
    wget https://huggingface.co/fchollet/stable-diffusion/resolve/main/diffusion_model.h5 -O /root/.keras/datasets/diffusion_model.h5 && \
    wget https://huggingface.co/fchollet/stable-diffusion/resolve/main/decoder.h5 -O /root/.keras/datasets/decoder.h5 && \
    wget https://huggingface.co/divamgupta/stable-diffusion-tensorflow/resolve/main/encoder_newW.h5 -O /root/.keras/datasets/encoder_newW.h5
