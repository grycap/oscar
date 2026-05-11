#!/bin/sh
set -eu

if [ -z "${OPENCLAW_GATEWAY_TOKEN:-}" ] && [ -n "${OSCAR_SERVICE_TOKEN:-}" ]; then
  OPENCLAW_GATEWAY_TOKEN="${OSCAR_SERVICE_TOKEN}"
  export OPENCLAW_GATEWAY_TOKEN
fi

[ -n "${OPENCLAW_STATE_DIR:-}" ] || export OPENCLAW_STATE_DIR="/data/openclaw-state"
[ -n "${OPENCLAW_CONFIG_PATH:-}" ] || export OPENCLAW_CONFIG_PATH="${OPENCLAW_STATE_DIR}/openclaw.json"

mkdir -p "${OPENCLAW_STATE_DIR}" "$(dirname "${OPENCLAW_CONFIG_PATH}")"

node /app/openclaw.mjs config set gateway.trustedProxies '["10.0.0.0/8","172.16.0.0/12","192.168.0.0/16","127.0.0.1/32","::1/128"]' --json >/dev/null 2>&1 || true
node /app/openclaw.mjs config set gateway.auth.mode '"token"' --json >/dev/null 2>&1 || true
[ -n "${OPENCLAW_GATEWAY_TOKEN:-}" ] && node /app/openclaw.mjs config set gateway.auth.token "\"${OPENCLAW_GATEWAY_TOKEN}\"" --json >/dev/null 2>&1 || true

if [ "${OPENCLAW_DISABLE_DEVICE_AUTH:-1}" = "1" ]; then
  node /app/openclaw.mjs config set gateway.controlUi.allowInsecureAuth true --json >/dev/null 2>&1 || true
  node /app/openclaw.mjs config set gateway.controlUi.dangerouslyDisableDeviceAuth true --json >/dev/null 2>&1 || true
fi

if [ -n "${OPENCLAW_GATEWAY_ALLOWED_ORIGINS:-}" ]; then
  node /app/openclaw.mjs config set gateway.controlUi.allowedOrigins "${OPENCLAW_GATEWAY_ALLOWED_ORIGINS}" --json >/dev/null 2>&1 || true
else
  node /app/openclaw.mjs config set gateway.controlUi.dangerouslyAllowHostHeaderOriginFallback true --json >/dev/null 2>&1 || true
fi

exec node /app/openclaw.mjs gateway --allow-unconfigured --bind lan --port "${OPENCLAW_GATEWAY_PORT:-18789}" ${OPENCLAW_GATEWAY_TOKEN:+--token ${OPENCLAW_GATEWAY_TOKEN}}
