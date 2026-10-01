#!/bin/sh
ARCH=$(uname -m)

FAAS_SUPERVISOR_NAME=supervisor
WATCHDOG_NAME=fwatchdog-amd64

echo "Downloading binaries for $ARCH..."

if [[ $ARCH == "aarch64" ]] || [[ $ARCH == "arm64" ]]; then
    FAAS_SUPERVISOR_NAME=$FAAS_SUPERVISOR_NAME-arm64
    WATCHDOG_NAME=fwatchdog-arm64
fi

# Download OSCAR Supervisor binary
wget "https://github.com/grycap/oscar-supervisor/releases/download/v$OSCAR_SUPERVISOR_VERSION/$FAAS_SUPERVISOR_NAME" -O /tmp/supervisor
cp -r /tmp/supervisor /data/supervisor
chmod +x /data/supervisor

# Download OpenFaaS watchdog and set execution permissions
wget "https://github.com/openfaas/classic-watchdog/releases/download/$WATCHDOG_VERSION/$WATCHDOG_NAME" -O /data/fwatchdog
chmod +x /data/fwatchdog
