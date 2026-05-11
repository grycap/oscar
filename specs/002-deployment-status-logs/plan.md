# Implementation Plan: Service Deployment Visibility

**Branch**: `002-deployment-status-logs` | **Date**: 2026-04-04 | **Spec**: /Users/gmolto/Documents/GitHub/grycap/oscar/specs/002-deployment-status-logs/spec.md
**Input**: Feature specification from `/Users/gmolto/Documents/GitHub/grycap/oscar/specs/002-deployment-status-logs/spec.md`

## Summary

Add API-only deployment visibility for OSCAR services by exposing a service-level
deployment summary and service-level current or recent deployment logs.
Coverage applies across OSCAR service types, but the status endpoint returns
`unavailable` when a service has no current deployment or runtime
representation to inspect. The log endpoint returns raw deployment logs to
authorized service viewers and may return recent last-attempt logs only when
those logs are already available through the existing operational log source
without introducing a separate complex retrieval path.

## Technical Context

**Language/Version**: Go 1.25.0  
**Primary Dependencies**: gin-gonic/gin, k8s.io/client-go, k8s.io/api,
                          k8s.io/apimachinery, existing OSCAR
                          handlers/backends/resources/types/auth packages,
                          knative.dev/serving already present in the repo  
**Storage**: N/A (read-only inspection of existing Kubernetes resources and log
              streams)  
**Testing**: `go test ./pkg/handlers ./pkg/types ./pkg/backends/...`,
             `go generate ./...` for API docs regeneration, `mkdocs serve`
             when docs validation is feasible  
**Target Platform**: Linux-based OSCAR manager running on Kubernetes  
**Project Type**: Single Go backend service with repository-hosted docs  
**Performance Goals**: One request returns the current deployment summary; one
                       additional request returns current or recent
                       service-level deployment logs; avoid history scans
                       beyond the recent log window  
**Constraints**: No new dependencies; preserve existing service CRUD and job-log
                 behavior; additive API-only scope; service-summary-only
                 responses; raw log content for authorized viewers; support all
                 OSCAR service types with `unavailable` fallback when
                 deployment visibility does not apply  
**Scale/Scope**: Add service-scoped deployment visibility endpoints under the
                 existing `/system/services/:serviceName` API surface, cover
                 exposed, sync, and async services at the API level, and allow
                 recent last-attempt logs only when available through existing
                 operational log sources

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Preserve existing behavior unless explicitly requested and approved: PASS.
  The feature adds new read-only endpoints and does not alter existing service,
  job, or system-log behavior.
- Keep scope minimal; avoid refactors without approval: PASS. The plan reuses
  the current service authorization flow, namespace resolution, and runtime
  resource lookups.
- No new dependencies, license changes, or CI/CD edits without approval: PASS.
  None are planned.
- Do not touch `dashboard/dist` unless the UI source is updated and rebuilt:
  PASS. Dashboard work remains out of scope.
- Go code must be gofmt-formatted and idiomatic; package names are short and
  lowercase: PASS. Planned changes stay within existing Go packages.
- Tests for touched Go packages are planned or a skip reason is recorded: PASS.
  Handler, type, and backend/resource tests plus API-doc regeneration are
  planned.
- Documentation updates are planned for any interface/flag/behavior changes:
  PASS. API contract and docs updates are in scope.

Gate evaluation: Pass. The requested feature is an additive API slice that fits
the repository guardrails without new dependencies or workflow changes.

## Project Structure

### Documentation (this feature)

```text
/Users/gmolto/Documents/GitHub/grycap/oscar/specs/002-deployment-status-logs/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── deployment-visibility.openapi.yaml
└── tasks.md
```

### Source Code (repository root)

```text
/Users/gmolto/Documents/GitHub/grycap/oscar/
├── main.go
├── pkg/
│   ├── handlers/
│   │   ├── logs.go
│   │   ├── read.go
│   │   ├── status.go
│   │   └── *_test.go
│   ├── backends/
│   │   ├── k8s.go
│   │   ├── knative.go
│   │   ├── resources/
│   │   │   └── expose.go
│   │   └── *_test.go
│   ├── types/
│   │   ├── service.go
│   │   ├── status.go
│   │   ├── deployment.go
│   │   └── *_test.go
│   └── apidocs/
└── docs/
    └── api.md
```

**Structure Decision**: Keep the existing single-backend Go layout. Add a
focused deployment-visibility handler and response types, wire the new routes
through `main.go`, reuse existing backend and resource lookups to inspect live
runtime objects, and keep API documentation in the existing Swagger/docs flow.

## Phase 0: Research Outcomes

See /Users/gmolto/Documents/GitHub/grycap/oscar/specs/002-deployment-status-logs/research.md

## Phase 1: Design Artifacts

- Data model: /Users/gmolto/Documents/GitHub/grycap/oscar/specs/002-deployment-status-logs/data-model.md
- Contracts: /Users/gmolto/Documents/GitHub/grycap/oscar/specs/002-deployment-status-logs/contracts/deployment-visibility.openapi.yaml
- Quickstart: /Users/gmolto/Documents/GitHub/grycap/oscar/specs/002-deployment-status-logs/quickstart.md

## Post-Design Constitution Check

- Preserve existing behavior unless explicitly requested and approved: PASS.
  The design adds new endpoints only and leaves existing handlers unchanged.
- Keep scope minimal; avoid refactors without approval: PASS. Existing
  authorization and namespace helpers are reused instead of introducing a new
  access layer.
- No new dependencies, license changes, or CI/CD edits without approval: PASS.
  The design stays within current Gin, Kubernetes, and Knative dependencies.
- Do not touch `dashboard/dist` unless the UI source is updated and rebuilt:
  PASS. No dashboard artifacts are part of this plan.
- Go code must be gofmt-formatted and idiomatic; package names are short and
  lowercase: PASS. Planned additions are small Go handlers and response types.
- Tests for touched Go packages are planned or a skip reason is recorded: PASS.
  Handler, backend/resource, and type tests remain part of the plan; docs-only
  planning changes do not require running Go tests now.
- Documentation updates are planned for any interface/flag/behavior changes:
  PASS. API contract and documentation regeneration remain in scope.

## Complexity Tracking

No constitution violations identified.
