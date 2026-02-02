# OSCAR API 

OSCAR exposes a secure REST API available at the Kubernetes master's node IP
through an Ingress Controller. This API has been described following the
[OpenAPI Specification](https://www.openapis.org/) and it is available below.

> ℹ️
>
> The bearer token used to run a service can be either the OSCAR [service access token](invoking-sync.md#service-access-tokens) or the [user's Access Token](integration-egi.md#obtaining-an-access-token) if the OSCAR cluster is integrated with EGI Check-in.

## Service replicas (federation)

Federated replicas are managed through `/system/replicas/{serviceName}` with
GET/POST/PUT/DELETE operations. Updates apply to the whole federation topology
(tree/mesh). Federated services MUST provide `environment.secrets.refresh_token`;
OSCAR Manager exchanges it for fresh OIDC bearer tokens when delegating jobs
across clusters. This requires `OIDC_CLIENT_ID` (and optionally
`OIDC_CLIENT_SECRET`) to be configured on the OSCAR Manager. When multiple
issuers are configured in `OIDC_ISSUERS`, the token exchange uses the first
issuer in the list, so ordering matters.

## Metrics reporting

Metrics reporting endpoints include `/system/metrics/{serviceName}`, `/system/metrics`, and
`/system/metrics/breakdown`. If the `metric` query parameter is omitted from
`/system/metrics/{serviceName}`, the API returns all supported per-service metrics.
The breakdown endpoint supports CSV output by setting
`format=csv` and grouping with `group_by` (service, user, country). To include
the list of users per service, set `include_users=true` (JSON only).

The `start`/`end` query parameters are optional. If omitted, the API defaults to
the last 24 hours (end = now, start = end - 24h).

### Status capacity note (single-node clusters)

When `/system/status` reports cluster capacity, control-plane nodes are normally
excluded. If a cluster has **no worker nodes**, OSCAR includes control-plane
nodes so that capacity is still reported in single-node dev setups (e.g., kind).

### Prometheus usage metrics

CPU/GPU hours are fetched from Prometheus. If `PROMETHEUS_URL` is not set, the
service defaults to `http://prometheus-server.monitoring.svc.cluster.local`.
You can override the default Prometheus queries via:

- `PROMETHEUS_CPU_QUERY` (default uses `{{service}}`, `{{range}}`, and `{{services_namespace}}`)
- `PROMETHEUS_GPU_QUERY` (default uses `{{service}}`, `{{range}}`, and `{{services_namespace}}`)

### Loki request logs (durable breakdowns)

Request-based metrics (breakdowns, request counts) can be sourced from Loki for
durable retention. Set `LOKI_URL` to enable Loki, otherwise the system falls back
to Kubernetes pod logs.

- `LOKI_URL` (e.g., `http://loki.monitoring.svc.cluster.local:3100`)
- `LOKI_QUERY` (default uses `{{namespace}}` and `{{app}}`; if you add `{{service}}`, prefer a regex matcher like `service=~"{{service}}"` so summary queries can expand to `.*`)
- `LOKI_EXPOSED_QUERY` (LogQL query for exposed-service requests; default filters `/system/services/.+/exposed`)
- `LOKI_EXPOSED_NAMESPACE` (default `ingress-nginx`)
- `LOKI_EXPOSED_APP` (default `ingress-nginx`)

!!swagger swagger.yaml!!
