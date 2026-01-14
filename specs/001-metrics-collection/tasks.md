---

description: "Task list template for feature implementation"
---

# Tasks: Metrics Collection Improvements

**Input**: Design documents from `/specs/001-metrics-collection/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Included to satisfy constitution testing requirements for Go packages.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Single project**: `pkg/`, `docs/` at repository root

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create metrics types file in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/types/metrics.go
- [ ] T002 Create metrics aggregation package files in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators.go and /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/sources.go
- [ ] T003 Create metrics handlers file in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Define source interfaces and base structs in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/sources.go
- [ ] T005 Add shared request validation helpers in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics.go
- [ ] T045 Define supported metric keys and validation in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/types/metrics.go and /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics.go
- [ ] T006 Wire metrics routes into the router in /Users/gmolto/Documents/GitHub/grycap/oscar/main.go

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Generate platform metrics summary (Priority: P1) 🎯 MVP

**Goal**: Support per-service, per-metric queries and summary outputs for a time range.

**Independent Test**: Request a specific metric for a given service and time range, and validate the response against known source data.

### Tests for User Story 1 ⚠️

- [ ] T007 [P] [US1] Add aggregator unit tests in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators_test.go
- [ ] T008 [P] [US1] Add handler tests for metric value and summary in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics_test.go
- [ ] T032 [P] [US1] Add unit tests for country attribution and summary totals in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators_test.go

### Implementation for User Story 1

- [ ] T009 [US1] Implement per-service metric value aggregation in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators.go
- [ ] T010 [US1] Implement summary aggregation in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators.go
- [ ] T011 [US1] Implement service inventory source in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/sources.go
- [ ] T012 [US1] Implement CPU/GPU usage source in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/sources.go
- [ ] T013 [US1] Implement request activity source (sync/async counts) in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/sources.go
- [ ] T028 [US1] Implement country attribution source (from request metadata) in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/sources.go
- [ ] T014 [US1] Implement metric value endpoint handler in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics.go
- [ ] T015 [US1] Implement summary endpoint handler in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics.go
- [ ] T029 [US1] Extend summary aggregation with country list/counts in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators.go

**Checkpoint**: User Story 1 is fully functional and independently testable

---

## Phase 4: User Story 2 - Drill down and export metrics (Priority: P2)

**Goal**: Provide breakdowns by service and user for a time range.

**Independent Test**: Request a per-service breakdown and verify totals match the summary for the same range.

### Tests for User Story 2 ⚠️

- [ ] T016 [US2] Add breakdown handler tests in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics_test.go
- [ ] T037 [US2] Add member/external classification tests in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators_test.go
- [ ] T038 [US2] Add CSV export handler tests in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics_test.go

### Implementation for User Story 2

- [ ] T017 [US2] Implement breakdown aggregation in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators.go
- [ ] T030 [US2] Add per-country breakdown aggregation in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators.go
- [ ] T018 [US2] Extend sources with per-user execution counts in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/sources.go
- [ ] T033 [US2] Implement user roster source lookup in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/sources.go
- [ ] T034 [US2] Add member/external classification in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators.go
- [ ] T035 [US2] Extend response types with membership classification in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/types/metrics.go
- [ ] T019 [US2] Implement breakdown endpoint handler in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics.go
- [ ] T031 [US2] Update breakdown handler to include countries in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics.go
- [ ] T036 [US2] Update breakdown handler to include membership classification in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics.go
- [ ] T039 [US2] Implement CSV export for breakdown outputs in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics.go

**Checkpoint**: User Story 2 is functional and independently testable

---

## Phase 5: User Story 3 - Validate data completeness (Priority: P3)

**Goal**: Surface missing or partial data sources in responses.

**Independent Test**: Simulate a missing source and confirm responses include completeness flags.

### Tests for User Story 3 ⚠️

- [ ] T020 [US3] Add completeness tests in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators_test.go

### Implementation for User Story 3

- [ ] T021 [US3] Add source status evaluation in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators.go
- [ ] T022 [US3] Extend response types with source status in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/types/metrics.go
- [ ] T023 [US3] Update handlers to return source status in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics.go

**Checkpoint**: User Story 3 is functional and independently testable

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation updates and cleanup

- [ ] T024 Update API documentation for metrics endpoints in /Users/gmolto/Documents/GitHub/grycap/oscar/docs/api.yaml
- [ ] T025 Update API documentation narrative in /Users/gmolto/Documents/GitHub/grycap/oscar/docs/api.md
- [ ] T040 Update metrics contract definitions for breakdown export and new fields in /Users/gmolto/Documents/GitHub/grycap/oscar/specs/001-metrics-collection/contracts/metrics.yaml
- [ ] T026 Run gofmt on new/updated files in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics.go, /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators.go, /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/sources.go, /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/types/metrics.go
- [ ] T027 Run Go tests for touched packages (e.g., ./...) from /Users/gmolto/Documents/GitHub/grycap/oscar
- [ ] T041 Add summary-vs-breakdown reconciliation tests in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators_test.go
- [ ] T042 Add country attribution percentage validation tests in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators_test.go
- [ ] T043 Add a benchmark for monthly summary aggregation in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators_test.go
- [ ] T044 Conduct stakeholder review of summary output and record notes in /Users/gmolto/Documents/GitHub/grycap/oscar/specs/001-metrics-collection/research.md

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
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - May integrate with US1 outputs
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - Depends on US1 data sources

### Within Each User Story

- Tests (if included) MUST be written and FAIL before implementation
- Aggregators before handlers
- Sources before aggregation outputs
- Story complete before moving to next priority

### Parallel Opportunities

- Setup tasks T001-T003 can run in parallel
- Foundational tasks T004-T006 can run in parallel
- Tests in a user story can run in parallel with other story tests

---

## Parallel Example: User Story 1

```bash
# Run unit tests for aggregators in parallel with handler test scaffolding:
Task: "Add aggregator unit tests in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/metrics/aggregators_test.go"
Task: "Add handler tests for metric value and summary in /Users/gmolto/Documents/GitHub/grycap/oscar/pkg/handlers/metrics_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Each story adds value without breaking previous stories
