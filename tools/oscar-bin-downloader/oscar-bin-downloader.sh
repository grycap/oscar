#!/bin/sh
ARCH=$(uname -m)

FAAS_SUPERVISOR_NAME=supervisor
FAAS_SUPERVISOR_ALPINE_NAME=supervisor-alpine
WATCHDOG_NAME=fwatchdog

echo "Downloading binaries for $ARCH..."

if [[ $ARCH == "aarch64" ]] || [[ $ARCH == "arm64" ]]; then
    FAAS_SUPERVISOR_NAME=$FAAS_SUPERVISOR_NAME-arm64
    FAAS_SUPERVISOR_ALPINE_NAME=$FAAS_SUPERVISOR_ALPINE_NAME-arm64
    WATCHDOG_NAME=$WATCHDOG_NAME-arm64
fi

# Download FaaS Supervisor and unzip
wget https://github.com/grycap/faas-supervisor/releases/download/$FAAS_SUPERVISOR_VERSION/$FAAS_SUPERVISOR_NAME.zip -O /tmp/supervisor.zip
unzip /tmp/supervisor.zip -d /tmp
cp -r /tmp/supervisor/* /data

# Download Alpine release of FaaS Supervisor and unzip
wget https://github.com/grycap/faas-supervisor/releases/download/$FAAS_SUPERVISOR_VERSION/$FAAS_SUPERVISOR_ALPINE_NAME.zip -O /tmp/supervisor-alpine.zip
mkdir /data/alpine
mkdir /tmp/alpine
unzip /tmp/supervisor-alpine.zip -d /tmp/alpine
cp -r /tmp/alpine/supervisor/* /data/alpine

# Download OpenFaaS watchdog and set execution permissions
wget https://github.com/openfaas/faas/releases/download/$WATCHDOG_VERSION/$WATCHDOG_NAME -O /data/fwatchdog
chmod +x /data/fwatchdog
