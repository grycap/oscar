# OpenClaw Gateway as OSCAR Exposed Service (Volume Persistence)

This example is adapted from branch `003-openclaw` to use OSCAR managed-volume persistence instead of MinIO mount persistence.

## Files

- `openclaw_expose_workspace.yaml`: volume-based OpenClaw deployment.
- `openclaw_expose.yaml`: baseline exposed OpenClaw deployment from `003-openclaw`.
- `openclawscript.sh`: OpenClaw startup script from `003-openclaw`.

## Deploy volume variant

```bash
cd /Users/gmolto/Documents/GitHub/grycap/oscar/examples/expose_services/openclaw
oscar-cli apply openclaw_expose_workspace.yaml
```

This deploys `openclaw-workspace` with:

- `volume.size: 1Gi`
- `volume.mount_path: /data`
- OpenClaw state/config persisted in:
  - `/data/openclaw-state`
  - `/data/openclaw-state/openclaw.json`

## Access exposed UI

```text
https://<OSCAR_ENDPOINT>/system/services/openclaw-workspace/exposed/
```

Because `set_auth: true`, use:

- user: `openclaw-workspace`
- password: service token

Get token:

```bash
oscar-cli service get openclaw-workspace | grep token
```

## Persistence check

1. Open OpenClaw UI and change a gateway setting.
2. Redeploy same service definition or restart pod.
3. Verify the setting remains (state persisted in the volume).
