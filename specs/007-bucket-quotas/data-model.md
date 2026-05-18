# Data Model: Bucket Quotas

## Entity: QuotaResponse

- Description: Existing user quota response extended with MinIO bucket quota
  information.
- Existing fields: `user_id`, `cluster_queue`, `resources`, `volumes`.
- New fields:
  - `minio` (optional): `MinIOQuotaResponse`
- Relationships:
  - One `QuotaResponse` describes one OSCAR user.
  - `minio` is present when MinIO quota reporting is available.

Validation rules:
- Existing CPU, memory, and volume quota fields keep their current behavior.
- `minio` may be omitted when MinIO quota support is disabled or unavailable.

## Entity: MinIOQuotaResponse

- Description: User-facing view of MinIO bucket count and storage quota data.
- Fields:
  - `buckets`: `MinIOBucketCountQuota`
  - `storage_per_bucket`: `MinIOStoragePerBucketQuota`
  - `storage_total`: `MinIOStorageTotalUsage`
- Relationships:
  - Belongs to one `QuotaResponse`.

Validation rules:
- `storage_per_bucket.max` is the enforceable bucket storage setting.
- `storage_total.used` is informational and must not be presented as a strict
  aggregate cap.
- `buckets.max` and `storage_per_bucket.max` are read from the user's
  `MinIOQuotaConfig` when present. When absent, both are reported with default
  zero values.

## Entity: MinIOQuotaConfig

- Description: Kubernetes-backed per-user MinIO quota configuration managed by
  OSCAR.
- Backing resource:
  - Kubernetes ConfigMap named `oscar-minio-quota`
  - Namespace: the user namespace resolved by OSCAR for the target user
  - Labels:
    - `oscar.grycap.upv.es/quota: minio`
- Data fields:
  - `buckets` (string, optional): maximum number of OSCAR-controlled buckets
    the user may create.
  - `storage_per_bucket` (string, optional): default MinIO bucket quota applied
    to each OSCAR-managed bucket owned by the user.
- Relationships:
  - One `MinIOQuotaConfig` belongs to one OSCAR user namespace.
  - One `MinIOQuotaConfig` feeds one user's `MinIOQuotaResponse`.

Validation rules:
- `buckets` must parse as an integer greater than or equal to zero.
- `storage_per_bucket` must parse as a valid storage quantity accepted by OSCAR.
- Missing fields mean that specific quota dimension is unset.
- Missing ConfigMap means MinIO bucket quotas are not configured for the user,
  and existing behavior is preserved.

## Entity: MinIOBucketCountQuota

- Description: Per-user maximum number of OSCAR-managed MinIO buckets and the
  current number attributed to the user.
- Fields:
  - `max` (integer, optional): maximum number of buckets allowed through
    OSCAR-controlled creation paths.
  - `used` (integer): number of buckets currently attributed to the user.
- Relationships:
  - Computed from `OwnedBucket` records.

Validation rules:
- `max` must be zero or greater when provided.
- `max=0` in an explicit quota configuration means the user cannot create new
  OSCAR-managed buckets.
- `used` must be zero or greater.

## Entity: MinIOStoragePerBucketQuota

- Description: Configured per-bucket MinIO storage quota for a user's
  OSCAR-managed buckets.
- Fields:
  - `max` (string, optional): storage quantity such as `10Gi`, `100Gi`, or
    `500Mi`.
- Relationships:
  - Applied to each OSCAR-managed bucket where quota enforcement is enabled.

Validation rules:
- `max` must parse as a positive storage quantity or be omitted.

## Entity: MinIOStorageTotalUsage

- Description: Informational aggregate storage usage for buckets attributed to a
  user.
- Fields:
  - `used` (string): human-readable storage quantity.
- Relationships:
  - Computed from buckets whose ownership can be attributed to the user.

Validation rules:
- `used` must be zero or greater and formatted as a storage quantity.

## Entity: QuotaUpdateRequest

- Description: Existing administrator quota update request extended with MinIO
  quota settings.
- Existing fields: `cpu`, `memory`, `volumes`.
- New fields:
  - `minio` (optional): `MinIOQuotaUpdate`

Validation rules:
- Existing CPU, memory, and volume validations keep their current behavior.
- An update is valid when at least one of CPU, memory, volumes, or MinIO quota
  fields is provided.

## Entity: MinIOQuotaUpdate

- Description: Administrator-provided MinIO quota changes for a user.
- Fields:
  - `buckets` (string or integer, optional): maximum bucket count.
  - `storage_per_bucket` (string, optional): storage quantity applied as the
    default per-bucket storage quota for the user.
- Relationships:
  - Belongs to one `QuotaUpdateRequest`.
  - Updates one `MinIOQuotaConfig` ConfigMap in the target user's namespace.

Validation rules:
- `buckets` must parse as an integer greater than or equal to zero.
- `storage_per_bucket` must parse as a positive storage quantity or zero when
  zero-quota semantics are intentionally supported.
- Invalid units are rejected with clear messages.

## Entity: MinIOBucket

- Description: Existing bucket representation extended with quota metadata.
- Existing fields: `bucket_name`, `visibility`, `allowed_users`, `owner`,
  `metadata`, `objects`.
- New fields:
  - `storage_quota` (optional): `MinIOStoragePerBucketQuota`
  - `storage_usage` (optional): `MinIOBucketStorageUsage`
  - `attribution` (enum, optional): `oscar_managed`, `direct`, or `unknown`

Validation rules:
- Existing bucket visibility and ownership semantics are preserved.
- `storage_quota` is present when quota metadata can be read from MinIO.

## Entity: MinIOBucketStorageUsage

- Description: Storage usage for one bucket when available.
- Fields:
  - `used` (string)
  - `used_bytes` (integer)
  - `objects` (integer, optional)

Validation rules:
- `used_bytes` and `objects` must be zero or greater.

## State and Behavior Notes

### MinIO quota settings update

1. Resolve the target user's namespace.
2. Validate `minio.buckets` and `minio.storage_per_bucket`.
3. Create or update the `oscar-minio-quota` ConfigMap in that namespace.
4. Return a refreshed quota response based on the ConfigMap and current MinIO
   usage.

### OSCAR-controlled bucket creation

1. Resolve requester UID and intended owner.
2. Read `oscar-minio-quota` from the owner's user namespace, when present.
3. Count buckets attributed to that owner.
4. Reject creation if `buckets.max` is configured and
   `buckets.used >= buckets.max`.
5. Create the bucket.
6. Tag ownership metadata.
7. Apply visibility policies.
8. Apply MinIO `storage_per_bucket` quota when configured.

If steps after bucket creation fail, implementation should avoid leaving
partially configured resources where feasible and report explicit errors.

### Direct MinIO bucket creation

1. User creates a bucket with AK/SK or the MinIO console.
2. OSCAR does not pre-validate the bucket count.
3. OSCAR may detect the bucket later only if MinIO listing/metadata allows it.
4. If owner metadata is missing, OSCAR cannot safely attribute the bucket to a
   user quota without an additional ownership strategy.
