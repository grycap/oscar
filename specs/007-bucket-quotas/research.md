# Research: Bucket Quotas

## Decision 1: Use MinIO native bucket quota for storage enforcement

- **Decision**: Implement `storage_per_bucket` by calling MinIO admin bucket
  quota APIs through the existing `madmin-go` dependency.
- **Rationale**: The repository already creates a `madmin.AdminClient` in
  `pkg/utils/minio.go`, and the installed `madmin-go` version exposes
  `SetBucketQuota` and `GetBucketQuota`. This avoids new dependencies and keeps
  enforcement in MinIO, where writes actually occur.
- **Alternatives considered**:
  - Proxy all object writes through OSCAR: rejected because it would be a large
    architecture change and would not cover direct MinIO clients without forcing
    all clients through the proxy.
  - Implement only reporting without MinIO quota configuration: rejected because
    it does not enforce storage growth for buckets.

## Decision 2: Name the enforceable storage setting `storage_per_bucket`

- **Decision**: Expose the enforceable storage quota as
  `minio.storage_per_bucket`.
- **Rationale**: MinIO enforces quotas per bucket, not as an aggregate
  per-user cap across many buckets. The name avoids implying a stronger
  guarantee than the platform can provide.
- **Alternatives considered**:
  - `storage`: rejected because it reads like a total per-user cap.
  - `storage_total.max`: rejected for this phase because there is no native
    MinIO total-user storage quota to back it.

## Decision 3: Report aggregate MinIO usage separately

- **Decision**: Report aggregate user storage usage as informational data based
  on OSCAR-attributed buckets, separate from the enforceable
  `storage_per_bucket` limit.
- **Rationale**: Users and administrators still need to understand total MinIO
  usage. Reporting it separately keeps the distinction between visibility and
  enforcement explicit.
- **Alternatives considered**:
  - Hide aggregate usage: rejected because the feature goal includes limiting
    and understanding user storage consumption.
  - Treat aggregate usage as a hard limit: rejected because direct MinIO writes
    and buckets created outside OSCAR prevent OSCAR from guaranteeing a strict
    aggregate cap.

## Decision 4: Enforce bucket count only on OSCAR-controlled creation paths

- **Decision**: Check the per-user bucket count before OSCAR creates a bucket
  through `/system/buckets` or service-driven bucket creation paths.
- **Rationale**: OSCAR can prevent only the operations it controls. Users may
  create buckets directly with AK/SK or the MinIO console, bypassing OSCAR's
  pre-check.
- **Alternatives considered**:
  - Claim global bucket-count enforcement: rejected because direct MinIO bucket
    creation cannot be prevented by OSCAR under the accepted assumption.
  - Remove bucket count quotas entirely: rejected because the OSCAR-controlled
    path is still important and should be governed.

## Decision 5: Use existing owner tags for attribution

- **Decision**: Count and report user bucket quota usage using existing MinIO
  bucket owner metadata, especially the `owner` tag.
- **Rationale**: OSCAR already tags buckets at creation time and uses owner tags
  for visibility/access decisions. Reusing this metadata keeps changes minimal.
- **Alternatives considered**:
  - Add a separate database of bucket ownership: rejected because the repository
    currently avoids a separate persistence layer for MinIO bucket ownership.
  - Infer ownership from MinIO policy alone: rejected because policy membership
    can reflect sharing and visibility rather than ownership.

## Decision 6: Treat unattributed direct buckets conservatively

- **Decision**: Buckets without reliable OSCAR owner metadata are not counted as
  owned by a specific user unless an ownership strategy is added later. They
  should be surfaced as outside OSCAR attribution when detectable.
- **Rationale**: Assigning untagged buckets to a user would risk incorrect quota
  enforcement and confusing administrative output.
- **Alternatives considered**:
  - Count all unattributed buckets against every user: rejected because it would
    block unrelated users.
  - Ignore the limitation silently: rejected because it hides a major
    enforcement boundary.

## Decision 7: Keep quota configuration optional

- **Decision**: Preserve current behavior when MinIO bucket quotas are not
  configured or the feature is disabled/omitted.
- **Rationale**: The repository requires preserving existing behavior unless a
  change is explicitly requested. Operators should be able to roll out this
  feature incrementally.
- **Alternatives considered**:
  - Require quotas on all deployments: rejected because it would alter existing
    deployments and may break installations that do not want quota enforcement.

## Decision 8: Store per-user MinIO quota settings in Kubernetes ConfigMaps

- **Decision**: Store per-user MinIO quota settings in an OSCAR-managed
  ConfigMap named `oscar-minio-quota` in the user's Kubernetes namespace. Use
  `data.buckets` for the OSCAR-controlled bucket count limit and
  `data.storage_per_bucket` for the MinIO per-bucket storage quota.
- **Rationale**: This is compatible with the current `/system/quotas` approach:
  quota handlers already receive a Kubernetes client through `QuotaBackend`,
  resolve user namespaces, and manage Kubernetes-backed quota state for volume
  quotas. A ConfigMap avoids new dependencies, keeps operator-visible state in
  the cluster, and supports admin updates without redeploying OSCAR.
- **Alternatives considered**:
  - Store limits only in OSCAR process configuration: rejected because per-user
    updates through `/system/quotas/user/{userId}` would require redeployment or
    external config mutation.
  - Store limits only in MinIO bucket metadata: rejected because the bucket
    count limit is per-user state, not per-bucket state.
  - Use Kueue resources: rejected because Kueue is scoped to CPU/memory
    scheduling and does not model MinIO bucket governance.
  - Add a database or new quota service: rejected because the repository
    forbids new dependencies without approval and Kubernetes already provides a
    suitable persistence mechanism.
