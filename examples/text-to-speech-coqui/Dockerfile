FROM bitnami/minideb:bullseye

ENV LANG C.UTF-8
ENV DEBIAN_FRONTEND noninteractive

RUN apt update && \
    apt upgrade -y && \
    apt -y install --no-install-recommends python3-pip build-essential python3-dev libsndfile1-dev && \
    apt clean && \
    rm -rf /var/lib/apt/lists/*

RUN pip install TTS
