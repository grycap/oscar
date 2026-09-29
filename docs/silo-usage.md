# Using Silo as a MinIO Alternative

[Silo](https://silo.pgsty.com) is a community-maintained fork of the
open-source MinIO server, published by Pigsty as the `pgsty/silo` container
image. It keeps a single MinIO-compatible release line alive after upstream
MinIO ended community distribution, shipping binaries, packages,
multi-architecture images, security fixes and the web console.

Silo is an independent project. It is not affiliated with, endorsed by or
sponsored by MinIO, Inc. "MinIO" is only used to identify the upstream project
and the compatibility lineage.

## Using Silo instead of MinIO

Silo is a drop-in replacement for MinIO: it preserves the S3 API, the `MINIO_*`
environment variables, the `minio_*` metrics, the `x-minio-*` headers, the
`/minio/*` routes and the on-disk format. For OSCAR this means that the change
is limited to the container image. Edit the MinIO deployment and change the
image name to `pgsty/silo:RELEASE.2026-09-16T00-00-00Z`, keeping the same root
credentials and the same persistent volumes, which are reused in place. No
OSCAR configuration change is needed: leave `OBJECT_STORAGE_TYPE=minio` and the
`minIO.*` settings (endpoint, credentials, TLS verification) untouched.

The recommended image is `pgsty/silo:RELEASE.2026-09-16T00-00-00Z`. Pin the
release instead of using `latest` so upgrades are explicit and reversible.

## Switching the image

### Deploying Silo with Helm

The `minio/minio` chart takes the image from the `image.repository` and
`image.tag` values, so Silo can be installed directly by overriding them:

```bash
helm install minio minio/minio --namespace minio --set rootUser=minio,\
rootPassword=<MINIO_PASSWORD>,service.type=NodePort,service.nodePort=30300,\
consoleService.type=NodePort,consoleService.nodePort=30301,mode=standalone,\
resources.requests.memory=512Mi,\
environment.MINIO_BROWSER_REDIRECT_URL=http://localhost:30301,\
image.repository=pgsty/silo,image.tag=RELEASE.2026-09-16T00-00-00Z \
 --create-namespace
```

### Migrating an existing MinIO deployment

Point the existing Helm release to the Silo image:

```bash
helm upgrade minio minio/minio --namespace minio \
  --set image.repository=pgsty/silo \
  --set image.tag=RELEASE.2026-09-16T00-00-00Z
```

### Editing the deployment directly

The same change can be applied with `kubectl`, which replaces the image in the
running deployment and triggers a rollout:

```bash
kubectl -n minio set image deployment/minio \
  minio=pgsty/silo:RELEASE.2026-09-16T00-00-00Z
```

The container name in the `minio/minio` chart is `minio`, and the deployment
name matches the Helm release name. If either was customized, check it first:

```bash
kubectl -n minio get deployments
kubectl -n minio get deployment minio \
  -o jsonpath='{.spec.template.spec.containers[*].name}'
```

## Verifying the change

After the rollout, check that the server is up and that the admin API used by
OSCAR is reachable:

```bash
kubectl -n minio get pods -l app=minio
mc alias set silo http://localhost:30300 minio <MINIO_PASSWORD>
mc admin info silo
```

Then confirm that the buckets listed in the OSCAR dashboard are still visible
and that a new upload triggers the associated service, as described in the
[MinIO storage provider](minio-usage.md).

## Compatibility notes

- Bucket policies, bucket tags, bucket quotas and the notification webhooks
  used by OSCAR go through the MinIO admin API, which Silo keeps, so
  `storage_per_bucket` quotas and the erasure-coded layout provided by the
  `--minio-quotas` option keep working.
- The upstream [MinIO client](https://min.io/docs/minio/linux/reference/minio-mc.html)
  works unchanged against Silo. An optional client fork is also published as
  `pgsty/mc`, shipped as `mcli`.
- Do not delete the persistent volumes when replacing MinIO, as the data is
  reused in place.
- Read the [release notes](https://silo.pgsty.com) and the compatibility notes
  before each upgrade, and keep a rollback path by pointing the deployment back
  to the previous image.

## See also

- [Using the MinIO storage provider](minio-usage.md)
- [Using the RustFS storage provider](rustfs-usage.md), which requires setting
  `OBJECT_STORAGE_TYPE=rustfs` instead of only changing the image.
