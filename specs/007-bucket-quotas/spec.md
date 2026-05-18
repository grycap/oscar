# Feature Specification: Bucket Quotas

**Feature Branch**: `007-bucket-quotas`  
**Created**: 2026-05-15  
**Status**: Draft  
**Input**: User description: "Introduce bucket quota support so OSCAR can limit the maximum number of MinIO buckets per user and define/enforce per-bucket storage quotas as part of a broader goal to limit MinIO storage consumption per user."

## Clarifications

### Session 2026-05-15

- Q: Should this feature require a distributed MinIO deployment? → A: No; bucket count quotas are enforced by OSCAR and do not depend on MinIO distributed mode.
- Q: Is this feature about limiting bytes stored in MinIO buckets? → A: Yes; it must support per-bucket storage quotas using MinIO bucket quota capabilities, while also limiting the number of buckets a user can own.
- Q: Does MinIO provide a native maximum-buckets-per-user quota? → A: No; OSCAR can enforce this only for bucket creation requests that pass through OSCAR.
- Q: Does MinIO provide a native aggregate storage quota per user across many buckets? → A: No; OSCAR must expose the enforceable storage setting as `storage_per_bucket` and may expose aggregate storage usage as reporting-only data.
- Q: Should the quota field be named `storage` or `storage_per_bucket`? → A: Use `storage_per_bucket` for the enforceable MinIO bucket quota, and reserve aggregate storage usage for reporting.
- Q: What ownership signal should be used for counting user buckets? → A: Existing OSCAR-managed MinIO bucket metadata/tags, especially the `owner` tag set to the authenticated user UID.
- Q: Can OSCAR prevent users from creating buckets directly with their MinIO AK/SK or through the MinIO console? → A: No; assume direct MinIO bucket creation may happen and cannot be prevented by OSCAR.
- Q: Does per-bucket quota support require MinIO distributed mode? → A: No; MinIO bucket quotas are available without requiring a distributed MinIO deployment.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Enforce Bucket Count Limit (Priority: P1)

As an OSCAR administrator, I want OSCAR to prevent a user from creating more
than their allowed number of MinIO buckets through OSCAR-managed bucket creation
flows so the platform-controlled path applies bucket governance consistently.

**Why this priority**: This is the core quota behavior available to OSCAR.
                       Without rejecting bucket creation at the OSCAR API
                       boundary, the platform-controlled path continues
                       creating buckets without a per-user cap.

**Independent Test**: Configure a maximum bucket count for a user, create
                      buckets through `/system/buckets` until the limit is
                      reached, and verify that the next creation request is
                      rejected without creating a MinIO bucket.

**Acceptance Scenarios**:

1. **Given** a user who owns fewer buckets than their configured maximum,
   **When** the user creates a new bucket through OSCAR, **Then** the bucket is
   created and counted against that user's bucket quota.
2. **Given** a user who already owns exactly their configured maximum number of
   buckets, **When** the user tries to create another bucket through OSCAR,
   **Then** the request is rejected with a clear quota-exceeded response and no
   bucket is created in MinIO.
3. **Given** a rejected bucket creation request, **When** the user lists their
   buckets afterwards, **Then** the bucket count remains unchanged.
4. **Given** a user creates a bucket directly in MinIO using their AK/SK or the
   MinIO console, **When** OSCAR later computes bucket usage, **Then** OSCAR may
   report the bucket if ownership can be identified but cannot claim it
   prevented the direct creation.

---

### User Story 2 - Enforce Per-Bucket Storage Quota (Priority: P1)

As an OSCAR administrator, I want to assign a maximum storage size to each
OSCAR-managed MinIO bucket so a user's buckets cannot grow without a configured
storage ceiling.

**Why this priority**: Limiting bucket count alone does not limit MinIO storage
                       consumption. Per-bucket storage quotas are the native
                       MinIO mechanism that can bound object storage growth.

**Independent Test**: Configure a storage quota for a bucket, upload objects
                      through supported MinIO/OSCAR flows until the configured
                      quota is reached, and verify that further writes are
                      rejected according to MinIO quota behavior.

**Acceptance Scenarios**:

1. **Given** a user creates a bucket with an applicable storage quota, **When**
   OSCAR creates the bucket, **Then** the bucket receives the configured
   per-bucket storage quota in MinIO.
2. **Given** a bucket whose stored objects are below its configured storage
   quota, **When** the user writes additional objects that keep usage within
   the quota, **Then** the write is allowed if all other access rules pass.
3. **Given** a bucket whose stored objects have reached or exceeded the
   configured storage quota as recognized by MinIO, **When** the user attempts
   additional writes, **Then** MinIO rejects the writes until usage is reduced
   below the quota.
4. **Given** an administrator updates the storage quota for an existing bucket,
   **When** the bucket quota is read afterwards, **Then** the new configured
   quota is reported.

---

### User Story 3 - Inspect Bucket Quota Usage (Priority: P1)

As an authenticated OSCAR user, I want to see my current bucket usage,
per-bucket storage limit, and aggregate storage usage so I understand why bucket
creation or object writes may be blocked and how much MinIO storage I am using.

**Why this priority**: Users need visibility before and after quota enforcement;
                       otherwise a rejection is surprising and hard to act on.

**Independent Test**: Request the authenticated user's quota information and
                      verify that the response reports the configured bucket
                      maximum, `storage_per_bucket` limit, and current usage
                      for that user.

**Acceptance Scenarios**:

1. **Given** an authenticated user with a bucket count limit, **When** the user
   requests their own quota information, **Then** the response includes the
   maximum bucket count and current used bucket count.
2. **Given** an authenticated user with MinIO bucket storage quotas, **When**
   the user requests quota information, **Then** the response includes current
   aggregate storage usage and the configured `storage_per_bucket` limit for
   the user's owned buckets.
3. **Given** an administrator requesting quota information for a specific user,
   **When** the request is authorized, **Then** the response includes that
   user's bucket count maximum, current bucket count, `storage_per_bucket`
   maximum, and aggregate storage usage.
4. **Given** an authenticated user with no owned buckets, **When** the user
   requests quota information, **Then** the used bucket count and storage usage
   are reported as zero.

---

### User Story 4 - Update User Bucket Limits (Priority: P2)

As an OSCAR administrator, I want to set or change the maximum number of MinIO
buckets and the per-bucket MinIO storage limit for a user so storage governance
can be adapted without redeploying the whole platform.

**Why this priority**: Enforcement is useful only if administrators can manage
                       the limit for individual users or align it with existing
                       quota management flows.

**Independent Test**: Update a user's bucket count maximum and
                      `storage_per_bucket` maximum, retrieve quota information,
                      and verify that later bucket creation and per-bucket
                      quota decisions use the updated values.

**Acceptance Scenarios**:

1. **Given** an administrator and a target user, **When** the administrator sets
   that user's bucket count limit, **Then** subsequent quota responses show the
   new maximum.
2. **Given** a user whose bucket limit is lowered below or equal to their
   current bucket count, **When** the user tries to create another bucket,
   **Then** creation is rejected until their owned bucket count falls below the
   limit.
3. **Given** a user whose bucket limit is increased above their current bucket
   count, **When** the user creates another bucket, **Then** creation is allowed
   if all other bucket validation rules pass.
4. **Given** an administrator sets or changes a user's `storage_per_bucket`
   limit,
   **When** the user creates or updates an owned bucket, **Then** OSCAR applies
   the corresponding per-bucket storage quota.

---

### User Story 5 - Preserve Existing Bucket Behavior (Priority: P2)

As an OSCAR user, I want existing bucket visibility, ownership, and service
workflows to keep working so bucket quotas are additive and do not alter
unrelated MinIO behavior.

**Why this priority**: OSCAR already manages bucket ownership, visibility, and
                       service-triggered buckets. Quota support must not
                       regress those flows.

**Independent Test**: Run existing bucket create, list, get, update, and delete
                      flows for users under their limits and verify behavior
                      matches current behavior aside from quota fields.

**Acceptance Scenarios**:

1. **Given** a user under their bucket limit, **When** the user creates private,
   restricted, or public buckets through supported OSCAR flows, **Then**
   existing visibility and policy behavior is preserved.
2. **Given** existing OSCAR-managed buckets with owner metadata, **When** quota
   usage is computed, **Then** those buckets are counted without changing their
   policies, tags, or object data.
3. **Given** a service workflow that creates MinIO buckets on behalf of a user,
   **When** bucket quota enforcement applies to that workflow, **Then** the
   system rejects over-limit creations before leaving partial MinIO resources
   behind.

### Edge Cases

- A bucket exists in MinIO but lacks OSCAR owner metadata because it predates
  bucket tagging or was not created by OSCAR.
- A bucket has malformed or empty owner metadata.
- The authenticated user's UID needs formatting or normalization before
  comparison with stored owner tags.
- The quota limit is set to zero.
- The quota limit is omitted for a user while bucket quota support is enabled.
- The quota limit is lowered below the user's current bucket count.
- A bucket storage quota is set to zero or omitted.
- A bucket storage quota is lowered below the bucket's current stored size.
- A user's reported aggregate storage usage is greater than the configured
  `storage_per_bucket` value for one or more buckets.
- MinIO quota enforcement is delayed because MinIO evaluates bucket quotas
  asynchronously.
- Two bucket creation requests for the same user arrive concurrently when only
  one bucket slot remains.
- MinIO is temporarily unavailable while OSCAR is counting owned buckets.
- MinIO is temporarily unavailable while OSCAR reads, sets, or clears bucket
  storage quota configuration.
- A bucket creation request passes the quota check but fails later during MinIO
  bucket creation, tagging, policy configuration, or bucket quota configuration.
- A regular user creates a bucket directly in MinIO using their AK/SK or the
  MinIO console, bypassing OSCAR's pre-creation quota checks.
- An administrator creates a bucket through OSCAR using basic authentication;
  the bucket owner must remain the OSCAR/admin owner and must not be counted
  against an unrelated end user.
- A directly created MinIO bucket has no OSCAR owner metadata and therefore
  cannot be safely assigned to a user quota without an ownership strategy.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST support a per-user maximum count for
  OSCAR-managed MinIO buckets.
- **FR-002**: The system MUST compute a user's current bucket usage by counting
  OSCAR-managed MinIO buckets whose ownership metadata identifies that user.
- **FR-003**: The system MUST support per-bucket storage quota configuration
  for OSCAR-managed MinIO buckets.
- **FR-004**: The system MUST apply per-bucket storage quotas using MinIO bucket
  quota capabilities when those quotas are configured.
- **FR-005**: The system MUST enforce the bucket count limit before creating a
  bucket through OSCAR-controlled bucket creation APIs such as
  `/system/buckets`.
- **FR-006**: The system MUST reject bucket creation when the user's current
  owned bucket count is greater than or equal to the user's configured maximum.
- **FR-007**: The system MUST return a human-readable quota-exceeded error that
  includes enough information for the caller to understand that the bucket count
  limit was reached.
- **FR-008**: The system MUST ensure that rejected over-limit bucket creation
  requests do not create, tag, or partially configure a MinIO bucket.
- **FR-009**: The system MUST include bucket quota information in user quota
  responses, including maximum allowed bucket count and current used bucket
  count.
- **FR-010**: User quota responses MUST include MinIO storage quota information,
  including configured `storage_per_bucket` limits and aggregate storage usage
  when available.
- **FR-011**: Bucket read/list responses SHOULD expose the configured
  per-bucket storage quota and current bucket storage usage when available.
- **FR-012**: The system MUST allow administrators to read bucket quota
  information for a specific user through the existing administrator quota
  access pattern.
- **FR-013**: The system MUST allow administrators to update a user's maximum
  bucket count and `storage_per_bucket` limit through the existing
  administrator quota update pattern or an equivalent quota-management API.
- **FR-014**: The system MUST allow administrators to define or update the
  storage quota for an individual OSCAR-managed MinIO bucket.
- **FR-015**: The system MUST validate quota update values and reject negative,
  non-numeric, unsupported-unit, or otherwise invalid bucket limits with clear
  validation messages.
- **FR-016**: The system MUST validate per-bucket storage quota values and
  reject negative, non-numeric, unsupported-unit, or otherwise invalid storage
  values with clear validation messages.
- **FR-017**: The system MUST expose `storage_per_bucket` as the enforceable
  MinIO storage quota setting and MUST NOT present it as a guaranteed aggregate
  per-user storage cap.
- **FR-018**: The system MUST report that MinIO bucket quota enforcement is not
  instantaneous and depends on MinIO's quota evaluation behavior.
- **FR-019**: The system MUST preserve existing bucket ownership tags,
  visibility policies, allowed-user behavior, and list/get/update/delete bucket
  semantics for users who are under their bucket and storage limits.
- **FR-020**: The system MUST NOT count buckets owned by another user against
  the authenticated user's quota.
- **FR-021**: The system MUST NOT count administrator-owned OSCAR buckets
  against a regular user's quota.
- **FR-022**: The system MUST define how buckets with missing or unreadable
  owner metadata affect quota usage and MUST avoid granting regular users extra
  quota capacity because of ambiguous ownership.
- **FR-023**: The system MUST handle MinIO read/count failures by failing quota
  checks safely and returning an explicit error rather than allowing unchecked
  bucket creation.
- **FR-024**: The system MUST handle MinIO bucket quota read/update failures by
  returning an explicit error and avoiding partial quota state where feasible.
- **FR-025**: The system MUST keep bucket count quotas and bucket storage quotas
  available without requiring MinIO distributed mode.
- **FR-026**: The system MUST state that direct bucket creation in MinIO through
  user AK/SK or the MinIO console bypasses OSCAR pre-creation bucket count
  checks and cannot be prevented by OSCAR.
- **FR-027**: The system MUST preserve compatibility with existing deployments
  when bucket quota support is not configured or disabled.
- **FR-028**: The system MUST provide test coverage for quota usage calculation,
  successful under-limit creation, over-limit rejection, per-bucket storage
  quota configuration, administrator quota retrieval, and quota update
  validation.
- **FR-029**: The system MUST NOT introduce new external dependencies for bucket
  quota support unless explicitly approved by maintainers.
- **FR-030**: The system MUST distinguish in documentation and user-facing
  quota responses between enforceable OSCAR-controlled bucket creation limits
  and buckets created directly in MinIO outside OSCAR's control when such
  buckets can be detected.

### Key Entities *(include if feature involves data)*

- **Bucket Quota**: The per-user maximum number of OSCAR-managed MinIO buckets
  a user may own, along with the user's current bucket usage.
- **Bucket Storage Quota**: The maximum amount of object data an individual
  OSCAR-managed MinIO bucket may store, configured through MinIO bucket quota
  support and exposed as `storage_per_bucket`.
- **User MinIO Storage Usage**: The reported amount of MinIO storage attributed
  to a user across OSCAR-managed buckets. This is reporting data, not a native
  MinIO aggregate quota.
- **Owned Bucket**: A MinIO bucket managed by OSCAR whose metadata identifies a
  specific OSCAR user as owner.
- **Bucket Owner Metadata**: The MinIO bucket tags or equivalent metadata used
  by OSCAR to associate a bucket with the UID of the user who owns it.
- **Quota Update**: An administrator-authorized change to a user's bucket count
  maximum, `storage_per_bucket` maximum, or individual bucket storage maximum.
- **Quota Usage Snapshot**: The current computed view of a user's owned bucket
  count and storage usage at the time OSCAR evaluates a quota request.

### Assumptions

- Regular users may create buckets directly in MinIO using their AK/SK or the
  MinIO console, and OSCAR cannot prevent or pre-validate those creations.
- OSCAR already tags newly created user buckets with owner metadata and can use
  that metadata as the source of truth for bucket ownership.
- Existing buckets that lack owner metadata may require conservative handling or
  a migration/repair path before strict quota enforcement is enabled.
- Bucket count quotas are optional and can be rolled out without changing
  behavior for installations that do not configure them.
- A quota limit of zero means the user is not allowed to create any
  user-owned MinIO buckets.
- A storage quota of zero means the bucket should not accept user object data
  beyond MinIO's representation of an empty bucket.
- Lowering a limit below the current bucket count does not delete existing
  buckets; it only prevents new bucket creation until usage is below the limit.
- Lowering a bucket storage quota below current usage does not delete objects;
  it prevents additional writes according to MinIO quota behavior until usage is
  below the limit.
- MinIO bucket quota enforcement is best-effort rather than an exact immediate
  byte-level admission control, because MinIO evaluates quotas asynchronously.
- `storage_per_bucket` is the enforceable MinIO storage setting. Aggregate
  MinIO storage usage per user is reported for visibility but is not a native
  MinIO per-user aggregate quota.
- Buckets created directly in MinIO outside OSCAR may be visible only after the
  fact and may lack the metadata needed for reliable per-user attribution.
- Dashboard UI changes may be needed for full user visibility, but the backend
  API/specification is the authoritative scope in this repository.

## Non-Goals

- Guaranteeing exact immediate byte-level enforcement beyond MinIO's bucket
  quota behavior.
- Replacing MinIO bucket quota support with a custom object admission proxy.
- Changing the current MinIO deployment topology or requiring distributed
  MinIO.
- Preventing regular users from creating buckets directly in MinIO with valid
  AK/SK credentials or through the MinIO console.
- Deleting existing buckets automatically when an administrator lowers a user's
  bucket count limit.
- Deleting existing objects automatically when an administrator lowers a bucket
  storage quota.
- Reworking bucket visibility, sharing, or policy semantics beyond what is
  necessary to preserve quota enforcement.
- Introducing new external dependencies or a new quota service.
- Guaranteeing a strict maximum bucket count across buckets created outside
  OSCAR's control.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In validation runs, 100% of over-limit bucket creation requests
  through OSCAR-controlled APIs are rejected before OSCAR creates a MinIO
  bucket.
- **SC-002**: In validation runs, 100% of under-limit bucket creation requests
  continue to succeed when existing bucket validation, tagging, and policy
  checks pass.
- **SC-003**: In validation runs, quota usage counts match the number of
  OSCAR-managed buckets tagged for the target user in 100% of tested ownership
  scenarios.
- **SC-004**: In validation runs, per-bucket storage quota configuration is
  applied and readable for 100% of tested OSCAR-managed MinIO buckets with a
  configured storage quota.
- **SC-005**: In validation runs, writes to a bucket at or above its configured
  storage quota are rejected according to MinIO bucket quota behavior.
- **SC-006**: In validation runs, reported user MinIO storage usage matches the
  sum of tested OSCAR-managed bucket usage for the target user.
- **SC-007**: In validation runs, lowering a user's bucket limit below current
  usage prevents new bucket creation without deleting existing buckets.
- **SC-008**: In validation runs, lowering a bucket storage quota below current
  usage prevents additional writes according to MinIO quota behavior without
  deleting existing objects.
- **SC-009**: 100% of invalid bucket quota update payloads are rejected with a
  human-readable reason.
- **SC-010**: Existing bucket API tests for create, list, get, update, and
  delete behavior pass unchanged except for intentional quota response additions.
- **SC-011**: Documentation and API descriptions clearly state that bucket count
  enforcement applies to OSCAR-controlled bucket creation and cannot prevent
  direct MinIO bucket creation with user AK/SK credentials.
