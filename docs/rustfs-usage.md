# Using the RustFS Storage Provider

[RustFS](https://rustfs.com) is an S3-compatible object storage system that OSCAR
can use instead of MinIO as its storage provider. As with MinIO, the storage
provider is responsible for triggering services: when a service is configured to
watch an input folder, any new data added to that folder fires an event that
starts a job of the associated service.

This document describes how OSCAR integrates with RustFS, how to configure it,
and what differs from the [MinIO storage provider](minio-usage.md).

## What is RustFS?

RustFS is an object storage system that is compatible with the S3 API for
standard bucket and object operations. OSCAR treats RustFS as a drop-in replacement for MinIO: the same policy creation,
assignment, update and removal flow is used, but routed through the RustFS-native
IAM API.

## Choosing RustFS: configuration

The storage backend is selected at OSCAR startup with the
`OBJECT_STORAGE_TYPE` environment variable. It accepts two values:

| Value     | Meaning                                  |
|-----------|------------------------------------------|
| `minio`   | Use MinIO (default).                     |
| `rustfs`  | Use RustFS.                              |

```bash
OBJECT_STORAGE_TYPE=rustfs
```

When deploying OSCAR with Helm, the same setting is forwarded through the
`minIO.object_storage_type` chart value. The rest of the object storage settings
(endpoint, credentials, TLS verification) keep the `minIO.*` prefix and behave
exactly as they do for MinIO:

```bash
helm install --namespace=oscar oscar grycap/oscar \
  --set minIO.object_storage_type=rustfs \
  --set minIO.endpoint=http://rustfs-svc.rustfs:9000 \
  --set minIO.TLSVerify=false \
  --set minIO.accessKey=rustfs --set minIO.secretKey=<RUSTFS_PASSWORD>
```

Because RustFS supports the S3 API, the `minIO.*` endpoint and credential
settings point to the RustFS S3 endpoint, and the OSCAR web interface
(including the buckets and files sections) works in the same way as with MinIO.

## How OSCAR interacts with RustFS

OSCAR distinguishes the backend at runtime through the configured
`OBJECT_STORAGE_TYPE` value. All IAM and notification operations are then
performed against the RustFS native admin API at `/rustfs/admin/v3`, with every
request signed using **AWS Signature Version 4** (service `s3`).

The main routes used by OSCAR are:

| Route                                   | Method | Purpose                                            |
|-----------------------------------------|--------|----------------------------------------------------|
| `/add-user`                             | PUT    | Create an OSCAR user.                              |
| `/update-group-members`                 | PUT    | Manage group membership (RustFS JSON format).      |
| `/add-canned-policy`                    | PUT    | Create a policy document.                          |
| `/set-user-or-group-policy`             | PUT    | Attach a policy to a user or group.                |
| `/target/notify_webhook/<service>`      | PUT    | Register a per-service webhook target.             |
| `/target/notify_webhook/<service>/reset`| DELETE | Remove a per-service webhook target.               |
| `/module-switches`                      | GET/PUT| Read and enable the notification module.           |

## Bucket management and visibility

### Metadata stored as bucket tags

RustFS uses group-based policies to model bucket visibility. Also, RustFS and OSCAR use metadata to store information:

| Tag               | Meaning                                         |
|-------------------|-------------------------------------------------|
| `owner`           | User ID of the bucket owner.                    |
| `owner_name`      | Username of the bucket owner.                   |
| `visibility`      | `public`, `restricted` or `private`.            |
| `allowed_users`   | Space-separated list of users allowed to access a `restricted` bucket. |
| `storage_quota`   | Per-bucket storage limit (when a quota is set). |


## Bucket quotas

Per-bucket storage limits configured through the `/system/quotas` API
(`storage_per_bucket`) are enforced with the MinIO-compatible bucket quota admin
API (`set-bucket-quota`), which RustFS implements.

## Deploying RustFS

### Local deployment

The local deployment script installs RustFS automatically when it is selected as
the storage backend:

```bash
bash oscar/deploy/kind-deploy.sh --storage=rustfs
```

This:

- Installs the `rustfs/rustfs` Helm chart in the `rustfs` namespace, using the
  standalone mode (no erasure-coded distributed layout).
- Creates the `rustfs-svc` service in the `rustfs` namespace; OSCAR is pointed at
  `http://rustfs-svc.rustfs:<port>`.
- Exposes the S3 endpoint and the RustFS console on the same node ports that MinIO
  uses (`30300` and `30301` by default).
- Sets `RUSTFS_OUTBOUND_ALLOW_ORIGINS=http://oscar.oscar:8080` so the OSCAR job
  endpoint is a valid webhook target.
- Configures OSCAR with `OBJECT_STORAGE_TYPE=rustfs` through the
  `minIO.object_storage_type` chart value.

The `--minio-quotas` option is **not compatible** with RustFS: it prepares an
erasure-coded distributed MinIO layout (one replica, four drives) which is a
MinIO-only prerequisite. Use it only when MinIO is selected.

## Accessing RustFS buckets

Because RustFS is S3-compatible, all the access methods documented for MinIO
apply to RustFS as well:

- **OSCAR Dashboard**: the buckets section lists the buckets visible to the
  authenticated user and allows uploading and downloading files, exactly as with
  MinIO.
- **oscar-cli**: the `get-file`, `list-files` and `put-file` commands operate on
  the configured storage provider regardless of the backend.
- **Any S3 client**: standard S3 tools (for example `mc`) can be pointed at the
  RustFS S3 endpoint with the user credentials to manage files inside buckets.
