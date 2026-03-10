# Phase 0 Research: Persistent Workspaces for OSCAR Services

## Decision 1: Represent workspace as an optional top-level field in Service/FDL

- Decision: Add an optional `workspace` object at service-definition level (`functions.oscar[].<cluster>.workspace`) with fields for size and mount path.
- Rationale: Aligns with feature spec and existing FDL style where optional service capabilities are top-level objects (`mount`, `expose`, `synchronous`). Keeps configuration declarative and minimal.
- Alternatives considered:
  - Reuse `mount` field: rejected because `mount` models external storage providers, not OSCAR-managed persistent workspaces.
  - Global workspace defaults only: rejected because per-service control is required.

## Decision 2: Lifecycle default is delete-with-service (no retain support in this scope)

- Decision: Workspace storage is removed when its service is deleted.
- Rationale: Matches current spec assumptions and RFC scope, and avoids introducing an additional lifecycle API/UX in this repository phase.
- Alternatives considered:
  - Add `retain` lifecycle option now: rejected as out of scope for this feature slice and would expand API/operational complexity.

## Decision 3: Workspace updates are immutable after creation in this phase

- Decision: Changing workspace configuration (size/mount path) via service update is rejected with validation error.
- Rationale: Avoids unsafe in-place storage mutation semantics and prevents ambiguous migration behavior in first release.
- Alternatives considered:
  - Allow size increase only: rejected for initial implementation due to backend/storage-class variability.
  - Allow full mutable updates with migration: rejected due to high complexity and failure-handling requirements.

## Decision 4: API integration pattern uses existing `/system/services` endpoints

- Decision: Add workspace support by extending existing service payload schema for POST/PUT/GET/LIST under `/system/services`.
- Rationale: Preserves existing API shape and client workflow; no new endpoint family required for MVP.
- Alternatives considered:
  - Create dedicated `/system/workspaces` endpoints: rejected as unnecessary for current requirements and higher migration cost.

## Decision 5: Status exposure through service read/list payload

- Decision: Expose workspace lifecycle state as part of service state metadata returned by existing service-read/list responses.
- Rationale: Satisfies operational visibility requirement without adding a separate status API.
- Alternatives considered:
  - Emit status in logs/events only: rejected because it is less discoverable for API clients.

## Decision 6: Validation rules for workspace input

- Decision: Enforce required workspace fields, valid size format, and mount-path sanity (absolute path, reserved-path safeguards), and reject invalid configs pre-deploy.
- Rationale: Meets FR-006 and prevents deployment-time failures where possible.
- Alternatives considered:
  - Best-effort validation at deploy time only: rejected due to weaker UX and harder troubleshooting.

## Decision 7: Backward compatibility and non-goal boundaries

- Decision: Services without `workspace` remain unchanged; object-storage input/output and external `mount` behavior remain intact.
- Rationale: Directly required by FR-009/FR-010 and constitution scope discipline.
- Alternatives considered:
  - Unify workspace with existing storage flows: rejected because it would risk behavior regressions.
