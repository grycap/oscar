# Metrics reporting

Metrics reporting endpoints include `/system/metrics/{serviceName}`, `/system/metrics`,
`/system/metrics/breakdown`, and the administrative owner catalog at
`/system/metrics/owners`. If the `metric` query parameter is omitted from
`/system/metrics/{serviceName}`, the API returns all supported per-service metrics.
The breakdown endpoint supports CSV output by setting
`format=csv` and grouping with `group_by` (service, user, country). To include
the list of users per service, set `include_users=true` (JSON only).

The `start`/`end` query parameters are optional. If omitted, the API defaults to
the last 24 hours (end = now, start = end - 24h).

When using OIDC Bearer tokens, metrics are limited to services owned by the
authenticated user. Public services owned by other users and restricted
services where the user is listed in `allowed_users` are not included. When
using Basic Auth as the OSCAR admin user, metrics remain cluster-wide and
include all services.

The OSCAR administrator can add the `owner` query parameter to the summary,
breakdown, and per-service endpoints. The result is then limited to the exact
namespace owned by that user, including attributable metrics for services that
have already been deleted. Bearer-authenticated requests cannot select another
owner.

The `/system/metrics/owners` endpoint is available only with administrator
Basic Auth. It builds its owner catalog from persistent namespaces managed by
OSCAR (`app.kubernetes.io/managed-by=oscar`) and the
`oscar.grycap.upv.es/owner` namespace annotation. Display names are enriched
from active service metadata when available. The shared `cluster_admin`
namespace is returned explicitly as `oscar`.

For example:

```text
GET /system/metrics?owner=user@example.org
GET /system/metrics/breakdown?group_by=service&owner=user@example.org
GET /system/metrics/my-service?owner=user@example.org
```

Owned service metrics are scoped by the user's service namespace. This allows
OIDC users to query retained metrics for their deleted services without
exposing services from another user's namespace. Historical OSCAR manager
request records created before the execution log included the
service namespace can only be attributed while the service is still active.
Existing Traefik JSON access logs remain attributable when their `ServiceName`
field is available.

### Prometheus usage metrics

CPU/GPU hours are fetched from Prometheus. If `PROMETHEUS_URL` is not set, the
service defaults to `http://prometheus-server.monitoring.svc.cluster.local`.
You can override the default Prometheus queries via:

- `PROMETHEUS_CPU_QUERY` (default uses `{{service}}`, `{{range}}`, and `{{namespace}}`)
- `PROMETHEUS_GPU_QUERY` (default uses `{{service}}`, `{{range}}`, and `{{namespace}}`)

Custom Prometheus templates should use `{{namespace}}` so OSCAR can apply an
exact user namespace for OIDC requests. The legacy `{{services_namespace}}`
placeholder remains supported for existing installations, but custom templates
using it should be migrated.

### Loki request logs (durable breakdowns)

Request-based metrics (breakdowns, request counts) can be sourced from Loki for
durable retention. Set `LOKI_URL` to enable Loki, otherwise the system falls back
to Kubernetes pod logs.

- `LOKI_URL` (e.g., `http://loki.monitoring.svc.cluster.local:3100`)
- `LOKI_QUERY` (default uses `{{namespace}}` and `{{app}}`; if you add `{{service}}`, prefer a regex matcher like `service=~"{{service}}"` so summary queries can expand to `.*`)
- `LOKI_EXPOSED_QUERY` (base selector for gateway access logs; OSCAR adds the service path or DNS host filter)
- `LOKI_EXPOSED_NAMESPACE` (gateway namespace; default `ingress-nginx`)
- `LOKI_EXPOSED_APP` (gateway app label; default `ingress-nginx`)

When exposed services are routed through DNS subdomains (see
[Exposed services](exposed-services.md)), expose the gateway access logs of the
routing gateway. For Traefik HTTPRoutes, set `LOKI_EXPOSED_NAMESPACE` and
`LOKI_EXPOSED_APP` to the gateway namespace and app label used by the Traefik
installation (e.g., `traefik`) instead of the `ingress-nginx` defaults. The
provided Alloy configuration keeps the full JSON body of the Traefik access
logs so that OSCAR can attribute requests.

#### Configuration validation

Check the effective configuration before relying on exposed-service metrics:

- If `EXPOSED_SERVICES_USE_SUBDOMAIN_ROUTE=true` and
  `EXPOSED_SERVICES_ROUTE_KIND=httproute`, `INGRESS_HOST` must be set; otherwise
  requests cannot be attributed from the DNS host.
- The gateway access logs must contain the JSON fields used for attribution:
  with DNS subdomains, OSCAR identifies the service from the request host
  (`RequestHost`, falling back to `RequestAddr`) and obtains its namespace from
  Traefik's `ServiceName` access-log field. Without DNS subdomains, the service
  is extracted from `RequestPath` (`/system/services/{service_name}/exposed/...`).
- The Alloy pipeline must retain the JSON access log body; if the access log is
  reformatted or relabeled before reaching Loki, the fields above are lost and
  requests will not be attributed. The provided Traefik configuration already
  does so.
