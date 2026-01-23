# Data Model: Metrics Collection Improvements

## Entities

### Service

- **Fields**: service_id, service_name, docker_image, created_at
- **Validation**: docker_image required; service_id required; service_name
  optional if not available
- **Relationships**: 1..n UsageBreakdownEntry (as service)

### User

- **Fields**: user_id, display_name, classification (project_member|external)
- **Validation**: user_id required; classification required
- **Relationships**: 1..n UsageBreakdownEntry (as user)

### Country

- **Fields**: country_code, country_name
- **Validation**: country_code optional if unknown; country_name optional if
  unknown
- **Relationships**: 1..n UsageBreakdownEntry (as country)

### MetricSourceStatus

- **Fields**: source_name, status (ok|missing|partial), last_updated, notes
- **Validation**: source_name required; status required
- **Relationships**: 1..n UsageSummary (as completeness metadata)

### MetricValue

- **Fields**: metric_key, service_id, start_time, end_time, value, unit
- **Validation**: metric_key required; service_id required; start_time/end_time
  required; value numeric
- **Relationships**: references MetricSourceStatus (as completeness metadata)

### UsageSummary

- **Fields**: start_time, end_time, generated_at, services_count,
  cpu_hours_total, gpu_hours_total, request_count_total,
  request_count_sync, request_count_async, countries_count,
  users_count, users, completeness
- **Validation**: start_time/end_time required; generated_at required; counts
  >= 0
- **Relationships**: has many UsageBreakdownEntry; references MetricSourceStatus

### UsageBreakdownEntry

- **Fields**: start_time, end_time, service_id, user_id, country_code,
  executions_count, unique_users_count
- **Validation**: start_time/end_time required; executions_count >= 0
- **Relationships**: belongs to Service, User, Country

## State Transitions

- MetricSourceStatus: ok -> partial -> missing (and back to ok) based on source
  availability for the requested range.
- UsageSummary: generated_at updated on each report request; counts recalculated
  from sources for the requested range.
- MetricValue: generated on-demand per query; values recalculated from sources
  for the requested range and service.
