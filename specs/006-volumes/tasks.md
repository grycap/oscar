# Tasks: Managed Volumes for OSCAR Services

**Input**: Design documents from `specs/006-volumes/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/volumes-services.openapi.yaml

**Tests**: Include Go test tasks for each story because the repository constitution requires tests for touched Go packages.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare the codebase entry points and shared files for the managed-volumes feature.

- [X] T001 Add `/system/volumes` route wiring in main.go
- [X] T002 [P] Create volume handler scaffolding in pkg/handlers/volumes.go
- [X] T003 [P] Create managed-volume resource helper scaffolding in pkg/backends/resources/volume.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish the shared data model, validation, naming, and status helpers required by all user stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T004 Replace the legacy service storage schema with `volume` config and status types in pkg/types/service.go
- [X] T005 [P] Update service serialization, naming, and pod-mount tests for `volume` in pkg/types/service_test.go
- [X] T006 Implement shared volume validation and defaulting helpers in pkg/handlers/create.go
- [X] T007 [P] Enforce immutable service volume updates in pkg/handlers/update.go
- [X] T008 [P] Implement shared PVC naming, metadata, and namespace resolution helpers in pkg/backends/resources/volume.go
- [X] T009 [P] Add foundational resource-helper tests in pkg/backends/resources/volume_test.go
- [X] T010 [P] Implement shared volume status resolution helpers in pkg/handlers/read.go and pkg/handlers/list.go
- [X] T011 [P] Add foundational handler validation and status tests in pkg/handlers/create_test.go, pkg/handlers/update_test.go, pkg/handlers/read_test.go, and pkg/handlers/list_test.go

**Checkpoint**: Foundation ready; service and volume workflows can now be implemented.

---

## Phase 3: User Story 1 - Create or Attach a Volume During Service Deployment (Priority: P1) 🎯 MVP

**Goal**: Let a service create a managed volume with an auto-generated or explicit name, or mount an existing named volume during deployment.

**Independent Test**: Submit a service definition with a `volume` block that either creates a new volume or references an existing volume by name, deploy the service, write test files, restart the service, and verify files remain available at the configured mount path.

### Tests for User Story 1

- [X] T012 [P] [US1] Add service-create tests for new-volume and mount-existing flows in pkg/handlers/create_test.go
- [X] T013 [P] [US1] Add backend tests for service volume provisioning and mount wiring in pkg/backends/k8s_test.go and pkg/backends/knative_test.go

### Implementation for User Story 1

- [X] T014 [US1] Implement service create-path volume creation and attachment logic in pkg/handlers/create.go
- [X] T015 [P] [US1] Implement service pod-spec volume mount wiring in pkg/types/service.go
- [X] T016 [US1] Implement backend create/update/delete handling for service-attached volumes in pkg/backends/k8s.go and pkg/backends/knative.go
- [X] T017 [US1] Implement service-facing volume lookup and owner metadata handling in pkg/backends/resources/volume.go

**Checkpoint**: User Story 1 should be independently deployable and preserve data across restart/redeploy.

---

## Phase 4: User Story 2 - Manage Volumes Through a Dedicated API (Priority: P1)

**Goal**: Provide `/system/volumes` list, create, read, and delete operations scoped to the caller namespace.

**Independent Test**: Create a volume through `/system/volumes`, verify it appears in list and read operations for the same user namespace, and delete it successfully when it has no attached services.

### Tests for User Story 2

- [X] T018 [P] [US2] Add handler tests for `/system/volumes` CRUD and namespace isolation in pkg/handlers/volumes_test.go
- [X] T019 [P] [US2] Add resource-layer tests for volume read/list metadata and attachment enumeration in pkg/backends/resources/volume_test.go

### Implementation for User Story 2

- [X] T020 [US2] Implement `/system/volumes` create, list, read, and delete handlers in pkg/handlers/volumes.go
- [X] T021 [P] [US2] Implement managed-volume CRUD and attachment-discovery helpers in pkg/backends/resources/volume.go
- [X] T022 [US2] Register Swagger annotations for `/system/volumes` in pkg/handlers/volumes.go and main.go

**Checkpoint**: User Story 2 should expose namespace-scoped volume management without requiring service deployment.

---

## Phase 5: User Story 3 - Control Volume Lifecycle Policy (Priority: P2)

**Goal**: Respect `volume.lifecycle_policy` so service-created volumes are retained or deleted when the creating service is removed.

**Independent Test**: Deploy one service with `volume.lifecycle_policy: retain` and another with `volume.lifecycle_policy: delete`, remove both services, and verify only the retained volume remains available for later reuse.

### Tests for User Story 3

- [X] T023 [P] [US3] Add lifecycle-policy service deletion tests in pkg/handlers/delete_test.go and pkg/backends/k8s_test.go
- [X] T024 [P] [US3] Add volume-delete guard tests for attached volumes in pkg/handlers/volumes_test.go and pkg/backends/resources/volume_test.go

### Implementation for User Story 3

- [X] T025 [US3] Implement `retain` and `delete` lifecycle-policy validation and defaults in pkg/handlers/create.go and pkg/types/service.go
- [X] T026 [US3] Implement retain/delete cleanup behavior for service-created volumes in pkg/backends/k8s.go and pkg/backends/knative.go
- [X] T027 [US3] Implement explicit delete blocking for attached volumes in pkg/handlers/volumes.go and pkg/backends/resources/volume.go

**Checkpoint**: User Story 3 should preserve retained volumes and clean up delete-policy volumes correctly.

---

## Phase 6: User Story 4 - Keep Existing Services Compatible (Priority: P2)

**Goal**: Preserve current behavior for services that do not use `volume` and avoid regressions in existing mount and storage flows.

**Independent Test**: Deploy a service definition without volume configuration and verify behavior matches pre-feature deployments.

### Tests for User Story 4

- [X] T028 [P] [US4] Add regression tests for legacy services without `volume` in pkg/handlers/update_test.go and pkg/backends/knative_test.go
- [X] T029 [P] [US4] Add regression tests for existing `mount`, `input`, and `output` flows in pkg/handlers/create_test.go and pkg/backends/k8s_test.go

### Implementation for User Story 4

- [X] T030 [US4] Preserve legacy no-volume service defaults in pkg/handlers/create.go, pkg/handlers/update.go, and pkg/types/service.go
- [X] T031 [US4] Keep read/list responses stable for services without volumes in pkg/handlers/read.go and pkg/handlers/list.go

**Checkpoint**: Legacy service definitions should remain independently deployable with no behavior regression.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Finalize docs, generated API artifacts, and validation across all stories.

- [X] T032 [P] Update managed-volume documentation in docs/fdl.md, docs/api.md, and docs/additional-config.md
- [X] T033 [P] Regenerate Swagger artifacts for the volume API in pkg/apidocs/docs.go, pkg/apidocs/swagger.json, and pkg/apidocs/swagger.yaml
- [X] T034 Run `go test ./pkg/types ./pkg/handlers ./pkg/backends/...` and record outcomes in specs/006-volumes/quickstart.md
- [X] T035 Run `go generate ./...` and record API-doc regeneration outcomes in specs/006-volumes/quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately.
- **Foundational (Phase 2)**: Depends on Phase 1; blocks all user stories.
- **User Story 1 (Phase 3)**: Depends on Phase 2.
- **User Story 2 (Phase 4)**: Depends on Phase 2 and can proceed in parallel with User Story 1 after the foundation is ready.
- **User Story 3 (Phase 5)**: Depends on User Story 1 and User Story 2 because lifecycle behavior builds on service attachments and volume CRUD.
- **User Story 4 (Phase 6)**: Depends on Phases 3 through 5 because regression checks must cover the final changed flows.
- **Polish (Phase 7)**: Depends on all user stories being complete.

### User Story Dependencies

- **User Story 1 (P1)**: MVP story; no dependency on other stories after the foundation is ready.
- **User Story 2 (P1)**: Independent API story after the foundation is ready.
- **User Story 3 (P2)**: Depends on the volume-creation and volume-API behavior from US1 and US2.
- **User Story 4 (P2)**: Validates compatibility after the main volume changes are in place.

### Parallel Opportunities

- `T002` and `T003` can run in parallel after `T001`.
- `T005`, `T007`, `T008`, `T009`, `T010`, and `T011` can run in parallel once `T004` starts the shared model transition.
- `T012` and `T013` can run in parallel for US1 tests.
- `T018` and `T019` can run in parallel for US2 tests.
- `T023` and `T024` can run in parallel for US3 tests.
- `T028` and `T029` can run in parallel for US4 regression coverage.
- `T032` and `T033` can run in parallel during polish.

---

## Parallel Example: User Story 1

```bash
Task: "Add service-create tests for new-volume and mount-existing flows in pkg/handlers/create_test.go"
Task: "Add backend tests for service volume provisioning and mount wiring in pkg/backends/k8s_test.go and pkg/backends/knative_test.go"
```

## Parallel Example: User Story 2

```bash
Task: "Add handler tests for /system/volumes CRUD and namespace isolation in pkg/handlers/volumes_test.go"
Task: "Add resource-layer tests for volume read/list metadata and attachment enumeration in pkg/backends/resources/volume_test.go"
```

## Parallel Example: User Story 3

```bash
Task: "Add lifecycle-policy service deletion tests in pkg/handlers/delete_test.go and pkg/backends/k8s_test.go"
Task: "Add volume-delete guard tests for attached volumes in pkg/handlers/volumes_test.go and pkg/backends/resources/volume_test.go"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Validate the service deployment, mount, and persistence flow independently.

### Incremental Delivery

1. Deliver US1 to support service-time volume creation and attachment.
2. Deliver US2 to expose reusable volume management through `/system/volumes`.
3. Deliver US3 to add `retain` and `delete` lifecycle semantics.
4. Deliver US4 regression coverage and compatibility hardening.
5. Finish with docs, Swagger regeneration, and final validation.

### Parallel Team Strategy

1. One developer completes Phase 1 and coordinates shared model changes in Phase 2.
2. After Phase 2, one developer can take US1 while another takes US2.
3. After US1 and US2 land, US3 and US4 can proceed with focused lifecycle and regression work.

---

## Notes

- All task lines follow the required checklist format.
- User story tasks include `[US#]` labels for traceability.
- Tests are included because Go package changes require verification in this repository.
- The suggested MVP scope is User Story 1 after Setup and Foundational phases.
