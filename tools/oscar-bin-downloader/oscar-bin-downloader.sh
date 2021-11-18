#!/bin/sh
ARCH=$(uname -m)

FAAS_SUPERVISOR_NAME=supervisor
WATCHDOG_NAME=fwatchdog

echo "Downloading binaries for $ARCH..."

if [[ $ARCH == "aarch64" ]] || [[ $ARCH == "arm64" ]]; then
    FAAS_SUPERVISOR_NAME=$FAAS_SUPERVISOR_NAME-arm64
    WATCHDOG_NAME=$WATCHDOG_NAME-arm64
fi

# Download FaaS Supervisor and unzip
wget https://github.com/grycap/faas-supervisor/releases/download/$FAAS_SUPERVISOR_VERSION/$FAAS_SUPERVISOR_NAME.zip -O /tmp/supervisor.zip
unzip /tmp/supervisor.zip -d /tmp
cp -r /tmp/supervisor/* /data

# Download OpenFaaS watchdog and set execution permissions
wget https://github.com/openfaas/faas/releases/download/$WATCHDOG_VERSION/$WATCHDOG_NAME -O /data/fwatchdog
chmod +x /data/fwatchdog
