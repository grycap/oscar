# Implementation Plan: Bucket Quotas

**Branch**: `007-bucket-quotas` | **Date**: 2026-05-15 | **Spec**:
            [specs/007-bucket-quotas/spec.md](specs/007-bucket-quotas/spec.md)
**Input**: Feature specification from `specs/007-bucket-quotas/spec.md`

## Summary

Implement MinIO quota support in OSCAR with two clearly separated guarantees:
OSCAR-controlled bucket creation enforces per-user bucket count limits before
creating new buckets, while storage enforcement uses MinIO native per-bucket
quotas exposed as `storage_per_bucket`. Aggregate MinIO storage usage is
reported for visibility, but it is not presented as a strict native per-user
storage cap because users may create buckets directly through MinIO AK/SK or the
MinIO console outside OSCAR pre-checks.

The implementation will extend the existing `/system/quotas/user` API shape,
reuse the current MinIO admin client and owner tags, add focused helpers in
`pkg/utils/minio.go`, and preserve existing bucket visibility and service flows.

## Technical Context

**Language/Version**: Go 1.25.0  
**Primary Dependencies**: Gin HTTP server, AWS S3 SDK, `madmin-go` already used
                          by OSCAR, existing OSCAR handlers/types/utils/auth
                          packages  
**Storage**: Kubernetes ConfigMap `oscar-minio-quota` in each user namespace for
             per-user quota settings, MinIO bucket metadata/tags for ownership,
             MinIO native bucket quota configuration for `storage_per_bucket`,
             existing quota API response models for user-facing quota data  
**Testing**: `go test ./pkg/types ./pkg/utils ./pkg/handlers ./pkg/handlers/buckets`
             and broader `go test ./...` when feasible; Swagger/docs
             regeneration if API docs are updated  
**Target Platform**: Linux-based OSCAR API server running on Kubernetes with a
                     configured MinIO deployment  
**Project Type**: Single backend service with repository-hosted docs  
**Performance Goals**: Quota checks should add only bounded MinIO metadata calls
                       to OSCAR-controlled bucket creation and quota reads  
**Constraints**: No new dependencies; no CI/CD edits; no `dashboard/dist`
                 changes; preserve current behavior when MinIO quotas are not
                 configured; store per-user quota settings in Kubernetes rather
                 than adding a database; do not claim strict enforcement for
                 direct MinIO bucket creation outside OSCAR  
**Scale/Scope**: Extend quota read/update models, bucket create enforcement,
                 MinIO admin helper methods, bucket/list/read quota metadata,
                 and documentation for limitations and operator behavior

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Preserve existing behavior unless explicitly requested and approved: PASS.
  Bucket quotas are additive and must be disabled/omitted without changing
  existing deployments.
- Keep scope minimal; avoid refactors without approval: PASS. The design
  extends current quota handlers, bucket handlers, and MinIO utilities.
- No new dependencies, license changes, or CI/CD edits without approval: PASS.
  Existing `madmin-go` already exposes bucket quota methods.
- Do not touch `dashboard/dist` unless the UI source is updated and rebuilt:
  PASS. Dashboard build artifacts are out of scope.
- Go code must be gofmt-formatted and idiomatic; package names are short and
  lowercase: PASS. Planned changes stay within existing packages.
- Tests for touched Go packages are planned or a skip reason is recorded: PASS.
  Targeted Go tests and broader tests are planned.
- Documentation updates are planned for any interface/flag/behavior changes:
  PASS. API docs and MinIO quota behavior documentation are in scope.

## Project Structure

### Documentation (this feature)

```text
specs/007-bucket-quotas/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── bucket-quotas.openapi.yaml
└── tasks.md
```

### Source Code (repository root)

```text
├── main.go
├── pkg/
│   ├── handlers/
│   │   ├── quotas.go
│   │   ├── quotas_test.go
│   │   └── buckets/
│   │       ├── create_bucket.go
│   │       ├── get_bucket.go
│   │       ├── list_bucket.go
│   │       └── *_test.go
│   ├── types/
│   │   ├── quotas.go
│   │   └── quotas_test.go
│   └── utils/
│       ├── minio.go
│       └── minio_test.go
└── docs/
    ├── api.md
    ├── additional-config.md
    └── minio-usage.md
```

**Structure Decision**: Use the existing single-backend structure. Extend the
                        quota API models in `pkg/types`, store per-user MinIO
                        quota settings in user-namespace ConfigMaps, add MinIO
                        quota helper methods under `pkg/utils`, enforce
                        OSCAR-controlled bucket creation limits in
                        `pkg/handlers/buckets`, and surface quota data through
                        existing quota and bucket handlers.

## Phase 0: Research Outcomes

See [specs/007-bucket-quotas/research.md](specs/007-bucket-quotas/research.md).

## Phase 1: Design Artifacts

- Data model:
  [specs/007-bucket-quotas/data-model.md](specs/007-bucket-quotas/data-model.md)
- Contract:
  [contracts/bucket-quotas.openapi.yaml](contracts/bucket-quotas.openapi.yaml)
- Quickstart:
  [specs/007-bucket-quotas/quickstart.md](specs/007-bucket-quotas/quickstart.md)

## Post-Design Constitution Check

- Preserve existing behavior unless explicitly requested and approved: PASS.
  Quota fields are optional and enforcement is scoped to OSCAR-controlled bucket
  creation.
- Keep scope minimal; avoid refactors without approval: PASS. No broad handler
  or storage-provider redesign is planned.
- No new dependencies, license changes, or CI/CD edits without approval: PASS.
  Existing MinIO admin client support is sufficient.
- Do not touch `dashboard/dist` unless the UI source is updated and rebuilt:
  PASS. Not part of this feature slice.
- Go code must be gofmt-formatted and idiomatic; package names are short and
  lowercase: PASS.
- Tests for touched Go packages are planned or a skip reason is recorded: PASS.
  Unit and handler tests are explicitly part of the implementation plan.
- Documentation updates are planned for any interface/flag/behavior changes:
  PASS. The API contract and docs will document direct-MinIO limitations.

## Complexity Tracking

No constitution violations identified.
