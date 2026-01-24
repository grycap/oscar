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

- [X] T001 Create metrics types file in pkg/types/metrics.go
- [X] T002 Create metrics aggregation package files in pkg/metrics/aggregators.go and pkg/metrics/sources.go
- [X] T003 Create metrics handlers file in pkg/handlers/metrics.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 Define source interfaces and base structs in pkg/metrics/sources.go
- [X] T005 Add shared request validation helpers in pkg/handlers/metrics.go
- [X] T045 Define supported metric keys and validation in pkg/types/metrics.go and pkg/handlers/metrics.go
- [X] T006 Wire metrics routes into the router in main.go

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Generate platform metrics summary (Priority: P1) 🎯 MVP

**Goal**: Support per-service, per-metric queries and summary outputs for a time range.

**Independent Test**: Request a specific metric for a given service and time range, and validate the response against known source data.

### Tests for User Story 1 ⚠️

- [X] T007 [P] [US1] Add aggregator unit tests in pkg/metrics/aggregators_test.go
- [X] T008 [P] [US1] Add handler tests for metric value and summary in pkg/handlers/metrics_test.go
- [X] T032 [P] [US1] Add unit tests for country attribution and summary totals in pkg/metrics/aggregators_test.go

### Implementation for User Story 1

- [X] T009 [US1] Implement per-service metric value aggregation in pkg/metrics/aggregators.go
- [X] T010 [US1] Implement summary aggregation in pkg/metrics/aggregators.go
- [X] T011 [US1] Implement service inventory source in pkg/metrics/sources.go
- [X] T012 [US1] Implement CPU/GPU usage source in pkg/metrics/sources.go
- [X] T048 [US1] Add Prometheus config fields and env vars in pkg/types/config.go
- [X] T049 [US1] Implement Prometheus usage metrics source in pkg/metrics/sources.go
- [X] T050 [US1] Wire Prometheus usage source when PROMETHEUS_URL is set in pkg/metrics/sources.go and main.go
- [X] T013 [US1] Implement request activity source (sync/async counts) in pkg/metrics/sources.go
- [X] T028 [US1] Implement country attribution source (from request metadata) in pkg/metrics/sources.go
- [X] T053 [US1] Plan Loki + Grafana Alloy deployment for durable request logs (retention >= 6 months) in specs/001-metrics-collection/research.md
- [X] T054 [US1] Add Loki query configuration (base URL, query templates) in pkg/types/config.go
- [X] T055 [US1] Implement Loki request log source (LogQL query + parsing) in pkg/metrics/sources.go
- [X] T056 [US1] Wire Loki request log source when configured (fallback to pod logs) in pkg/metrics/sources.go and main.go
- [X] T057 [US1] Add request log source tests for Loki queries in pkg/metrics/aggregators_test.go
- [X] T072 [US1] Add exposed request log source (ingress controller logs) in pkg/metrics/sources.go
- [X] T073 [US1] Add requests_count_exposed to summary totals in pkg/types/metrics.go and pkg/metrics/aggregators.go
- [ ] T047 [US1] Verify 6-month retention guarantees for all data sources and document retention ownership in specs/001-metrics-collection/spec.md
- [X] T014 [US1] Implement metric value endpoint handler in pkg/handlers/metrics.go
- [X] T015 [US1] Implement summary endpoint handler in pkg/handlers/metrics.go
- [X] T029 [US1] Extend summary aggregation with country list/counts in pkg/metrics/aggregators.go

**Checkpoint**: User Story 1 is fully functional and independently testable

---

## Phase 4: User Story 2 - Drill down and export metrics (Priority: P2)

**Goal**: Provide breakdowns by service and user for a time range.

**Independent Test**: Request a per-service breakdown and verify totals match the summary for the same range.

### Tests for User Story 2 ⚠️

- [X] T016 [US2] Add breakdown handler tests in pkg/handlers/metrics_test.go
- [X] T037 [US2] Add member/external classification tests in pkg/metrics/aggregators_test.go
- [X] T038 [US2] Add CSV export handler tests in pkg/handlers/metrics_test.go

### Implementation for User Story 2

- [X] T017 [US2] Implement breakdown aggregation in pkg/metrics/aggregators.go
- [X] T030 [US2] Add per-country breakdown aggregation in pkg/metrics/aggregators.go
- [X] T018 [US2] Extend sources with per-user execution counts in pkg/metrics/sources.go
- [X] T033 [US2] Implement user roster source lookup in pkg/metrics/sources.go
- [X] T034 [US2] Add member/external classification in pkg/metrics/aggregators.go
- [X] T035 [US2] Extend response types with membership classification in pkg/types/metrics.go
- [X] T019 [US2] Implement breakdown endpoint handler in pkg/handlers/metrics.go
- [X] T031 [US2] Update breakdown handler to include countries in pkg/handlers/metrics.go
- [X] T036 [US2] Update breakdown handler to include membership classification in pkg/handlers/metrics.go
- [X] T039 [US2] Implement CSV export for breakdown outputs in pkg/handlers/metrics.go

**Checkpoint**: User Story 2 is functional and independently testable

---

## Phase 5: User Story 3 - Validate data completeness (Priority: P3)

**Goal**: Surface missing or partial data sources in responses.

**Independent Test**: Simulate a missing source and confirm responses include completeness flags.

### Tests for User Story 3 ⚠️

- [X] T020 [US3] Add completeness tests in pkg/metrics/aggregators_test.go

### Implementation for User Story 3

- [X] T021 [US3] Add source status evaluation in pkg/metrics/aggregators.go
- [X] T022 [US3] Extend response types with source status in pkg/types/metrics.go
- [X] T023 [US3] Update handlers to return source status in pkg/handlers/metrics.go

**Checkpoint**: User Story 3 is functional and independently testable

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation updates and cleanup

- [X] T024 Update API documentation for metrics endpoints in docs/api.yaml (removed; now swaggo)
- [X] T025 Update API documentation narrative in docs/api.md
- [X] T051 Document Prometheus metrics configuration in docs/api.md
- [ ] T046 Validate documentation changes with mkdocs serve (or record why infeasible) in repo root
- [X] T040 Update metrics contract definitions for breakdown export and new fields in specs/001-metrics-collection/contracts/metrics.yaml
- [X] T026 Run gofmt on new/updated files in pkg/handlers/metrics.go, pkg/metrics/aggregators.go, pkg/metrics/sources.go, pkg/types/metrics.go
- [ ] T027 Run Go tests for touched packages (e.g., ./...) from repo root
- [X] T041 Add summary-vs-breakdown reconciliation tests in pkg/metrics/aggregators_test.go
- [X] T042 Add country attribution percentage validation tests in pkg/metrics/aggregators_test.go
- [X] T043 Add a benchmark for monthly summary aggregation in pkg/metrics/aggregators_test.go
- [ ] T052 Add benchmark for breakdown CSV export generation in pkg/handlers/metrics_test.go
- [X] T058 Document Loki + Alloy deployment steps in docs/local-testing.md
- [X] T059 Document Loki configuration env vars in docs/api.md
- [ ] T044 Conduct stakeholder review of summary output and record notes in specs/001-metrics-collection/research.md
- [X] T060 Update Alloy log collection to only include OSCAR manager pods in docs/snippets/alloy-values.kind.yaml
- [X] T074 Update summary contracts/spec/data-model for exposed requests in specs/001-metrics-collection/spec.md, specs/001-metrics-collection/data-model.md, and specs/001-metrics-collection/contracts/metrics.yaml
- [X] T075 Update Alloy log collection to include ingress-nginx controller logs in docs/snippets/alloy-values.kind.yaml
- [X] T076 Allow optional start/end with 24h default range in pkg/handlers/metrics.go and update specs/001-metrics-collection/spec.md
- [X] T077 Rename metrics endpoints to /system/metrics and /system/metrics/{serviceName} in main.go, handlers, and specs/docs/tests
- [X] T078 Allow /system/metrics/{serviceName} without metric to return all per-service metrics in pkg/handlers/metrics.go and docs/specs/contracts
- [X] T061 Update local testing docs to note OSCAR-only log filtering in docs/local-testing.md
- [X] T062 Apply Alloy configuration update in the local kind cluster (helm upgrade) from repo root
- [X] T063 Add minimal Prometheus values file to collect only OSCAR CPU/GPU metrics in docs/snippets/prometheus-values.kind.yaml
- [X] T064 Update local testing docs to use minimal Prometheus values in docs/local-testing.md
- [X] T065 Apply minimal Prometheus configuration in the local kind cluster (helm upgrade) from repo root
- [X] T066 Add Grafana values and dashboard snippets in docs/snippets/grafana-values.kind.yaml and docs/snippets/oscar-metrics-dashboard.json
- [X] T067 Document Grafana deployment and dashboard notes in specs/001-metrics-collection/monitoring-docs.md
- [X] T068 Apply Grafana deployment in the local kind cluster (helm upgrade) from repo root
- [X] T069 Update Alloy config to enrich logs with GeoIP labels in docs/snippets/alloy-values.kind.yaml
- [X] T070 Populate request country from Loki stream labels in pkg/metrics/sources.go
- [X] T071 Document GeoIP enrichment requirements in specs/001-metrics-collection/monitoring-docs.md

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
Task: "Add aggregator unit tests in pkg/metrics/aggregators_test.go"
Task: "Add handler tests for metric value and summary in pkg/handlers/metrics_test.go"
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
