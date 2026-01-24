# Feature Specification: Metrics Collection Improvements

**Feature Branch**: `001-metrics-collection`  
**Created**: 2026-01-13  
**Status**: Draft  
**Input**: "User description: I want to improve the metrics collection system for OSCAR. For each OSCAR cluster, we need to account for: 

 - Number of OSCAR services deployed.  
 - OSCAR service usage cycles, measured in CPU/GPU hours.
 - Number of synchronous/asynchronous requests to an OSCAR service.
 - Number of requests per user and per OSCAR service.
 - Country of origin for each user request. 
 - Names/number of the countries from which requests where received"

## Clarifications

### Session 2026-01-14

- Q: What user identifier should be used? → A: OIDC user id.
- Q: What roster source defines member vs external? → A: OIDC-authenticated requests are members; service-token executions are external.
- Q: How is the time range interpreted? → A: Inclusive start and end.
- Q: How long must data remain available for reporting? → A: At least 6 months to cover 3/6 month reports.
- Q: When should membership be included in breakdowns? → A: Only when group_by=user.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Generate platform metrics summary (Priority: P1)

As a platform reporting owner, I want a consolidated metrics summary so I can
report platform impact and usage for an OSCAR cluster.

**Why this priority**: This is the primary reporting output required by the
platform.

**Independent Test**: Request a specific metric for a given OSCAR service and
time range, then verify the value against known source data for that range.

**Acceptance Scenarios**:

1. **Given** a valid time range and OSCAR service, **When** I request a specific
   metric, **Then** the report returns only that metric and its value for the
   range.
2. **Given** no data exists for a time range, **When** I request a summary,
   **Then** the report clearly indicates zero values and data source status.

---

### User Story 2 - Drill down and export metrics (Priority: P2)

As a reporting analyst, I want to break down metrics by OSCAR service and/or user
so I can prepare detailed summaries for stakeholders.

**Why this priority**: Stakeholders need detailed breakdowns beyond the summary.

**Independent Test**: Request a breakdown by service for a fixed range and verify
that totals match the summary output.

**Acceptance Scenarios**:

1. **Given** a time range, **When** I request a per-service breakdown,
   **Then** each OSCAR service shows total executions, unique users, and
   associated countries.
2. **Given** a time range and breakdown selection, **When** I request an export,
   **Then** the system returns the breakdown in the requested export format.

---

### User Story 3 - Validate data completeness (Priority: P3)

As a platform operator, I want to see data completeness flags so I can detect
missing inputs that could skew reporting.

**Why this priority**: Incomplete inputs reduce trust in reporting outcomes.

**Independent Test**: Remove one data source for a known period and verify the
report marks the source as missing and flags affected metrics.

**Acceptance Scenarios**:

1. **Given** one or more data sources are unavailable, **When** I request a
   report, **Then** the report lists missing sources and identifies impacted
   metrics.

---

### Edge Cases

- A user has no geolocation data available (label as `unknown` and exclude from
  country attribution percentage).
- A service is deleted but still appears in historical usage data (retain
  historical metric data for at least 6 months).
- A report spans a period with partial data source coverage (surface completeness
  flags and partial counts).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow querying a specific metric for a requested time
  range and OSCAR service. If no metric is specified, the system MUST return
  all supported per-service metrics in a single response.
- **FR-002**: System MUST generate a metrics summary for a requested time range.
- **FR-003**: Summary MUST include the count of services currently deployed at
  query time (`services_count_active`). This reflects the live service
  inventory and is not time-range aware.
- **FR-004**: Summary MUST include total CPU hours and GPU hours used during the
  range.
- **FR-005**: Summary MUST include the list and count of countries for the
  range.
- **FR-006**: Summary MUST include total requests plus separate synchronous,
  asynchronous, and exposed counts for the range (`requests_count_total`,
  `requests_count_sync`, `requests_count_async`, `requests_count_exposed`).
- **FR-015**: Summary MUST include the count of services that had activity
  during the requested range, even if they were deleted later
  (`services_count_total`).
- **FR-016**: Summary MUST include the count of requests to exposed OSCAR
  services during the requested range (`requests_count_exposed`).
- **FR-007**: System MUST provide (a) executions per user (total executions per
  user) and (b) users per OSCAR service (unique users per service) in breakdown
  outputs.
- **FR-008**: System MUST classify users as project members or external based on
  authentication method (OIDC = member, OSCAR service token = external).
- **FR-009**: System MUST flag missing or incomplete data sources for the
  requested range.
- **FR-010**: System MUST attribute requests to a country when request metadata
  is available.
- **FR-011**: Breakdown outputs MUST include per-country request totals for the
  range.
- **FR-012**: System MUST support export of breakdown outputs in CSV format.
- **FR-013**: System MUST report on metric data retained for at least 6 months.
- **FR-014**: Summary outputs MUST include the list of unique users (OIDC `sub`
  values) observed in the requested time range.

### Non-Functional Requirements

- **NFR-001**: Monthly summary aggregation SHOULD complete within 5 seconds for a
  typical cluster.
- **NFR-002**: Breakdown export generation SHOULD complete within 10 seconds for
  a typical cluster.
- **NFR-003**: Metrics collection components MUST minimize resource consumption
  (CPU, RAM, and storage) and avoid cluster-wide log ingestion when only OSCAR
  manager logs are required.

### Metric Catalog (initial)

- `services-count`
- `cpu-hours`
- `gpu-hours`
- `requests-sync-per-service`
- `requests-async-per-service`
- `requests-exposed-per-service`
- `requests-per-user`
- `users-per-service`
- `countries-count`
- `countries-list`

Metric scope: `requests-sync-per-service` and `requests-async-per-service` are
per-service when used with `/system/metrics/{serviceName}`.

### Key Entities *(include if feature involves data)*

- **Service**: OSCAR service identified by the service ID and Docker image.
- **Usage Summary**: Aggregated metrics for a time range, including the current
  deployed service count and total services seen in logs.
- **Usage Breakdown**: Per-service, per-user, and per-country totals.
- **User**: Actor who triggers executions, including membership classification
  by authentication method (OIDC = member, service token = external); identified
  by OIDC user id when OIDC-authenticated.
- **Country**: Derived location label for usage attribution.

### Definitions

- **OSCAR service**: Deployed service instance.
- **Execution**: A single request (synchronous or asynchronous) processed by an
  OSCAR service during the requested time range.
- **User identifier**: OIDC user id from the IdP.
- **Time range**: Both start and end timestamps are inclusive. If omitted,
  the system defaults to the last 24 hours (end = now, start = end - 24h).
- **Metrics base path**: All metrics endpoints are served under `/system/metrics`.
- **Breakdown membership**: Include membership only when `group_by=user`.
- **OSCAR manager log scope**: Grafana Alloy MUST only collect logs from OSCAR
  manager pods (expected labels: `namespace=oscar`, `app=oscar`) unless exposed
  service request counts are enabled, in which case it MAY also collect logs
  from the ingress controller pods responsible for exposed services to keep
  resource usage minimal.
- **Prometheus metric scope**: Prometheus MUST only retain CPU/GPU usage metrics
  for OSCAR service namespaces (expected prefix: `oscar-svc`) and avoid
  cluster-wide scrape jobs to minimize storage and CPU usage.
- **Users list**: Summary outputs include the list of unique OIDC `sub` values
  observed in the requested time range.

### Assumptions

- Existing service inventory, execution logs, resource usage metrics, and OIDC
  group membership data are available and can be queried for the requested time
  range.
- Metric data is retained for at least 6 months to support 3/6 month reporting
  windows.
- Country attribution can be derived from request metadata when available.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-002**: Summary totals match breakdown totals for the same time range in
  100% of verification samples.
- **SC-003**: At least 95% of usage records include a country attribution when
  request metadata is present.
- **SC-004**: Reporting stakeholders confirm the summary meets their monthly
  reporting needs in a review session.
