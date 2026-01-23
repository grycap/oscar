# OSCAR API 

OSCAR exposes a secure REST API available at the Kubernetes master's node IP
through an Ingress Controller. This API has been described following the
[OpenAPI Specification](https://www.openapis.org/) and it is available below.

> ℹ️
>
> The bearer token used to run a service can be either the OSCAR [service access token](invoking-sync.md#service-access-tokens) or the [user's Access Token](integration-egi.md#obtaining-an-access-token) if the OSCAR cluster is integrated with EGI Check-in.

## Metrics reporting

Metrics reporting endpoints include `/system/metrics/value`, `/system/metrics/summary`, and
`/system/metrics/breakdown`. The breakdown endpoint supports CSV output by setting
`format=csv` and grouping with `group_by` (service, user, country).

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

!!swagger swagger.yaml!!
