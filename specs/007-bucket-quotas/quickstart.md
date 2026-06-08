# Quickstart: Bucket Quotas

## 1. Configure MinIO quotas for a user

Representative administrator request:

```json
{
  "minio": {
    "buckets": "10",
    "storage_per_bucket": "100Gi"
  }
}
```

Update through the quota API:

```bash
curl -X PUT "$OSCAR_URL/system/quotas/user/$USER_ID" \
  -H "Content-Type: application/json" \
  -u "$OSCAR_USER:$OSCAR_PASS" \
  -d '{"minio":{"buckets":"10","storage_per_bucket":"100Gi"}}'
```

Expected result:
- HTTP `200`.
- The response includes `minio.buckets.max=10`.
- The response includes `minio.storage_per_bucket.max=100Gi`.
- OSCAR creates or updates a ConfigMap named `oscar-minio-quota` in the target
  user's namespace with `data.buckets=10` and
  `data.storage_per_bucket=100Gi`.

Representative ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: oscar-minio-quota
  namespace: <user-namespace>
  labels:
    oscar.grycap.upv.es/quota: minio
data:
  buckets: "10"
  storage_per_bucket: "100Gi"
```

## 2. Read own MinIO quota usage

```bash
curl "$OSCAR_URL/system/quotas/user" \
  -H "Authorization: Bearer $TOKEN"
```

Expected representative response fragment:

```json
{
  "user_id": "user@example.org",
  "minio": {
    "buckets": {
      "max": 10,
      "used": 3
    },
    "storage_per_bucket": {
      "max": "100Gi"
    },
    "storage_total": {
      "used": "42Gi"
    }
  }
}
```

## 3. Create a bucket under the bucket count limit

```bash
curl -X POST "$OSCAR_URL/system/buckets" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"bucket_name":"quota-demo","visibility":"private"}'
```

Expected result:
- HTTP `201`.
- The bucket is tagged with OSCAR owner metadata.
- The bucket receives the configured MinIO `storage_per_bucket` quota when the
  setting is available.

## 4. Reject an OSCAR-controlled bucket over the count limit

1. Configure `minio.buckets` to a small value, such as `1`.
2. Create one bucket through OSCAR.
3. Try to create a second bucket through OSCAR.

Expected result:
- The second request is rejected before OSCAR creates a MinIO bucket.
- The error message states that the user bucket count quota has been reached.

## 5. Validate per-bucket storage quota behavior

1. Configure `storage_per_bucket` for the user.
2. Create a bucket through OSCAR.
3. Upload objects to the bucket until the bucket reaches the configured MinIO
   quota as recognized by MinIO.
4. Attempt another object write.

Expected result:
- MinIO rejects additional writes according to native bucket quota behavior.
- OSCAR documentation and responses do not claim exact immediate byte-level
  enforcement.

## 6. Verify direct MinIO bucket creation limitation

Create a bucket directly with user AK/SK or the MinIO console.

Expected result:
- OSCAR did not pre-check the bucket count before creation.
- Later quota responses may report the bucket only if ownership can be
  identified.
- Documentation/API descriptions state that direct MinIO bucket creation
  bypasses OSCAR pre-creation checks.

## 7. Validate invalid quota payloads

Use invalid requests such as:
- Negative bucket count
- Non-numeric bucket count
- Invalid `storage_per_bucket` unit
- Malformed MinIO quota object

Expected result:
- HTTP `400` with clear validation errors.
- Existing CPU, memory, and volume quota update behavior remains unchanged.

## 8. Test commands for implementation phase

Run targeted tests for touched packages:

```bash
CGO_ENABLED=0 go test ./pkg/types ./pkg/utils ./pkg/handlers ./pkg/handlers/buckets
```

Run broader tests when feasible:

```bash
CGO_ENABLED=0 go test ./...
```

Latest implementation validation:

- `CGO_ENABLED=0 go test ./pkg/types ./pkg/utils ./pkg/handlers ./pkg/handlers/buckets`
- `CGO_ENABLED=0 go test ./...`
- `go generate ./...` was executed successfully. The generated `pkg/apidocs`
  directory is ignored by the repository, so local generated artifacts were not
  kept in the working tree.

Regenerate Swagger/OpenAPI docs if API comments or generated docs are updated:

```bash
go generate ./...
```

Validate docs rendering when feasible:

```bash
mkdocs serve
```
