# Implementation Plan: Federated OSCAR Service Replicas (Topology: star/mesh)

**Branch**: `002-federation` | **Date**: 2026-01-29 | **Spec**: `/specs/002-federation/spec.md`
**Input**: Feature specification from `/specs/002-federation/spec.md`

## Summary

Enable federated OSCAR service replicas across multiple clusters with star/mesh
topology support. OSCAR Manager expands coordinator FDLs into per-cluster
services, and `/system/federation` manages topology-wide replica updates. Job
delegation follows static/random/load-based policies using `/system/status`
metrics, with inter-cluster auth based on refresh-token exchange to mint fresh
OIDC bearer tokens and create-time transactional deployment across clusters
(rollback on any replica failure during initial creation).
Load-based ranking uses total free CPU only, with a per-node fit check.

## Technical Context

**Language/Version**: Go 1.25  
**Primary Dependencies**: gin-gonic, client-go, metrics.k8s.io client  
**Storage**: Kubernetes API resources; output storage via MinIO/external providers  
**Testing**: `go test ./...` (touched Go packages)  
**Target Platform**: Linux / Kubernetes OSCAR clusters
**Project Type**: Single Go service (OSCAR Manager/API)  
**Performance Goals**: No new numeric targets; must not regress existing
scheduling/delegation behavior  
**Constraints**: No new dependencies/CI changes without approval; do not modify
`dashboard/dist`; preserve existing behavior unless requested; refresh-token
handling must avoid exposing tokens to service pods  
**Scale/Scope**: Multi-cluster federations, N unspecified; design for reasonable
cluster counts without new infrastructure

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Preserve existing behavior unless explicitly requested and approved.
- Keep scope minimal; avoid refactors without approval.
- No new dependencies, license changes, or CI/CD edits without approval.
- Do not touch `dashboard/dist` unless the UI source is updated and rebuilt.
- Go code must be gofmt-formatted and idiomatic; package names are short and lowercase.
- Tests for touched Go packages are planned (e.g., `go test ./...`) or a skip reason is recorded.
- Documentation updates are planned for any interface/flag/behavior changes.

## Project Structure

### Documentation (this feature)

```text
specs/002-federation/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
main.go
pkg/
├── handlers/
├── types/
├── backends/
├── resourcemanager/
├── utils/
└── metrics/
```

**Structure Decision**: Single Go service with `main.go` and feature work in
`pkg/handlers`, `pkg/types`, and supporting packages.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
