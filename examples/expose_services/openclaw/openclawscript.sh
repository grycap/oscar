#!/bin/bash
set -euo pipefail

# Always load OSCAR's runtime token for OpenClaw gateway auth.
if [[ -z "${OPENCLAW_GATEWAY_TOKEN:-}" ]] && [[ -f "/oscar/config/function_config.yaml" ]]; then
  oscar_token="$(awk -F': ' '/^[[:space:]]*token:/{print $2; exit}' /oscar/config/function_config.yaml || true)"
  if [[ -n "${oscar_token}" ]]; then
    export OPENCLAW_GATEWAY_TOKEN="${oscar_token}"
  fi
fi
if [[ -z "${OPENCLAW_GATEWAY_TOKEN:-}" ]]; then
  echo "error: OPENCLAW_GATEWAY_TOKEN is empty and OSCAR token could not be read from /oscar/config/function_config.yaml" >&2
  exit 1
fi

# OpenClaw startup can exceed Node's default heap limit on constrained containers.
# Tune heap size explicitly unless the user already provided NODE_OPTIONS.
if [[ -z "${NODE_OPTIONS:-}" ]]; then
  export NODE_OPTIONS="--max-old-space-size=768"
fi

# Keep runtime footprint low for OSCAR local/small clusters.
export OPENCLAW_SKIP_CHANNELS="${OPENCLAW_SKIP_CHANNELS:-1}"
export OPENCLAW_SKIP_BROWSER_CONTROL_SERVER="${OPENCLAW_SKIP_BROWSER_CONTROL_SERVER:-1}"
export OPENCLAW_SKIP_CANVAS_HOST="${OPENCLAW_SKIP_CANVAS_HOST:-1}"
export OPENCLAW_SKIP_GMAIL_WATCHER="${OPENCLAW_SKIP_GMAIL_WATCHER:-1}"
export OPENCLAW_SKIP_CRON="${OPENCLAW_SKIP_CRON:-1}"

# Trust typical private proxy ranges so OpenClaw can honor forwarded headers when
# running behind OSCAR ingress and avoid local-client misclassification.
trusted_proxies_json="${OPENCLAW_GATEWAY_TRUSTED_PROXIES:-[\"10.0.0.0/8\",\"172.16.0.0/12\",\"192.168.0.0/16\",\"127.0.0.1/32\",\"::1/128\"]}"
if ! node /app/openclaw.mjs config set gateway.trustedProxies "${trusted_proxies_json}" --json >/dev/null 2>&1; then
  echo "warning: could not set gateway.trustedProxies automatically" >&2
fi

# Force gateway auth mode/token on every startup so persisted local state cannot
# drift away from OSCAR's current service token.
if [[ -n "${OPENCLAW_GATEWAY_TOKEN:-}" ]]; then
  if ! node /app/openclaw.mjs config set gateway.auth.mode token --json >/dev/null 2>&1; then
    echo "warning: could not set gateway.auth.mode=token" >&2
  fi
  if ! node /app/openclaw.mjs config set gateway.auth.token "\"${OPENCLAW_GATEWAY_TOKEN}\"" --json >/dev/null 2>&1; then
    echo "warning: could not set gateway.auth.token" >&2
  fi
fi

# Disable per-device pairing by default to avoid reconnect friction after each
# redeploy when using browser access behind OSCAR ingress.
if [[ "${OPENCLAW_DISABLE_DEVICE_AUTH:-1}" == "1" ]]; then
  if ! node /app/openclaw.mjs config set gateway.controlUi.allowInsecureAuth true --json >/dev/null 2>&1; then
    echo "warning: could not set gateway.controlUi.allowInsecureAuth=true" >&2
  fi
  if ! node /app/openclaw.mjs config set gateway.controlUi.dangerouslyDisableDeviceAuth true --json >/dev/null 2>&1; then
    echo "warning: could not set gateway.controlUi.dangerouslyDisableDeviceAuth=true" >&2
  fi
fi

# Expose the gateway on the container network so OSCAR can route traffic to it.
cmd=(
  node
  /app/openclaw.mjs
  gateway
  --allow-unconfigured
  --bind
  lan
  --port
  "${OPENCLAW_GATEWAY_PORT:-18789}"
)
if [[ -n "${OPENCLAW_GATEWAY_TOKEN:-}" ]]; then
  cmd+=(--token "${OPENCLAW_GATEWAY_TOKEN}")
fi

exec "${cmd[@]}"
