FROM bitnami/minideb:bullseye

ENV LANG C.UTF-8
ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y git curl zip jq libjpeg-dev \
	libgl1-mesa-glx \
        libglib2.0-0 \
	python3 python3-pip 

RUN pip3 install numpy --upgrade
RUN pip3 install torch torchvision
RUN pip3 install awscli
RUN git clone https://github.com/NeelBhowmik/efficient-compact-fire-detection-cnn.git
RUN pip3 install -r efficient-compact-fire-detection-cnn/requirements.txt

#Download pretrained models
RUN cd efficient-compact-fire-detection-cnn && sh ./download-models.sh
COPY inference_ff_oscar.py efficient-compact-fire-detection-cnn/inference_ff_oscar.py
