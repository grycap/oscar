# Quickstart: Metrics Collection Improvements

## Goal

Generate metrics summaries, per-service metric values, and breakdowns for a time
range to support reporting.

## Prerequisites

- Access to the OSCAR manager API with appropriate credentials.
- A time range expressed as ISO 8601 timestamps.

## Example: Single service metric value

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "https://YOUR_OSCAR_MANAGER/system/metrics/value?service_id=service-123&metric=cpu-hours&start=2026-01-01T00:00:00Z&end=2026-01-31T23:59:59Z"
```

## Example: Summary report

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "https://YOUR_OSCAR_MANAGER/system/metrics/summary?start=2026-01-01T00:00:00Z&end=2026-01-31T23:59:59Z"
```

## Example: Breakdown by service

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "https://YOUR_OSCAR_MANAGER/system/metrics/breakdown?start=2026-01-01T00:00:00Z&end=2026-01-31T23:59:59Z&group_by=service"
```

## Example: Breakdown export (CSV)

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "https://YOUR_OSCAR_MANAGER/system/metrics/breakdown?start=2026-01-01T00:00:00Z&end=2026-01-31T23:59:59Z&group_by=service&format=csv"
```

## Expected Output

- Metric value: per-service value plus completeness status for the requested
  metric.
- Summary: totals for services, CPU/GPU hours, request counts, countries, users,
  plus source completeness status.
- Breakdown: per-service, per-user, or per-country executions, unique users,
  membership classification, and per-country request totals.
