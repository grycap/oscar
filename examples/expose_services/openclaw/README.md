# OpenClaw Gateway as an OSCAR Exposed Service (Authenticated)

This example deploys [OpenClaw](https://github.com/openclaw/openclaw) as an OSCAR exposed service and publishes the Gateway UI through the OSCAR exposed endpoint with OSCAR authentication enabled (`set_auth: true`).

## Why this example uses one container

In OpenClaw Docker Compose there are usually two services:

- `openclaw-gateway`: long-running runtime service (serves the web UI and API)
- `openclaw-cli`: helper container for one-shot operations (`onboard`, `dashboard`, `devices`, channel setup)

For OSCAR exposed services, only the long-running HTTP service is required to expose the browser UI, so this example runs only the Gateway container.

The CLI role is handled outside OSCAR (for example, during image preparation) or by later configuration updates.

## Prerequisites

- An OSCAR cluster with exposed services enabled.
- `oscar-cli` configured against your OSCAR endpoint.
- OpenClaw source code available locally at (optional, for custom images):
  - `/Users/gmolto/Documents/GitHub/openclaw/openclaw`

Resource note for local clusters:
- This example is tuned for constrained single-node local environments (`memory: 1Gi`, `cpu: 0.5`).
- If your cluster has more capacity, you can increase these values in `openclaw_expose.yaml`.
- To reduce runtime footprint, optional components are disabled by default in the example:
  - `OPENCLAW_SKIP_CHANNELS=1`
  - `OPENCLAW_SKIP_BROWSER_CONTROL_SERVER=1`
  - `OPENCLAW_SKIP_CANVAS_HOST=1`
  - `OPENCLAW_SKIP_GMAIL_WATCHER=1`
  - `OPENCLAW_SKIP_CRON=1`

## 1. Choose the OpenClaw image tag

This example uses:

```yaml
image: ghcr.io/openclaw/openclaw:2026.2.9
```

For production-like usage, prefer pinning by digest:

```yaml
image: ghcr.io/openclaw/openclaw:<tag>@sha256:<digest>
```

If you want a custom image from local source instead, build and push from:
`/Users/gmolto/Documents/GitHub/openclaw/openclaw`, then replace the `image:` value.

## 2. Deploy the OSCAR service

```bash
cd /Users/gmolto/Documents/GitHub/grycap/oscar/examples/expose_services/openclaw
oscar-cli apply openclaw_expose.yaml
```

This creates service `openclaw-gateway` and exposes:

```text
https://<OSCAR_ENDPOINT>/system/services/openclaw-gateway/exposed/
```

## 3. Get OSCAR service credentials

Because `set_auth: true` is enabled, OSCAR protects the exposed endpoint with basic authentication:

- Username: service name (`openclaw-gateway`)
- Password: service token

Get the token:

```bash
oscar-cli service get openclaw-gateway | grep token
```

## 4. Open the Gateway in your browser

Go to:

```text
https://<OSCAR_ENDPOINT>/system/services/openclaw-gateway/exposed/
```

When prompted by the browser for credentials, use:

- user: `openclaw-gateway`
- password: service token from previous step

After authentication, the OpenClaw Gateway Control UI should load without additional token prompts in the default example setup.

## 5. Quick auth checks

Unauthenticated request should be denied:

```bash
curl -i https://<OSCAR_ENDPOINT>/system/services/openclaw-gateway/exposed/
```

Authenticated request should return the UI:

```bash
curl -i -u "openclaw-gateway:<SERVICE_TOKEN>" \
  https://<OSCAR_ENDPOINT>/system/services/openclaw-gateway/exposed/
```

## Notes

- The startup script (`openclawscript.sh`) forces `--bind lan` so the gateway is reachable from OSCAR networking.
- The startup script always aligns OpenClaw gateway auth with OSCAR's current service token by reading `/oscar/config/function_config.yaml`, setting `gateway.auth.*`, and running gateway with `--token`.
- No token is stored in `openclaw_expose.yaml`; the token is read at runtime from OSCAR service config.
- The startup script sets `NODE_OPTIONS=--max-old-space-size=768` by default to avoid Node.js heap OOM on small local clusters.
- The startup script auto-configures `gateway.trustedProxies` to private proxy ranges so WebSocket clients behind OSCAR ingress are correctly identified. Override with `OPENCLAW_GATEWAY_TRUSTED_PROXIES` (JSON array string).
- Device pairing for the Control UI is disabled by default in this example (`OPENCLAW_DISABLE_DEVICE_AUTH=1`) to avoid manual approvals after each redeploy. This removes OpenClaw per-device identity checks.
- For MinIO-mounted deployments, the startup script auto-uses `/mnt` as `OPENCLAW_STATE_DIR` and `/mnt/openclaw.json` as `OPENCLAW_CONFIG_PATH` when available, so config/state are persisted even if custom FDL env vars are not propagated.
- `health_path: "/"` is set so OSCAR readiness/liveness probes check a valid OpenClaw HTTP endpoint.
- `rewrite_target: true` is enabled so the exposed service path is rewritten correctly when accessing the web UI behind OSCAR's `/system/services/<name>/exposed/` prefix.
