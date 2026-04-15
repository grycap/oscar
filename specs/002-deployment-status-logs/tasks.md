---

description: "Task list template for feature implementation"
---

# Tasks: Service Deployment Visibility

**Input**: Design documents from `/specs/002-deployment-status-logs/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Included to satisfy the repository constitution for touched Go packages.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Single project**: `pkg/`, `docs/`, and `specs/` at repository root

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the new deployment-visibility files and align the documentation touchpoints before implementation begins.

- [X] T001 [P] Create deployment visibility response type files in pkg/types/deployment.go and pkg/types/deployment_test.go
- [X] T002 [P] Create deployment visibility handler files in pkg/handlers/deployment.go and pkg/handlers/deployment_test.go
- [X] T003 [P] Align deployment visibility design references in specs/002-deployment-status-logs/contracts/deployment-visibility.openapi.yaml, specs/002-deployment-status-logs/quickstart.md, and docs/api.md

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core plumbing that MUST be complete before ANY user story can be implemented.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 Define ServiceDeploymentStatus, DeploymentLogStream, and DeploymentLogEntry types in pkg/types/deployment.go
- [X] T005 [P] Add shared deployment request parsing, service authorization reuse, and namespace-resolution helpers in pkg/handlers/deployment.go
- [X] T006 [P] Add Kubernetes deployment and pod inspection helpers for deployment visibility in pkg/backends/k8s.go and pkg/backends/k8s_test.go
- [X] T007 [P] Add Knative or runtime inspection helpers plus unavailable fallback utilities in pkg/backends/knative.go and pkg/backends/knative_test.go
- [X] T008 Wire deployment visibility routes into main.go
- [X] T009 Add Swagger annotations for the deployment summary and deployment logs endpoints in pkg/handlers/deployment.go

**Checkpoint**: Foundation ready; user story implementation can now proceed.

---

## Phase 3: User Story 1 - Inspect Deployment Health (Priority: P1) 🎯 MVP

**Goal**: Let an authorized user retrieve a current service-level deployment summary and understand whether the service is ready, pending, degraded, failed, or unavailable.

**Independent Test**: Request `/system/services/{serviceName}/deployment` for services in ready, pending, degraded, failed, and unavailable states and verify that state, reason, last transition time, and aggregate counts match the runtime evidence.

### Tests for User Story 1 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T010 [P] [US1] Add deployment summary type tests for state validation and aggregate counts in pkg/types/deployment_test.go
- [X] T011 [P] [US1] Add handler tests for GET /system/services/:serviceName/deployment across ready, pending, degraded, failed, and unavailable cases in pkg/handlers/deployment_test.go

### Implementation for User Story 1

- [X] T012 [US1] Implement service-level deployment summary lookup for exposed-service Kubernetes resources in pkg/handlers/deployment.go and pkg/backends/k8s.go
- [X] T013 [US1] Implement service-level deployment summary lookup for backend-managed runtimes in pkg/handlers/deployment.go and pkg/backends/knative.go
- [X] T014 [US1] Implement aggregate affected-instance counting, reason selection, and last-transition mapping in pkg/types/deployment.go and pkg/handlers/deployment.go
- [X] T015 [US1] Implement the GET /system/services/:serviceName/deployment handler in pkg/handlers/deployment.go
- [X] T016 [US1] Update deployment summary contract details in specs/002-deployment-status-logs/contracts/deployment-visibility.openapi.yaml and Swagger comments in pkg/handlers/deployment.go

**Checkpoint**: User Story 1 should now be independently functional and testable.

---

## Phase 4: User Story 2 - Review Deployment Logs (Priority: P2)

**Goal**: Let an authorized user retrieve current or recent service-level deployment logs, including explicit messages when logs are not available.

**Independent Test**: Request `/system/services/{serviceName}/deployment/logs` for a service with recent startup activity, a service with no logs yet, and a service the caller cannot access; verify availability flags, ordered entries, and access behavior.

### Tests for User Story 2 ⚠️

- [X] T017 [P] [US2] Add deployment log response type tests for ordered entries, timestamps, and unavailable messages in pkg/types/deployment_test.go
- [X] T018 [P] [US2] Add handler tests for GET /system/services/:serviceName/deployment/logs across current-log, no-log, and unauthorized scenarios in pkg/handlers/deployment_test.go

### Implementation for User Story 2

- [X] T019 [US2] Implement current runtime deployment log retrieval with timestamps and tailLines support in pkg/handlers/deployment.go and pkg/backends/k8s.go
- [X] T020 [US2] Implement service-level deployment log response shaping, ordering, and explicit availability messages in pkg/types/deployment.go and pkg/handlers/deployment.go
- [X] T021 [US2] Implement the GET /system/services/:serviceName/deployment/logs handler in pkg/handlers/deployment.go
- [X] T022 [US2] Update deployment log contract details in specs/002-deployment-status-logs/contracts/deployment-visibility.openapi.yaml and Swagger comments in pkg/handlers/deployment.go

**Checkpoint**: User Story 2 should now be independently functional and testable.

---

## Phase 5: User Story 3 - Diagnose Failures Faster (Priority: P3)

**Goal**: Make deployment summaries and deployment logs reinforce each other so users can identify likely failure causes without inspecting cluster internals directly.

**Independent Test**: Reproduce degraded, failed, and unavailable deployment scenarios, review both deployment endpoints together, and verify that the summary reason and recent log evidence are consistent enough to identify the likely next troubleshooting step.

### Tests for User Story 3 ⚠️

- [X] T023 [P] [US3] Add handler tests for degraded summaries, reason-to-log consistency, and unavailable-with-last-attempt-log fallback in pkg/handlers/deployment_test.go

### Implementation for User Story 3

- [X] T024 [US3] Implement service-summary partial-failure classification and human-readable diagnosis mapping in pkg/handlers/deployment.go and pkg/types/deployment.go
- [X] T025 [US3] Implement cheap last-attempt deployment log fallback without changing unavailable status semantics in pkg/handlers/deployment.go, pkg/backends/k8s.go, and pkg/backends/knative.go
- [X] T026 [US3] Normalize unavailable behavior across exposed, sync, and async services in pkg/handlers/deployment.go, pkg/backends/k8s.go, and pkg/backends/knative.go

**Checkpoint**: User Story 3 should now be independently functional and testable.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finish documentation, regenerate derived artifacts, and validate the feature end to end.

- [X] T027 Update deployment visibility documentation in docs/api.md
- [X] T028 Regenerate Swagger artifacts in pkg/apidocs/docs.go, pkg/apidocs/swagger.json, and pkg/apidocs/swagger.yaml from repo root
- [X] T029 [P] Run gofmt on pkg/handlers/deployment.go, pkg/handlers/deployment_test.go, pkg/types/deployment.go, pkg/types/deployment_test.go, pkg/backends/k8s.go, and pkg/backends/knative.go
- [X] T030 Run targeted Go tests for pkg/handlers, pkg/types, and pkg/backends from repo root
- [X] T031 Validate the deployment visibility quickstart scenarios against specs/002-deployment-status-logs/quickstart.md
- [X] T032 Validate documentation rendering with mkdocs serve using mkdocs.yml and docs/api.md, or record why validation was infeasible in specs/002-deployment-status-logs/quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately.
- **Foundational (Phase 2)**: Depends on Setup completion and blocks all user stories.
- **User Stories (Phases 3-5)**: Depend on Foundational completion.
- **Polish (Phase 6)**: Depends on all desired user stories being complete.

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Phase 2 and is the MVP slice.
- **User Story 2 (P2)**: Starts after Phase 2 and reuses the deployment visibility types and helpers introduced for US1.
- **User Story 3 (P3)**: Starts after US1 and US2 because it depends on both deployment summaries and deployment log behavior being present.

### Within Each User Story

- Tests MUST be written and fail before implementation.
- Shared types and lookup helpers come before endpoint behavior.
- Endpoint wiring and contract updates follow the core behavior.
- Each story should be validated independently before moving on.

### Parallel Opportunities

- Setup tasks `T001`-`T003` can run in parallel.
- Foundational backend helper tasks `T006` and `T007` can run in parallel after `T004` and `T005`.
- US1 tests `T010` and `T011` can run in parallel.
- US2 tests `T017` and `T018` can run in parallel.
- Formatting `T029` can run in parallel with docs updates once implementation is complete.

---

## Parallel Example: User Story 1

```bash
Task: "Add deployment summary type tests in pkg/types/deployment_test.go"
Task: "Add deployment summary handler tests in pkg/handlers/deployment_test.go"
```

---

## Parallel Example: User Story 2

```bash
Task: "Add deployment log response type tests in pkg/types/deployment_test.go"
Task: "Add deployment log handler tests in pkg/handlers/deployment_test.go"
```

---

## Parallel Example: User Story 3

```bash
Task: "Implement last-attempt deployment log fallback in pkg/handlers/deployment.go, pkg/backends/k8s.go, and pkg/backends/knative.go"
Task: "Update deployment visibility documentation in docs/api.md after endpoint behavior is stable"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Validate deployment summaries independently before expanding scope.

### Incremental Delivery

1. Deliver User Story 1 to provide immediate deployment-health visibility.
2. Add User Story 2 to expose service-level deployment logs without changing job-log behavior.
3. Add User Story 3 to improve diagnosis quality and unavailable fallback behavior.
4. Finish with documentation, Swagger regeneration, formatting, and validation.

### Parallel Team Strategy

1. One contributor completes Setup and Foundational tasks.
2. After Phase 2, one contributor can finish US1 while another prepares US2 tests.
3. US3 begins once US1 and US2 behaviors are stable enough to correlate summary and logs.

---

## Notes

- [P] tasks touch different files or can proceed independently after prerequisites.
- [US1] delivers the recommended MVP scope.
- Keep the new endpoints additive under `/system/services/:serviceName`.
- Do not alter existing job-log routes or dashboard behavior while implementing this feature.
