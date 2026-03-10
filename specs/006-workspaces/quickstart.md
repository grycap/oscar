# Quickstart: Persistent Workspaces for OSCAR Services

## 1. Create a service with workspace enabled

Example request body (representative):

```json
{
  "name": "workspace-demo",
  "image": "grycap/demo:latest",
  "script": "run.sh",
  "workspace": {
    "size": "10Gi",
    "mount_path": "/data"
  }
}
```

Create via API:

```bash
curl -X POST "$OSCAR_URL/system/services" \
  -H "Content-Type: application/json" \
  -u "$OSCAR_USER:$OSCAR_PASS" \
  -d @service-workspace.json
```

Expected result:
- HTTP `201`.
- Service is listed with workspace configuration.

## 2. Verify workspace persistence across restart/redeploy

1. Invoke service to write a sentinel file under `mount_path`.
2. Restart/redeploy service without changing workspace config.
3. Verify sentinel file still exists.

Expected result:
- Workspace data remains available after restart/redeploy.

## 3. Validate backward compatibility

Create or update a service without `workspace` field.

Expected result:
- Behavior matches current non-workspace services.
- No new required fields for legacy payloads.

## 4. Validate invalid workspace payload handling

Use invalid requests such as:
- Missing `size`
- Missing `mount_path`
- Invalid size format
- Non-absolute `mount_path`

Expected result:
- HTTP `400` with clear validation error.

## 5. Validate immutable workspace update policy

Attempt to update an existing service changing `workspace.size` or `workspace.mount_path`.

Expected result:
- HTTP `400` (or current validation error status) indicating workspace mutation is not allowed in this phase.

## 6. Validate delete lifecycle

Delete service with workspace.

Expected result:
- Service deleted successfully.
- Workspace resources removed under default lifecycle.

## 7. Test commands for implementation phase

Run targeted tests for touched packages:

```bash
go test ./pkg/types ./pkg/handlers ./pkg/backends/...
```

If docs are updated, validate docs rendering when feasible:

```bash
mkdocs serve
```

## 8. Implementation Validation Notes (2026-03-09)

- Targeted tests executed successfully:

```bash
go test ./pkg/types ./pkg/handlers ./pkg/backends/...
```

- API docs regeneration completed successfully:

```bash
go generate ./...
```
