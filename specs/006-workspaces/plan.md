# Implementation Plan: Persistent Workspaces for OSCAR Services

**Branch**: `006-workspaces` | **Date**: 2026-03-09 | **Spec**: [/Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/spec.md](/Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/spec.md)
**Input**: Feature specification from `/specs/006-workspaces/spec.md`

## Summary

Add optional workspace support to OSCAR service definitions so services can declare persistent POSIX-like storage (size + mount path), preserve data across restart/redeploy, and keep current behavior unchanged for services without workspaces. The implementation will extend the existing Service model and `/system/services` flows, add validation and basic workspace status exposure, and document the new FDL field.

## Technical Context

**Language/Version**: Go 1.25.0  
**Primary Dependencies**: Gin HTTP server, Kubernetes client-go/api/apimachinery, existing OSCAR backend/resource packages  
**Storage**: Kubernetes PersistentVolumeClaim-backed workspace per service  
**Testing**: `go test ./pkg/types ./pkg/handlers ./pkg/backends/...` (targeted to touched packages)  
**Target Platform**: Kubernetes-based Linux deployment running OSCAR API server  
**Project Type**: Single backend service (monorepo; Go API + docs)  
**Performance Goals**: Service create/update/read latency remains in current operational range; no measurable regression for non-workspace services  
**Constraints**: No new dependencies; backward-compatible API/FDL behavior; dashboard features explicitly out of scope  
**Scale/Scope**: Per-service workspace provisioning and delete-with-service lifecycle for create/update/delete, plus basic workspace status exposure in existing service responses

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Preserve existing behavior unless explicitly requested and approved: PASS. Workspace is optional; non-workspace services keep current behavior.
- Keep scope minimal; avoid refactors without approval: PASS. Changes are localized to service schema/validation/deployment/documentation paths.
- No new dependencies, license changes, or CI/CD edits without approval: PASS. None planned.
- Do not touch `dashboard/dist` unless the UI source is updated and rebuilt: PASS. Not in scope.
- Go code must be gofmt-formatted and idiomatic; package names are short and lowercase: PASS. Planned for touched Go files.
- Tests for touched Go packages are planned (e.g., `go test ./...`) or a skip reason is recorded: PASS. Targeted package tests planned.
- Documentation updates are planned for any interface/flag/behavior changes: PASS. `docs/fdl.md` and OpenAPI artifacts in `pkg/apidocs` will be updated.

## Project Structure

### Documentation (this feature)

```text
/Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── workspace-services.openapi.yaml
└── tasks.md
```

### Source Code (repository root)

```text
/Users/gmolto/Documents/GitHub/grycap/oscar/
├── main.go
├── pkg/
│   ├── handlers/
│   │   ├── create.go
│   │   ├── update.go
│   │   ├── read.go
│   │   ├── list.go
│   │   ├── delete.go
│   │   └── *_test.go
│   ├── types/
│   │   ├── service.go
│   │   └── *_test.go
│   ├── backends/
│   │   ├── resources/
│   │   └── *_test.go
│   └── apidocs/
└── docs/
    └── fdl.md
```

**Structure Decision**: Use the existing single backend project structure. Implement workspace support by extending existing service/type/handler/backend code paths and associated docs/tests, without introducing new modules.

## Phase 0: Research Outcomes

See [/Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/research.md](/Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/research.md).

## Phase 1: Design Artifacts

- Data model: [/Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/data-model.md](/Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/data-model.md)
- Contracts: [/Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/contracts/workspace-services.openapi.yaml](/Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/contracts/workspace-services.openapi.yaml)
- Quickstart: [/Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/quickstart.md](/Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/quickstart.md)

## Post-Design Constitution Check

- Preserve existing behavior unless explicitly requested and approved: PASS. Contract and model keep workspace optional and default-off.
- Keep scope minimal; avoid refactors without approval: PASS. Design confines changes to service lifecycle paths.
- No new dependencies, license changes, or CI/CD edits without approval: PASS. None introduced.
- Do not touch `dashboard/dist` unless the UI source is updated and rebuilt: PASS. Excluded.
- Go code must be gofmt-formatted and idiomatic; package names are short and lowercase: PASS. Still applicable for implementation phase.
- Tests for touched Go packages are planned or a skip reason is recorded: PASS. Test plan retained in quickstart.
- Documentation updates are planned for any interface/flag/behavior changes: PASS. `docs/fdl.md` and OpenAPI artifacts are explicitly listed.

## Complexity Tracking

No constitution violations identified.
