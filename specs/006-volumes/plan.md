# Implementation Plan: Managed Volumes for OSCAR Services

**Branch**: `006-volumes` | **Date**: 2026-03-11 | **Spec**:
            [specs/006-volumes/spec.md](specs/006-volumes/spec.md)
**Input**: Feature specification from `specs/006-volumes/spec.md`

## Summary

Implement namespace-scoped managed volumes in OSCAR. The implementation will add
a dedicated `/system/volumes` API, use a `volume` block in the service FDL to
create or mount named volumes, support `retain` and `delete` lifecycle policies
for service-created volumes, and preserve existing behavior for services that do
not opt in. The design reuses the existing Kubernetes PVC provisioning path,
user-namespace resolution, and service CRUD flow to keep the change minimal.

## Technical Context

**Language/Version**: Go 1.25.0  
**Primary Dependencies**: Gin HTTP server, Kubernetes
                          client-go/api/apimachinery, existing OSCAR
                          auth/utils/backends/resources packages
**Storage**: Kubernetes PersistentVolumeClaims in per-user namespaces, backed by
             the current NFS RWX storage-class approach used for managed service
             storage
**Testing**: `go test ./pkg/types ./pkg/handlers ./pkg/backends/...` and `go
             generate ./...` for API docs regeneration; `mkdocs serve` when docs
             validation is feasible
**Target Platform**: Linux-based OSCAR API server running on Kubernetes  
**Project Type**: Single backend service with repository-hosted docs  
**Performance Goals**: No additional measurable performance target is introduced
                       for this feature slice beyond preserving functional
                       correctness
**Constraints**: No new dependencies; preserve existing behavior for services
                 without `volume`; keep auth namespace isolation intact; avoid
                 broad refactors; keep the volume model declarative and
                 low-level-storage-agnostic
**Scale/Scope**: Dedicated `/system/volumes` list/create/read/delete operations,
                 service-time volume creation or attachment by name,
                 namespace-scoped reuse, `retain`/`delete` lifecycle handling,
                 and basic volume status exposure in volume and service
                 responses

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Preserve existing behavior unless explicitly requested and approved: PASS.
  `volume` support is optional and existing services without `volume` keep their
  current behavior.
- Keep scope minimal; avoid refactors without approval: PASS. The design extends
  existing service handlers and resource helpers and adds targeted volume
  handlers rather than reorganizing the backend.
- No new dependencies, license changes, or CI/CD edits without approval: PASS.
  None planned.
- Do not touch `dashboard/dist` unless the UI source is updated and rebuilt:
  PASS. Dashboard work is out of scope.
- Go code must be gofmt-formatted and idiomatic; package names are short and
  lowercase: PASS. Planned changes stay within existing Go packages.
- Tests for touched Go packages are planned (e.g., `go test ./...`) or a skip
  reason is recorded: PASS. Targeted package tests and swagger regeneration are
  planned.
- Documentation updates are planned for any interface/flag/behavior changes:
  PASS. `docs/api.md`, `docs/fdl.md`, `docs/additional-config.md`, and generated
  API docs are in scope.

## Project Structure

### Documentation (this feature)

```text
specs/006-volumes/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── volumes-services.openapi.yaml
└── tasks.md
```

### Source Code (repository root)

```text
├── main.go
├── pkg/
│   ├── handlers/
│   │   ├── create.go
│   │   ├── update.go
│   │   ├── read.go
│   │   ├── list.go
│   │   ├── delete.go
│   │   ├── volume*.go
│   │   └── *_test.go
│   ├── types/
│   │   ├── service.go
│   │   ├── backends.go
│   │   └── *_test.go
│   ├── backends/
│   │   ├── k8s.go
│   │   ├── knative.go
│   │   ├── fake.go
│   │   ├── resources/
│   │   │   ├── volume.go
│   │   │   └── *_test.go
│   │   └── *_test.go
│   └── apidocs/
└── docs/
    ├── api.md
    ├── fdl.md
    └── additional-config.md
```

**Structure Decision**: Use the existing single-backend structure. Extend
                        current service schema and deployment code for the new
                        `volume` field, add dedicated volume handlers under
                        `pkg/handlers`, and add or adapt storage helpers under
                        `pkg/backends/resources` to manage named PVC-backed
                        volumes without introducing new modules or dependencies.

## Phase 0: Research Outcomes

See [specs/006-volumes/research.md](specs/006-volumes/research.md).

## Phase 1: Design Artifacts

- Data model: [specs/006-volumes/data-model.md](specs/006-volumes/data-model.md)
- Contracts:
  [contract](contracts/volumes-services.openapi.yaml)
- Quickstart: [specs/006-volumes/quickstart.md](specs/006-volumes/quickstart.md)

## Post-Design Constitution Check

- Preserve existing behavior unless explicitly requested and approved: PASS.
  Legacy service flows remain unchanged; the new API and FDL fields are opt-in.
- Keep scope minimal; avoid refactors without approval: PASS. The design uses
  new handlers and focused helper updates instead of reshaping backend
  architecture.
- No new dependencies, license changes, or CI/CD edits without approval: PASS.
  None introduced.
- Do not touch `dashboard/dist` unless the UI source is updated and rebuilt:
  PASS. Not part of this feature slice.
- Go code must be gofmt-formatted and idiomatic; package names are short and
  lowercase: PASS. Implementation remains in existing idiomatic Go packages.
- Tests for touched Go packages are planned or a skip reason is recorded: PASS.
  Unit/integration-style handler/backend tests and swagger regeneration are part
  of the plan.
- Documentation updates are planned for any interface/flag/behavior changes:
  PASS. API and FDL docs are explicitly included.

## Complexity Tracking

No constitution violations identified.
