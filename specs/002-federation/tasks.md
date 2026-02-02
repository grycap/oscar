---

description: "Task list for federated OSCAR service replicas (topology: tree/mesh)"
---

# Tasks: Federated OSCAR Service Replicas (Topology: tree/mesh)

**Input**: Design documents from `/specs/002-federation/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Tests are optional; none are required by the spec at this stage.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and scoping alignment

- [x] T001 Review existing service create/update flow for federation hooks in `pkg/handlers/create.go`
- [x] T002 Review delegation and status logic for extension points in `pkg/resourcemanager/delegate.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared types and validation needed by all stories

- [x] T003 Extend federation fields and validation in `pkg/types/service.go`
- [x] T004 Align replica/federation structs with new semantics in `pkg/types/replica.go`
- [x] T005 Add federation expansion helpers in `pkg/utils/federation.go`

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Deploy a federated replica network (Priority: P1) 🎯 MVP

**Goal**: Deploy a federated service across multiple clusters with tree/mesh topology and expansion rules

**Independent Test**: Provide a valid federation FDL for 2+ clusters and confirm all services are created with correct topology and federation metadata

### Implementation for User Story 1

- [x] T006 [US1] Implement tree topology expansion in `pkg/utils/federation.go`
- [x] T007 [US1] Implement mesh topology expansion in `pkg/utils/federation.go`
- [x] T008 [P] [US1] Integrate federation expansion into service creation in `pkg/handlers/create.go`
- [x] T009 [P] [US1] Integrate federation expansion into service updates in `pkg/handlers/update.go`
- [x] T010 [US1] Ensure worker replicas use empty `federation.members` and strip cluster creds in `pkg/utils/federation.go`

**Checkpoint**: User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - Manage replicas via API (Priority: P2)

**Goal**: Add topology-wide replica management through `/system/replicas`

**Independent Test**: Add a replica via API and confirm it appears in `GET /system/replicas/{serviceName}` across the topology

### Implementation for User Story 2

- [x] T011 [P] [US2] Add replicas request/response models in `pkg/types/replica.go`
- [x] T012 [P] [US2] Implement replicas handlers (GET/POST/PUT/DELETE) in `pkg/handlers/replicas.go`
- [x] T013 [US2] Register `/system/replicas` routes in `main.go`
- [x] T014 [US2] Implement topology-wide replica update propagation in `pkg/utils/federation.go`
- [x] T015 [US2] Add Swagger annotations for replicas endpoints in `pkg/handlers/replicas.go`

**Checkpoint**: User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - Delegate jobs based on policy (Priority: P3)

**Goal**: Delegate jobs using static/random/load-based policies and OIDC-based MinIO access

**Independent Test**: Configure `delegation=random`, submit multiple jobs, and observe varied target clusters

### Implementation for User Story 3

- [x] T016 [P] [US3] Parse delegation policy from service spec in `pkg/types/service.go`
- [x] T017 [US3] Implement policy selection (static/random/load-based) in `pkg/resourcemanager/delegate.go`
- [x] T018 [US3] Use `/system/status` metrics during delegation in `pkg/resourcemanager/delegate.go`
- [x] T019 [US3] Ensure delegated async jobs use bearer token to call `/system/config` for MinIO creds in `pkg/handlers/job.go`
- [x] T020 [P] [US3] Extend status payload if needed for delegation metrics in `pkg/handlers/status.go`
- [x] T021 [US3] Exchange refresh-token secret for fresh OIDC bearer token during delegation in `pkg/resourcemanager/delegate.go`
- [x] T022 [US3] Validate `secrets.refresh_token` in FDL and store it as a Secret in the user namespace in `pkg/handlers/create.go`
- [x] T023 [US3] Enforce RBAC so only OSCAR Manager can read refresh-token Secrets (service pods must not mount) in `deploy/` or `pkg/utils/auth`
- [x] T024 [US3] Document refresh-token requirement and rotation/revocation behavior in `docs/api.md` and `docs/invoking-async.md`

**Checkpoint**: All user stories should now be independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and cross-cutting updates

- [x] T025 Update API docs for replicas and delegation in `docs/api.md`
- [x] T026 Update async invocation docs for delegated MinIO access in `docs/invoking-async.md`
- [x] T027 Document federated refresh-token requirement in `docs/fdl.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Uses federation expansion outputs from US1
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - Independent of US1/US2 but benefits from federation definitions

### Within Each User Story

- Types and helpers before handlers
- Handler wiring before documentation
- Story complete before moving to next priority

### Parallel Opportunities

- Tasks marked [P] can run in parallel

---

## Parallel Example: User Story 1

- T008 and T009 can be implemented in parallel (`pkg/handlers/create.go` vs `pkg/handlers/update.go`)

## Parallel Example: User Story 2

- T011 and T012 can be implemented in parallel (`pkg/types/replica.go` vs `pkg/handlers/replicas.go`)

## Parallel Example: User Story 3

- T016 and T020 can be implemented in parallel (`pkg/types/service.go` vs `pkg/handlers/status.go`)

---

## Implementation Strategy

- MVP scope: User Story 1 (federation expansion + deployment)
- Next increment: User Story 2 (replicas API)
- Final increment: User Story 3 (delegation policies and async MinIO access)
