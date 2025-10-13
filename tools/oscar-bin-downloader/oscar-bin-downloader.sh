#!/bin/sh
ARCH=$(uname -m)

FAAS_SUPERVISOR_NAME=supervisor
FAAS_SUPERVISOR_ALPINE_NAME=supervisor-alpine

echo "Downloading binaries for $ARCH..."

if [[ $ARCH == "aarch64" ]] || [[ $ARCH == "arm64" ]]; then
    FAAS_SUPERVISOR_NAME=$FAAS_SUPERVISOR_NAME-arm64
    FAAS_SUPERVISOR_ALPINE_NAME=$FAAS_SUPERVISOR_ALPINE_NAME-arm64
fi

# Download FaaS Supervisor and unzip
wget "https://github.com/grycap/faas-supervisor/releases/download/$FAAS_SUPERVISOR_VERSION/$FAAS_SUPERVISOR_NAME.zip" -O /tmp/supervisor.zip
unzip /tmp/supervisor.zip -d /tmp
cp -r /tmp/supervisor/* /data

# Download Alpine release of FaaS Supervisor and unzip
wget "https://github.com/grycap/faas-supervisor/releases/download/$FAAS_SUPERVISOR_VERSION/$FAAS_SUPERVISOR_ALPINE_NAME.zip" -O /tmp/supervisor-alpine.zip
mkdir /data/alpine
mkdir /tmp/alpine
unzip /tmp/supervisor-alpine.zip -d /tmp/alpine
cp -r /tmp/alpine/supervisor/* /data/alpine
