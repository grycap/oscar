# Tasks: Bucket Quotas

**Input**: Design documents from `specs/007-bucket-quotas/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md,
                   contracts/bucket-quotas.openapi.yaml

**Tests**: Include Go test tasks for each story because the repository
           constitution requires tests for touched Go packages.

**Organization**: Tasks are grouped by user story to enable independent
                  implementation and testing of each story.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare shared quota types and MinIO helper surfaces used by all
             bucket quota stories.

- [X] T001 [P] Add MinIO quota response and update structs in pkg/types/quotas.go
- [X] T002 [P] Extend MinIO bucket response structs with storage quota, storage
  usage, and attribution fields in pkg/utils/minio.go
- [X] T003 [P] Add tests for MinIO quota JSON serialization in
  pkg/types/quotas_test.go
- [X] T004 [P] Add tests for extended MinIO bucket JSON serialization in
  pkg/utils/minio_test.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish shared parsing, MinIO quota, attribution, and usage
             helpers required before user stories can be implemented.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T005 Implement storage quantity parsing/formatting helpers for
  `storage_per_bucket` in pkg/utils/minio.go
- [X] T006 [P] Add unit tests for valid and invalid `storage_per_bucket`
  quantities in pkg/utils/minio_test.go
- [X] T007 Implement MinIO admin helper methods for get/set bucket quota in
  pkg/utils/minio.go using existing `madmin-go`
- [X] T008 [P] Add MinIO admin helper tests for bucket quota get/set behavior in
  pkg/utils/minio_test.go
- [X] T009 Implement helper to list/count buckets by OSCAR owner metadata in
  pkg/utils/minio.go
- [X] T010 [P] Add owner-attribution tests for tagged, untagged, malformed, and
  admin-owned buckets in pkg/utils/minio_test.go
- [X] T011 Implement helper to collect attributed bucket storage usage in
  pkg/utils/minio.go
- [X] T012 [P] Add usage aggregation tests for complete, partial, and unknown
  attribution in pkg/utils/minio_test.go
- [X] T013 Implement ConfigMap helpers for reading/upserting
  `oscar-minio-quota` in the user namespace in pkg/handlers/quotas.go or a
  focused helper file
- [X] T014 [P] Add ConfigMap helper tests for missing, valid, invalid, and
  updated MinIO quota settings in pkg/handlers/quotas_test.go

**Checkpoint**: Foundation ready; quota enforcement and reporting stories can
                now be implemented.

---

## Phase 3: User Story 1 - Enforce Bucket Count Limit (P1 MVP)

**Goal**: Reject over-limit bucket creation for OSCAR-controlled bucket
          creation paths while documenting that direct MinIO creation bypasses
          pre-checks.

**Independent Test**: Configure a bucket count limit, create buckets through
                      `/system/buckets` until the limit is reached, and verify
                      the next OSCAR request is rejected without creating a
                      MinIO bucket.

### Tests for User Story 1

- [X] T015 [P] [US1] Add `/system/buckets` under-limit and over-limit handler
  tests in pkg/handlers/buckets/create_bucket_test.go
- [X] T016 [P] [US1] Add service-driven bucket creation quota tests in
  pkg/handlers/create_test.go
- [X] T017 [P] [US1] Add quota error response tests for safe failure when MinIO
  bucket counting fails in pkg/handlers/buckets/create_bucket_test.go

### Implementation for User Story 1

- [X] T018 [US1] Add bucket count quota validation before
  `CreateS3Path` in pkg/handlers/buckets/create_bucket.go
- [X] T019 [US1] Add bucket count quota validation before service-managed
  MinIO bucket creation in pkg/handlers/create.go
- [X] T020 [US1] Ensure over-limit bucket creation returns a clear quota error
  without creating/tagging/configuring a bucket in pkg/handlers/buckets/create_bucket.go

**Checkpoint**: User Story 1 should enforce bucket count quotas for
                OSCAR-controlled creation paths.

---

## Phase 4: User Story 2 - Enforce Per-Bucket Storage Quota (P1)

**Goal**: Apply MinIO native per-bucket quotas using the configured
          `storage_per_bucket` value for OSCAR-managed buckets.

**Independent Test**: Configure `storage_per_bucket`, create a bucket through
                      OSCAR, and verify the configured MinIO bucket quota is
                      applied and readable.

### Tests for User Story 2

- [X] T021 [P] [US2] Add bucket create tests that verify configured
  `storage_per_bucket` is applied in pkg/handlers/buckets/create_bucket_test.go
- [X] T022 [P] [US2] Add service-driven bucket create tests that verify
  `storage_per_bucket` is applied in pkg/handlers/create_test.go
- [X] T023 [P] [US2] Add failure tests for MinIO quota set errors after bucket
  creation in pkg/handlers/buckets/create_bucket_test.go

### Implementation for User Story 2

- [X] T024 [US2] Apply configured `storage_per_bucket` after successful
  OSCAR-managed bucket creation in pkg/handlers/buckets/create_bucket.go
- [X] T025 [US2] Apply configured `storage_per_bucket` for service-created
  MinIO buckets in pkg/handlers/create.go
- [X] T026 [US2] Return explicit errors for MinIO quota read/update failures in
  pkg/handlers/buckets/create_bucket.go and pkg/handlers/create.go

**Checkpoint**: User Story 2 should apply per-bucket MinIO storage quotas for
                OSCAR-managed buckets.

---

## Phase 5: User Story 3 - Inspect Bucket Quota Usage (P1)

**Goal**: Show bucket count usage, `storage_per_bucket`, aggregate storage usage,
          and direct-creation limitation in user quota and bucket responses.

**Independent Test**: Request own quota and admin quota views and verify MinIO
                      quota fields reflect attributed buckets and usage.

### Tests for User Story 3

- [X] T027 [P] [US3] Add quota response tests for `minio.buckets`,
  `minio.storage_per_bucket`, and `minio.storage_total` in
  pkg/handlers/quotas_test.go
- [X] T028 [P] [US3] Add bucket list/detail quota metadata tests in
  pkg/handlers/buckets/list_bucket_test.go and
  pkg/handlers/buckets/get_bucket_test.go
- [X] T029 [P] [US3] Add partial attribution tests for directly created or
  untagged buckets in pkg/handlers/quotas_test.go (covered at helper level;
  `/system/quotas` no longer exposes attribution)

### Implementation for User Story 3

- [X] T030 [US3] Extend quota fetch logic to populate MinIO quota fields from
  the `oscar-minio-quota` ConfigMap in
  pkg/handlers/quotas.go
- [X] T031 [US3] Include storage quota and storage usage metadata in bucket list
  responses in pkg/handlers/buckets/list_bucket.go
- [X] T032 [US3] Include storage quota and storage usage metadata in bucket
  detail responses in pkg/handlers/buckets/get_bucket.go
- [X] T033 [US3] Keep quota responses focused on machine-readable MinIO quota
  fields without per-response explanatory messages in pkg/handlers/quotas.go

**Checkpoint**: User Story 3 should make MinIO quota state visible without
                overstating enforcement for direct MinIO bucket creation.

---

## Phase 6: User Story 4 - Update User Bucket Limits (P2)

**Goal**: Let administrators update per-user bucket count and
          `storage_per_bucket` limits through the quota update flow.

**Independent Test**: Update a user's MinIO quota settings through
                      `/system/quotas/user/{userId}` and verify later quota
                      reads and bucket creation use the updated values.

### Tests for User Story 4

- [X] T034 [P] [US4] Add quota update validation tests for valid bucket count
  and `storage_per_bucket` payloads in pkg/handlers/quotas_test.go
- [X] T035 [P] [US4] Add quota update validation tests for negative,
  non-numeric, and invalid-unit MinIO quota payloads in pkg/handlers/quotas_test.go
- [X] T036 [P] [US4] Add type-level JSON tag tests for MinIO quota updates in
  pkg/types/quotas_test.go

### Implementation for User Story 4

- [X] T037 [US4] Extend `QuotaUpdateRequest` validation to accept MinIO quota
  updates in pkg/handlers/quotas.go
- [X] T038 [US4] Persist updated per-user bucket count and
  `storage_per_bucket` settings in the `oscar-minio-quota` ConfigMap in
  pkg/handlers/quotas.go
- [X] T039 [US4] Ensure lowered bucket count limits block only new
  OSCAR-controlled bucket creation without deleting existing buckets in
  pkg/handlers/quotas.go and pkg/handlers/buckets/create_bucket.go

**Checkpoint**: User Story 4 should let admins manage MinIO quota settings and
                have those settings affect later OSCAR-controlled operations.

---

## Phase 7: User Story 5 - Preserve Existing Bucket Behavior (P2)

**Goal**: Preserve existing bucket visibility, ownership, service, and direct
          MinIO limitation behavior when quotas are absent or not applicable.

**Independent Test**: Run existing bucket create/list/get/update/delete flows
                      with no MinIO quota configured and verify behavior is
                      unchanged.

### Tests for User Story 5

- [X] T040 [P] [US5] Add no-quota regression tests for bucket create/list/get
  flows in pkg/handlers/buckets/create_bucket_test.go,
  pkg/handlers/buckets/list_bucket_test.go, and
  pkg/handlers/buckets/get_bucket_test.go
- [X] T041 [P] [US5] Add regression tests for existing service MinIO bucket
  creation without quota settings in pkg/handlers/create_test.go

### Implementation for User Story 5

- [X] T042 [US5] Preserve existing bucket behavior when MinIO quota settings
  are unset in pkg/handlers/buckets/create_bucket.go,
  pkg/handlers/buckets/list_bucket.go, and pkg/handlers/buckets/get_bucket.go
- [X] T043 [US5] Preserve existing service bucket behavior when MinIO quota
  settings are unset in pkg/handlers/create.go

**Checkpoint**: User Story 5 should confirm bucket quotas are additive and
                non-disruptive when unset.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Finalize docs, generated API artifacts, and validation across all
             stories.

- [X] T044 [P] Update MinIO quota documentation in docs/minio-usage.md and
  docs/additional-config.md
- [X] T045 [P] Update quota API documentation in docs/api.md
- [X] T046 Regenerate Swagger artifacts if API comments changed with
  `go generate ./...`
- [X] T047 Run targeted tests:
  `go test ./pkg/types ./pkg/utils ./pkg/handlers ./pkg/handlers/buckets`
- [X] T048 Run broader validation with `go test ./...` when feasible
- [X] T049 Record implementation validation results in
  specs/007-bucket-quotas/quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately.
- **Foundational (Phase 2)**: Depends on Phase 1; blocks all user stories.
- **User Story 1 (Phase 3)**: Depends on Phase 2.
- **User Story 2 (Phase 4)**: Depends on Phase 2 and can proceed in parallel
  with US1 after shared helpers exist.
- **User Story 3 (Phase 5)**: Depends on Phase 2 and can proceed once quota
  response models and helper surfaces exist; integrates with US1/US2 data when
  available.
- **User Story 4 (Phase 6)**: Depends on Phase 2 and should land before final
  end-to-end validation of US1/US2.
- **User Story 5 (Phase 7)**: Depends on Phases 3 through 6 because regression
  checks should cover final changed flows.
- **Polish (Phase 8)**: Depends on all desired user stories being complete.

### User Story Dependencies

- **User Story 1 (P1)**: MVP bucket-count enforcement for OSCAR-controlled
  creation.
- **User Story 2 (P1)**: Independent per-bucket storage enforcement once MinIO
  quota helpers exist.
- **User Story 3 (P1)**: Independent visibility story after shared models and
  usage helpers exist.
- **User Story 4 (P2)**: Admin management story that wires settings into the
  enforcement/reporting paths.
- **User Story 5 (P2)**: Compatibility story after main quota changes are in
  place.

### Parallel Opportunities

- `T001`, `T002`, `T003`, and `T004` can run in parallel.
- `T006`, `T008`, `T010`, `T012`, and `T014` can run in parallel with helper
  implementation tasks once file ownership is coordinated.
- `T015`, `T016`, and `T017` can run in parallel for US1 tests.
- `T021`, `T022`, and `T023` can run in parallel for US2 tests.
- `T027`, `T028`, and `T029` can run in parallel for US3 tests.
- `T034`, `T035`, and `T036` can run in parallel for US4 tests.
- `T040` and `T041` can run in parallel for US5 regression tests.
- `T044` and `T045` can run in parallel during polish.

---

## Parallel Example: User Story 1

```bash
Task: "Add /system/buckets under-limit and over-limit handler tests in
pkg/handlers/buckets/create_bucket_test.go"
Task: "Add service-driven bucket creation quota tests in pkg/handlers/create_test.go"
Task: "Add quota error response tests for safe failure when MinIO bucket counting
fails in pkg/handlers/buckets/create_bucket_test.go"
```

## Parallel Example: User Story 2

```bash
Task: "Add bucket create tests that verify configured storage_per_bucket is
applied in pkg/handlers/buckets/create_bucket_test.go"
Task: "Add service-driven bucket create tests that verify storage_per_bucket is
applied in pkg/handlers/create_test.go"
```

## Implementation Strategy

### MVP First

1. Complete Phase 1 and Phase 2.
2. Implement US1 bucket count enforcement for OSCAR-controlled creation.
3. Implement US2 per-bucket storage quota application.
4. Implement enough of US3 to expose quota state to users/admins.
5. Validate with targeted tests before adding admin update and regression
   polish.

### Incremental Delivery

1. Foundation helpers and types.
2. Bucket count enforcement.
3. Per-bucket storage quota application.
4. Quota visibility/reporting.
5. Admin updates.
6. Compatibility and documentation polish.

## Notes

- Direct MinIO bucket creation with user AK/SK or the MinIO console bypasses
  OSCAR pre-creation checks and must remain documented as a limitation.
- `storage_per_bucket` is the enforceable MinIO storage quota setting.
- Aggregate `storage_total` is reporting data and must not be presented as a
  strict native per-user storage cap.
- Avoid new dependencies; use existing `madmin-go` support.
