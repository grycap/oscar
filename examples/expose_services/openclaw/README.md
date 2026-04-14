# OpenClaw Gateway as OSCAR Exposed Service (Volume Persistence)

This example is adapted from branch `003-openclaw` to use OSCAR managed-volume persistence instead of MinIO mount persistence.

## Files

- `openclaw_expose_workspace.yaml`: volume-based OpenClaw deployment.
- `openclaw_expose.yaml`: baseline exposed OpenClaw deployment from `003-openclaw`.
- `openclawscript.sh`: simplified OpenClaw startup script for volume-backed persistence.

## Deploy volume variant

```bash
cd /Users/gmolto/Documents/GitHub/grycap/oscar/examples/expose_services/openclaw
oscar-cli apply openclaw_expose_workspace.yaml
```

This deploys `openclaw-volume` with:

- `volume.size: 1Gi`
- `volume.mount_path: /data`
- image `ghcr.io/openclaw/openclaw:2026.3.8`
- CPU `2.0`
- OpenClaw state/config persisted in:
  - `/data/openclaw-state`
  - `/data/openclaw-state/openclaw.json`

The startup script now assumes:

- persistence is handled by the managed volume mounted at `/data`
- FDL environment variables are propagated correctly
- only the OSCAR service token may need to be read from `/oscar/config/function_config.yaml`

## Access exposed UI

```text
https://<OSCAR_ENDPOINT>/system/services/openclaw-volume/exposed/
```

Because `set_auth: true`, use:

- user: `openclaw-volume`
- password: service token

Get token:

```bash
oscar-cli service get openclaw-volume | grep token
```

## Persistence check

1. Open OpenClaw UI and change a gateway setting.
2. Redeploy same service definition or restart pod.
3. Verify the setting remains (state persisted in the volume).
