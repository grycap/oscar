# Quickstart: Managed Volumes for OSCAR Services

## 1. Create a volume directly through `/system/volumes`

Representative request body:

```json
{
  "name": "shared-data",
  "size": "10Gi"
}
```

Create via API:

```bash
curl -X POST "$OSCAR_URL/system/volumes" \
  -H "Content-Type: application/json" \
  -u "$OSCAR_USER:$OSCAR_PASS" \
  -d '{"name":"shared-data","size":"10Gi"}'
```

Expected result:
- HTTP `201`.
- The returned volume belongs to the caller namespace.
- `GET /system/volumes` includes `shared-data` only for that same caller namespace.

## 2. Deploy a service that creates its own volume with an auto-generated name

Representative request body:

```json
{
  "name": "trainer",
  "image": "grycap/demo:latest",
  "script": "run.sh",
  "volume": {
    "size": "10Gi",
    "mount_path": "/data",
    "lifecycle_policy": "retain"
  }
}
```

Expected result:
- HTTP `201`.
- The service starts with a managed volume mounted at `/data`.
- The resolved volume name is derived from `trainer` and exposed in service read output.

## 3. Deploy a service that creates its own volume with an explicit name override

Representative request body:

```json
{
  "name": "openclaw-volume",
  "image": "ghcr.io/openclaw/openclaw:latest",
  "script": "echo start",
  "volume": {
    "name": "openclaw-data",
    "size": "20Gi",
    "mount_path": "/home/node/.openclaw",
    "lifecycle_policy": "delete"
  }
}
```

Expected result:
- HTTP `201`.
- The service mounts `openclaw-data`.
- `GET /system/volumes/openclaw-data` shows `creation_mode=service`.

## 4. Deploy a second service that mounts an existing named volume

Representative request body:

```json
{
  "name": "volume-files",
  "image": "filebrowser/filebrowser:s6",
  "script": "echo start",
  "volume": {
    "name": "openclaw-data",
    "mount_path": "/data"
  }
}
```

Expected result:
- HTTP `201`.
- No new volume is created.
- The service mounts the existing `openclaw-data` volume in the same namespace.

## 5. Verify persistence across restart and redeploy

1. Write a sentinel file into the mounted path.
2. Restart or redeploy the service without changing its `volume` block.
3. Verify the sentinel file still exists.

Expected result:
- The file remains available after restart and redeploy.

## 6. Verify lifecycle policy behavior

### Retain

1. Deploy a service that creates a volume with `lifecycle_policy: retain`.
2. Delete the service.
3. Read or list `/system/volumes`.
4. Deploy another service that mounts the retained volume by name.

Expected result:
- The retained volume still exists after service deletion.
- Another service in the same namespace can mount it.

### Delete

1. Deploy a service that creates a volume with `lifecycle_policy: delete`.
2. Delete the service.
3. Read or list `/system/volumes`.

Expected result:
- The service-created volume is removed along with its backing storage.

## 7. Validate namespace isolation

Use two distinct authenticated users:

1. User A creates `shared-data`.
2. User B calls `GET /system/volumes`.
3. User B deploys a service that references `shared-data` by name.

Expected result:
- User B does not see User A's volume in list/read results.
- User B's service deployment is rejected when referencing `shared-data`.

## 8. Validate invalid payload handling

Use invalid requests such as:
- Service `volume` missing `mount_path`
- Service `volume` missing both `size` and `name`
- Service `volume` setting `lifecycle_policy` without `size`
- Invalid volume name format
- Duplicate volume name in the same namespace
- Deleting an attached volume through `/system/volumes/{name}`

Expected result:
- HTTP `400` or `409` with clear validation errors.

## 9. Validate backward compatibility

Create or update a service without the `volume` field.

Expected result:
- Behavior matches current legacy service deployments.
- Existing `mount`, `input`, `output`, and exposed-service flows remain unchanged.

## 10. Test commands for implementation phase

Run targeted tests for touched packages:

```bash
go test ./pkg/types ./pkg/handlers ./pkg/backends/...
```

Regenerate Swagger/OpenAPI docs:

```bash
go generate ./...
```

Validate docs rendering when feasible:

```bash
mkdocs serve
```
