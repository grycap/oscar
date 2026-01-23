# Research: Metrics Collection Improvements

## Decision 1: Data sources for metrics

**Decision**: Use existing OSCAR service inventory, request logs, and
cluster resource usage metrics to populate the report; derive country data from
available request metadata and existing geolocation outputs (if present in the
logging stack).

**Rationale**: These sources already exist in the platform and can be queried
without introducing new dependencies or storage.

**Alternatives considered**:

- Introduce a new metrics database (rejected: new dependency and storage scope).
- Rely solely on Prometheus for all metrics (rejected: logs and user metadata
  are required for per-user and geo summaries).

### Metric Source Map

| Metric | Primary Source | APIs / Queries | Retention / Expiration | Details |
|--------|----------------|----------------|----------------------|---------|
| Services hosted (unique) | Service inventory | OSCAR manager service list API (existing service registry endpoints) | N/A (current state) | Use service registry to list active services; unique by service ID and Docker image. |
| CPU hours | Cluster usage metrics (Prometheus) | Prometheus query API (HTTP) over container CPU usage timeseries filtered by service label | TBD: depends on Prometheus retention config (e.g., `--storage.tsdb.retention.time`) | Sum CPU usage across all containers for a service over the requested range; multiple replicas/containers per service must be aggregated. |
| GPU hours | Cluster usage metrics (Prometheus) | Prometheus query API (HTTP) over container GPU usage timeseries filtered by service label | TBD: depends on Prometheus retention config (e.g., `--storage.tsdb.retention.time`) | Sum GPU usage across all containers for a service over the requested range; multiple replicas/containers per service must be aggregated. |
| Requests processed (sync/async) | Request logs | OSCAR logs source (existing log storage/query mechanism) filtered by service ID and request type | TBD: log retention policy in current logging stack | Count synchronous and asynchronous invocations from OSCAR request logs. |
| Requests per user | Request logs + user roster | OSCAR logs source grouped by user ID; user roster API or config | TBD: log retention policy + user roster retention | Aggregate log entries by user ID; enrich with user classification. |
| Requests per service | Request logs | OSCAR logs source grouped by service ID | TBD: log retention policy | Aggregate log entries by service ID across all containers. |
| Users per service | Request logs + user roster | OSCAR logs source grouped by service ID + user roster API/config | TBD: log retention policy + user roster retention | Aggregate unique users by service ID; enrich with classification. |
| Countries reached | Request metadata + geolocation | OSCAR logs/metadata source + geolocation output (if configured) | TBD: log retention policy; geolocation update cadence | Derive country from request metadata when present, using existing geolocation outputs for lookup. |
| Country per user | Request metadata + geolocation | OSCAR logs/metadata source + geolocation output (if configured) | TBD: log retention policy; geolocation update cadence | Map user requests to country attribution based on request metadata. |
| User classification | Authentication method | OSCAR request metadata (OIDC vs service token) | N/A | Identify project members vs external users based on auth method. |

### Source Details and Assumptions

- **Prometheus retention**: The CPU/GPU usage history depends on the cluster's
  Prometheus retention settings. Confirm the configured retention time and
  storage policies for the OSCAR deployment.
- **Prometheus scrape interval**: Short-lived executions can be under-sampled
  when scrapes are infrequent (for example, 60s). CPU usage metrics derived from
  `increase()` over short windows may appear near zero if the workload does not
  overlap a scrape or consumes little CPU.
- **cgroup accounting accuracy**: cgroup CPU accounting can capture precise CPU
  time for short runs (microsecond/nanosecond resolution), but it must be read
  at execution start/end to avoid scrape-interval gaps. Prometheus/cAdvisor
  still sample at the configured scrape interval unless a higher-frequency
  collector is used.
- **Log retention**: Request-based metrics rely on the current logging stack's
  retention policy and query access. Confirm where logs are stored and the
  retention window for production clusters.
- **Log format**: Request logs are obtained from the GIN logger using the
  `[GIN-EXECUTIONS-LOGGER]` format, for example:

  ```text
  2024-10-10T09:34:35.919996606Z stderr F [GIN-EXECUTIONS-LOGGER] 2024/10/10 - 09:34:35 | 201 |  1.455744552s | 79.117.163.142 | POST    /job/yolo | 62bb11b40398f73778b66f344d282242debb8ee3ebb106717a123ca213162926@egi.eu
  ```
  Parsed fields (pipe-delimited) map to: timestamp, status code, latency,
  client IP, method + path, user identifier.
- **GeoIP availability**: Repository search did not find GeoIP/GoAccess usage.
  Confirm whether the logging stack already produces geolocation outputs or
  whether GeoIP lookup is available externally for request IPs.
- **Multi-container services**: OSCAR services may scale to multiple pods or
  containers per service ID. All resource usage and request counts must be
  aggregated across all containers that match the service identifier.

### Prometheus storage estimate (6 months)

**Method**:  
Storage ~= `active_series * samples_per_series * bytes_per_sample`, plus index
overhead (plan for ~20% overhead). For a 30s scrape interval:

- Samples per day: 2,880  
- Samples per 6 months (~180 days): 518,400

**Rule of thumb** (compressed TSDB data): 1.5-2.5 bytes per sample, plus ~20%
overhead for index/metadata.

**Example estimates** (30s scrape):

- 10k series: ~8-13 GB for 6 months  
- 50k series: ~40-65 GB for 6 months  
- 100k series: ~80-130 GB for 6 months

**Action**: Confirm `active_series` and `scrape_interval` in the current
Prometheus config to refine the estimate and verify storage capacity for a
6-month retention window.

### Country Attribution Options (when logging lacks GeoIP)

**Option A: In-service GeoIP lookup (library + GeoLite2 database)**

**Advantages**:
- Full control over lookup logic and attribution rules.
- Works with existing request IP data without changes to ingress/logging.
- Can support offline processing and backfills if IPs are retained.

**Disadvantages**:
- Introduces new dependency and data file (requires explicit approval).
- Requires periodic GeoIP database updates and secure distribution.
- Adds CPU/memory cost to request processing or batch aggregation.

**Option B: Ingress/logging GeoIP enrichment (e.g., NGINX GeoIP2 module)**

**Advantages**:
- Centralized enrichment; consistent across all services.
- Keeps application layer simpler; avoids app-level GeoIP dependency.
- Can attach country to logs/headers once for multiple consumers.

**Disadvantages**:
- Requires ops changes to ingress/logging stack and rollout coordination.
- GeoIP DB management still required (updates and licensing).
- Adds risk if headers can be spoofed without trusted proxy handling.
- Not viable if NGINX is being retired in the Kubernetes environment.

**Option C: Defer country attribution (report as unknown)**

**Advantages**:
- No new dependencies or infrastructure changes.
- Zero operational overhead; avoids GeoIP licensing/compliance concerns.
- Keeps delivery scope minimal.

**Disadvantages**:
- Fails country-related requirements and success criteria.
- Reduces report usefulness for stakeholders.
- Requires future rework to introduce attribution.

**Option D: Log-ingestion GeoIP enrichment (Grafana Alloy `loki.process`)**

**Advantages**:
- Centralized enrichment without adding latency to OSCAR requests.
- Keeps application layer simple; no new app dependencies.
- Works with existing Loki pipeline; can add country as labels once.

**Disadvantages**:
- Requires GeoIP database distribution and updates.
- Requires log parsing rules in Alloy (format coupling to log lines).
- Adds CPU/memory cost to the log shipper.

**Preferred approach**: Option D (log-ingestion GeoIP enrichment via Alloy),
because it avoids per-request latency and aligns with the resource-minimization
goal while keeping the application code unchanged.

## Decision 2: Report interface shape

**Decision**: Provide endpoints for per-service, per-metric queries plus summary
and breakdown outputs.

**Rationale**: Reporting owners need single-metric values for specific services
while still supporting full reporting views.

**Alternatives considered**:
- Only a summary endpoint (rejected: does not satisfy per-metric queries).
- Write periodic CSV files to storage (rejected: adds operational coupling and
  lacks on-demand ranges).

## Decision 3: Time range handling

**Decision**: Require explicit `start` and `end` timestamps for report requests
and treat missing or partial data sources as completeness warnings.

**Rationale**: Reporting is time-bounded and must communicate data gaps
explicitly for accuracy.

**Alternatives considered**:
- Default to a fixed time window (rejected: ambiguous for monthly reporting).
- Fail the request if any source is missing (rejected: operators still need
  partial results with warnings).

## Decision 4: User classification

**Decision**: Classify users based on authentication method: OIDC-authenticated
requests are members; OSCAR service token executions are external.

**Rationale**: This uses existing request metadata without requiring new
roster integrations.

**Alternatives considered**:
- Add new user tagging fields (rejected: schema/config changes not required).

## Decision 5: Durable request logs (retention compliance)

**Decision**: Use Loki for log storage and Grafana Alloy for log shipping to meet
the 6-month retention requirement for request-derived metrics.

**Rationale**: Loki + Alloy provides durable, queryable logs with a lightweight
operational footprint and integrates well with the existing Prometheus stack.

**Alternatives considered**:
- OpenSearch/Elasticsearch (heavier operational footprint).
- Object storage archive (durable but higher query latency and custom tooling).

## Log persistence options (retention compliance)

The metrics breakdown and request counts depend on request logs. Pod logs alone
are not durable across restarts, so they cannot satisfy the 6-month retention
requirement. To meet FR-013, request logs must be stored in a durable backend.

### Option 1: Loki + Grafana Alloy (recommended)

**Summary**: Deploy Loki for log storage and Grafana Alloy to ship Kubernetes
logs. Alloy is the recommended successor to Promtail and receives ongoing
feature development.

**Advantages**:
- Lightweight compared to Elasticsearch/OpenSearch.
- Fits well with the existing Prometheus stack.
- Kubernetes-native, easy to deploy via Helm.
- Supports label-based queries by namespace/service.

**Disadvantages**:
- Requires a new logging stack component and retention configuration.
- Query language (LogQL) is different from Elasticsearch.

### Option 2: OpenSearch/Elasticsearch

**Summary**: Centralized log storage with full-text search and rich query
capabilities.

**Advantages**:
- Powerful search and aggregation.
- Widely used with large ecosystem.

**Disadvantages**:
- Heavier operational footprint (CPU/memory/storage).
- More complex to run in small clusters.

### Option 3: Object storage archive (S3/MinIO)

**Summary**: Stream request logs to object storage and query from there.

**Advantages**:
- Durable, inexpensive storage.
- Fits environments already using MinIO/S3.

**Disadvantages**:
- Custom ingestion and query tooling required.
- Higher latency for queries unless indexed.

### Recommendation

Use Loki + Grafana Alloy for the shortest path to retention compliance, unless
the environment already runs OpenSearch/Elasticsearch or has strict requirements
to archive logs in object storage.
