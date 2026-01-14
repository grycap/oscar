# Implementation Plan: Metrics Collection Improvements

**Branch**: `001-metrics-collection` | **Date**: 2026-01-13 | **Spec**: /Users/gmolto/Documents/GitHub/grycap/oscar/specs/001-metrics-collection/spec.md
**Input**: Feature specification from `/specs/001-metrics-collection/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Deliver metrics reporting for OSCAR clusters with per-service, per-metric
queries plus summary and breakdown outputs, including completeness flags for
missing sources and CSV export for breakdowns.

## Technical Context

**Language/Version**: Go 1.25  
**Primary Dependencies**: gin-gonic, client-go, metrics.k8s.io client  
**Storage**: N/A (aggregation from existing data sources)  
**Testing**: go test ./...  
**Target Platform**: Linux servers running Kubernetes control-plane services  
**Project Type**: single  
**Constraints**: no new dependencies; preserve existing behavior; API changes only for this feature  
**Export Formats**: CSV for breakdown outputs  
**Scale/Scope**: per-cluster reporting for monthly and custom date ranges

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Preserve existing behavior unless explicitly requested and approved.
- Keep scope minimal; avoid refactors without approval.
- No new dependencies, license changes, or CI/CD edits without approval.
- Do not touch `dashboard/dist` unless the UI source is updated and rebuilt.
- Go code must be gofmt-formatted and idiomatic; package names are short and lowercase.
- Tests for touched Go packages are planned (e.g., `go test ./...`) or a skip reason is recorded.
- Documentation updates are planned for any interface/flag/behavior changes.

Gate evaluation: Pass. Changes are additive and confined to reporting APIs and
aggregation logic; no new dependencies. Public API additions are part of the
requested reporting feature.

## Project Structure

### Documentation (this feature)

```text
specs/001-metrics-collection/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── metrics.yaml
└── tasks.md
```

### Source Code (repository root)

```text
main.go
pkg/
├── handlers/
│   └── metrics.go
├── metrics/
│   ├── aggregators.go
│   └── sources.go
└── types/
    └── metrics.go
```

**Structure Decision**: Single Go service. Add a small metrics aggregation
package, a handler for reporting endpoints, and shared types.

## Complexity Tracking

No constitution violations identified.

Post-design check: Pass. No new dependencies or scope expansions introduced.
