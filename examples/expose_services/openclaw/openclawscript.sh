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

# If a bucket is mounted by OSCAR, rclone mounts it under /mnt/<mount.path>.
# Resolve the mount path from env first, then fallback to OSCAR runtime config.
mount_path="${OPENCLAW_MOUNT_PATH:-}"
if [[ -z "${mount_path}" ]] && [[ -f "/oscar/config/function_config.yaml" ]]; then
  mount_path="$(
    awk -F': ' '
      /^[[:space:]]*mount:[[:space:]]*$/ { in_mount=1; next }
      in_mount && /^[[:space:]]*path:[[:space:]]*/ { print $2; exit }
    ' /oscar/config/function_config.yaml || true
  )"
fi
mount_path="${mount_path%\"}"
mount_path="${mount_path#\"}"
mount_path="${mount_path#/}"
mount_target=""
if [[ -n "${mount_path}" ]]; then
  mount_target="/mnt/${mount_path}"
fi

# OSCAR may not propagate FDL environment variables into exposed service pods in
# some deployments. Set robust defaults here so behavior does not depend on FDL envs.
OPENCLAW_PERSIST_CONFIG_ONLY="${OPENCLAW_PERSIST_CONFIG_ONLY:-1}"
OPENCLAW_LOCAL_STATE_DIR="${OPENCLAW_LOCAL_STATE_DIR:-/tmp/openclaw-state}"
OPENCLAW_CONFIG_SYNC_INTERVAL="${OPENCLAW_CONFIG_SYNC_INTERVAL:-30}"

# Direct-bucket config mode: read/write config and agent auth directly from
# mounted bucket path, while keeping lock-heavy sessions on local disk.
persist_only="${OPENCLAW_PERSIST_CONFIG_ONLY}"
if [[ -z "${OPENCLAW_STATE_DIR:-}" ]] && [[ "${persist_only}" == "1" ]] && [[ -n "${mount_target}" ]] && [[ -d "${mount_target}" ]] && [[ -w "${mount_target}" ]]; then
  export OPENCLAW_STATE_DIR="${OPENCLAW_LOCAL_STATE_DIR}"
  export OPENCLAW_CONFIG_PATH="${OPENCLAW_CONFIG_PATH:-${mount_target}/openclaw.json}"
  mkdir -p "${OPENCLAW_STATE_DIR}" "$(dirname "${OPENCLAW_CONFIG_PATH}")"

  # Keep agent config/auth on bucket mount; keep sessions local for robust locks.
  agent_id="${OPENCLAW_DEFAULT_AGENT_ID:-main}"
  local_agent_root="${OPENCLAW_STATE_DIR}/agents/${agent_id}"
  local_sessions_dir="${local_agent_root}/sessions"
  local_agent_link="${local_agent_root}/agent"
  bucket_agent_dir="${mount_target}/agents/${agent_id}/agent"
  mkdir -p "${local_sessions_dir}" "${bucket_agent_dir}"
  if [[ -L "${local_agent_link}" ]]; then
    rm -f "${local_agent_link}"
  elif [[ -e "${local_agent_link}" ]]; then
    rm -rf "${local_agent_link}"
  fi
  ln -s "${bucket_agent_dir}" "${local_agent_link}"
fi

if [[ -z "${OPENCLAW_STATE_DIR:-}" ]] && [[ -n "${mount_target}" ]] && [[ -d "${mount_target}" ]] && [[ -w "${mount_target}" ]]; then
  export OPENCLAW_STATE_DIR="${mount_target}"
fi

# Final fallback for setups without bucket mount.
if [[ -z "${OPENCLAW_STATE_DIR:-}" ]] && [[ -d "/mnt" ]] && [[ -w "/mnt" ]]; then
  export OPENCLAW_STATE_DIR="/mnt"
fi
if [[ -z "${OPENCLAW_CONFIG_PATH:-}" ]] && [[ -n "${OPENCLAW_STATE_DIR:-}" ]]; then
  export OPENCLAW_CONFIG_PATH="${OPENCLAW_STATE_DIR}/openclaw.json"
fi

# Ensure custom persisted state/config locations exist (e.g. MinIO mount on /mnt).
if [[ -n "${OPENCLAW_STATE_DIR:-}" ]]; then
  mkdir -p "${OPENCLAW_STATE_DIR}"
fi
if [[ -n "${OPENCLAW_CONFIG_PATH:-}" ]]; then
  mkdir -p "$(dirname "${OPENCLAW_CONFIG_PATH}")"
fi
echo "openclaw: state dir=${OPENCLAW_STATE_DIR:-<default>} config path=${OPENCLAW_CONFIG_PATH:-<default>}" >&2

# OpenClaw startup can exceed Node's default heap limit on constrained containers.
# Tune heap size explicitly unless the user already provided NODE_OPTIONS.
if [[ -z "${NODE_OPTIONS:-}" ]]; then
  export NODE_OPTIONS="--max-old-space-size=768"
fi

# Keep runtime footprint low for OSCAR local/small clusters while leaving
# channels/providers enabled unless explicitly disabled by the operator.
export OPENCLAW_SKIP_CHANNELS="${OPENCLAW_SKIP_CHANNELS:-0}"
export OPENCLAW_SKIP_PROVIDERS="${OPENCLAW_SKIP_PROVIDERS:-0}"
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
  if ! node /app/openclaw.mjs config set gateway.auth.mode "\"token\"" --json >/dev/null 2>&1; then
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
