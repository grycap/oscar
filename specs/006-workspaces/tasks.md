# Tasks: Persistent Workspaces for OSCAR Services (MVP)

**Input**: Design documents from `/Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/workspace-services.openapi.yaml

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Keep scope minimal and aligned with existing OSCAR architecture.

- [X] T001 Confirm MVP scope and immutable workspace policy in /Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/spec.md
- [X] T002 [P] Align contract draft to MVP-only behavior in /Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/contracts/workspace-services.openapi.yaml

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add only essential schema + validation needed by both stories.

- [X] T003 Add optional `workspace` service field (`size`, `mount_path`) in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/types/service.go
- [X] T004 [P] Add workspace serialization/parsing tests in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/types/service_test.go
- [X] T005 Implement workspace validation helpers (required fields, size format, absolute mount path) in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/create.go
- [X] T006 [P] Enforce immutable workspace update policy in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/update.go
- [X] T007 Add validation tests for create/update workspace payloads in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/create_test.go
- [X] T008 [P] Add immutable-update rejection tests in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/update_test.go

**Checkpoint**: Minimal model and validation complete.

---

## Phase 3: User Story 1 - Request Persistent Workspace in Service Definition (Priority: P1) 🎯 MVP

**Goal**: Let services declare workspace storage and persist data across restart/redeploy.

**Independent Test**: Create a workspace-enabled service via `/system/services`, verify mount is available, restart/redeploy unchanged config, verify persisted data remains.

- [X] T009 [US1] Implement workspace handling in service create path in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/create.go
- [X] T010 [P] [US1] Implement workspace PVC/resource provisioning logic in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/backends/resources/
- [X] T011 [P] [US1] Add workspace mount wiring in service runtime spec in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/types/service.go
- [X] T012 [US1] Implement basic workspace status fields in service read response in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/read.go
- [X] T013 [P] [US1] Implement basic workspace status fields in service list response in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/list.go
- [X] T014 [US1] Ensure default delete-with-service workspace cleanup in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/backends/k8s.go and /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/backends/knative.go
- [X] T015 [P] [US1] Add create/delete workspace behavior tests in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/create_test.go
- [X] T016 [P] [US1] Add workspace provisioning/cleanup tests in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/backends/resources/
- [X] T017 [P] [US1] Add read/list workspace status tests in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/read_test.go

**Checkpoint**: US1 MVP is independently functional.

---

## Phase 4: User Story 2 - Keep Existing Services Compatible (Priority: P2)

**Goal**: Ensure services without workspace behave exactly as before.

**Independent Test**: Deploy/update legacy service payloads without `workspace` and confirm unchanged behavior; verify existing `mount` behavior is unaffected.

- [X] T018 [US2] Preserve no-workspace create/update behavior in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/create.go
- [X] T019 [P] [US2] Add regression tests for legacy payloads without workspace in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/update_test.go
- [X] T020 [P] [US2] Add regression tests for existing `mount` and storage-provider flows in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/create_test.go

**Checkpoint**: US2 compatibility validated independently.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Keep docs and API description in sync with MVP implementation.

- [X] T021 [P] Regenerate and verify API docs for new `workspace` field in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/apidocs/
- [X] T022 [P] Update FDL docs for MVP workspace behavior in /Users/gmolto/Documents/GitHub/grycap/oscar/docs/fdl.md
- [X] T023 Run targeted Go tests and record outcomes in /Users/gmolto/Documents/GitHub/grycap/oscar/specs/006-workspaces/quickstart.md

---

## Dependencies & Execution Order

- Phase 1 -> Phase 2 -> Phase 3 (US1 MVP) -> Phase 4 (US2) -> Phase 5
- US1 depends on foundational tasks only.
- US2 depends on foundational tasks and validates compatibility after US1 changes.

---

## Parallel Execution Examples

### User Story 1 Parallel Work

- Run T010 and T011 in parallel.
- Run T013 and T016 in parallel.
- Run T015 and T017 in parallel.

### User Story 2 Parallel Work

- Run T019 and T020 in parallel.

---

## Implementation Strategy

### MVP First

1. Complete Phase 1 and Phase 2.
2. Deliver Phase 3 only (US1) and validate independently.
3. Add Phase 4 compatibility hardening (US2).
4. Finish Phase 5 docs/tests.

### Notes

- This task set intentionally avoids overengineering: no new workspace endpoints, no retain lifecycle, no advanced status state machine.
