# Monitoring Stack (Prometheus + Loki + Alloy)

This document collects the monitoring setup and verification notes for the
metrics collection feature.

## Deploy Prometheus (optional, for metrics reporting)

If you want to test the metrics reporting endpoints that depend on Prometheus
(CPU/GPU hours), you can deploy Prometheus into the local Kind cluster using
Helm.

Prerequisites:

- `kubectl` configured for your Kind cluster
- `helm` installed

Steps:

```sh
kubectl create namespace monitoring
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
cp docs/snippets/prometheus-values.kind.yaml /tmp/prometheus-values.yaml
helm upgrade --install prometheus prometheus-community/prometheus \
  --namespace monitoring \
  --set server.service.type=ClusterIP \
  --values /tmp/prometheus-values.yaml
```

Notes:

- The kind values file sets `scrape_interval` to `2m` to reduce storage.

Wait for the Prometheus server pod to be ready, then port-forward it:

```sh
kubectl -n monitoring wait --for=condition=Ready pod \
  -l app.kubernetes.io/name=prometheus,app.kubernetes.io/instance=prometheus --timeout=120s
kubectl -n monitoring port-forward svc/prometheus-server 9090:80
```

Prometheus will be available at `http://127.0.0.1:9090`. Configure OSCAR to use
it:

```sh
export PROMETHEUS_URL="http://127.0.0.1:9090"
```

You can override the default Prometheus queries with:

```sh
export PROMETHEUS_CPU_QUERY='sum(increase(container_cpu_usage_seconds_total{namespace=~"{{services_namespace}}.*",service=~"{{service}}"}[{{range}}])) / 3600'
export PROMETHEUS_GPU_QUERY='sum(increase(container_gpu_usage_seconds_total{namespace=~"{{services_namespace}}.*",service=~"{{service}}"}[{{range}}])) / 3600'
```

Verify the minimal Prometheus config is active and only the kubelet cAdvisor
job is configured:

```sh
kubectl -n monitoring exec prometheus-server-<pod> -c prometheus-server -- \
  /bin/sh -c 'wget -qO- http://127.0.0.1:9090/api/v1/status/config'
```

Check that only OSCAR service namespaces are present in recent CPU series
(allow a few minutes for old series to expire after config changes):

```sh
kubectl -n monitoring exec prometheus-server-<pod> -c prometheus-server -- \
  /bin/sh -c 'wget -qO- "http://127.0.0.1:9090/api/v1/query?query=count%20by%20(namespace)%20(rate(container_cpu_usage_seconds_total%5B5m%5D))"'
```

## Deploy Loki + Grafana Alloy (optional, for durable request logs)

To keep request logs across pod restarts and support long-range breakdowns, you
can deploy Loki with Grafana Alloy to ship Kubernetes logs.

Prerequisites:

- `kubectl` configured for your Kind cluster
- `helm` installed

Steps:

```sh
kubectl create namespace monitoring
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update
cp docs/snippets/loki-values.kind.yaml /tmp/loki-values.yaml
helm upgrade --install loki grafana/loki --namespace monitoring --values /tmp/loki-values.yaml
```

> Note: `chunksCache` and `resultsCache` are disabled to fit kind’s limited
> memory. This reduces RAM usage but makes Loki queries slower and more
> CPU/IO-intensive. For production-sized clusters, enable caches with
> appropriate resource limits.
>
> Note: `max_query_length` is set to `0h` in the kind snippet to disable Loki's
> default query-range limit, which helps with month-long reports during testing.

Create an Alloy values file (example) and install Alloy. The kind example
filters logs to the OSCAR manager pods (`namespace=oscar` and `app=oscar`) and
the ingress-nginx controller pods (for exposed-service request counts) to keep
resource usage low. It also shows how to enrich OSCAR manager logs with GeoIP
data via `loki.process` (requires a GeoIP database file mounted into the pod):

```sh
kubectl apply -f docs/snippets/geoip-pvc.yaml
cp docs/snippets/alloy-values.kind.yaml /tmp/alloy-values.yaml
helm upgrade --install alloy grafana/alloy --namespace monitoring --values /tmp/alloy-values.yaml
```

Notes:

- The GeoIP enrichment stages expect the GeoLite2 Country database to be
  available at `/var/lib/alloy/geoip/GeoLite2-Country.mmdb` inside the pod.
- To load the GeoIP DB into the PVC locally, use the loader pod:

```sh
kubectl apply -f docs/snippets/geoip-loader-pod.yaml
kubectl -n monitoring cp /path/to/GeoLite2-Country.mmdb \
  geoip-loader:/var/lib/geoip/GeoLite2-Country.mmdb
kubectl -n monitoring delete pod geoip-loader
```

Example (local file already downloaded):

```sh
kubectl apply -f docs/snippets/geoip-loader-pod.yaml
kubectl -n monitoring cp /Users/gmolto/Downloads/GeoLite2-Country.mmdb \
  geoip-loader:/var/lib/geoip/GeoLite2-Country.mmdb
kubectl -n monitoring delete pod geoip-loader
```

API verification (countries in summary):

```sh
curl -s "http://127.0.0.1:8080/system/metrics?start=2026-01-01T00:00:00Z&end=2026-01-31T23:59:59Z" | jq .totals.countries
```

Configure OSCAR to use Loki:

```sh
export LOKI_URL="http://loki.monitoring.svc.cluster.local:3100"
```

Use the gateway service for Loki queries if enabled:

```sh
export LOKI_URL="http://loki-gateway.monitoring.svc.cluster.local"
```

## Query Loki logs (examples)

LogQL queries must be URL-encoded. In zsh, `{}` can be expanded, so use
`--data-urlencode` when calling the Loki API.

If you are running locally, port-forward Loki first:

```sh
kubectl -n monitoring port-forward svc/loki 3100:3100
```

List log lines (last 10 entries) for OSCAR manager logs:

```sh
curl -sG "http://127.0.0.1:3100/loki/api/v1/query_range" \
  --data-urlencode 'query={namespace="oscar"}' \
  --data-urlencode 'limit=10' | jq .
```

If you get `jq: parse error`, retry without `jq` to inspect the raw output:

```sh
curl -sG "http://127.0.0.1:3100/loki/api/v1/query_range" \
  --data-urlencode 'query={namespace="oscar"}' \
  --data-urlencode 'limit=10'
```

Expose request counts for exposed services with these optional overrides:

```sh
export LOKI_EXPOSED_QUERY='{namespace="{{namespace}}", app="{{app}}"} |~ "/system/services/.+/exposed"'
export LOKI_EXPOSED_NAMESPACE="ingress-nginx"
export LOKI_EXPOSED_APP="ingress-nginx"
```

## Deploy Grafana (optional, for visualization)

Grafana can visualize the metrics defined in the spec using Prometheus and Loki
as data sources. The provided dashboard includes CPU/GPU hours, service counts,
and request counts derived from logs.

Prerequisites:

- `kubectl` configured for your Kind cluster
- `helm` installed

Steps:

```sh
kubectl create namespace monitoring
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update
cp docs/snippets/grafana-values.kind.yaml /tmp/grafana-values.yaml
cp docs/snippets/oscar-metrics-dashboard.json /tmp/oscar-metrics-dashboard.json

kubectl -n monitoring create configmap grafana-oscar-dashboard \
  --from-file=dashboard.json=/tmp/oscar-metrics-dashboard.json \
  --dry-run=client -o yaml | kubectl apply -f -

helm upgrade --install grafana grafana/grafana \
  --namespace monitoring \
  --values /tmp/grafana-values.yaml
```

Port-forward Grafana and log in with the default credentials from the values
file:

```sh
kubectl -n monitoring port-forward svc/grafana 3000:80
```

Notes:

- The dashboard uses Loki queries that assume OSCAR manager logs include
  `/run/<service>` and `/job/<service>` paths in the log line. Adjust LogQL
  expressions if your log format differs.
- The "Services Count" panel uses active pod counts as a proxy for services.
  If you need exact service inventory counts, consider adding a Grafana JSON
  datasource panel backed by the OSCAR `/system/metrics` endpoint.

## Check Prometheus/Loki disk usage (kind)

Prometheus images include `du`, so you can check usage from the pod. Loki uses
a distroless image, so check its PVC on the kind node filesystem.

```sh
kubectl -n monitoring get pods
```

Prometheus data usage:

```sh
kubectl -n monitoring exec prometheus-server-<pod> -c prometheus-server -- du -sh /data
```

Loki data usage (PVC -> PV -> hostPath, then check on kind node container):

```sh
kubectl -n monitoring get pvc
kubectl get pv pvc-<loki-pv> -o jsonpath='{.spec.hostPath.path}'
docker exec <kind-control-plane-container> du -sh <hostpath>
```

Example (local-path provisioner):

```sh
kubectl -n monitoring get pvc
kubectl get pv pvc-<loki-pv> -o jsonpath='{.spec.hostPath.path}'
kubectl get pv pvc-<prometheus-pv> -o jsonpath='{.spec.hostPath.path}'
docker exec <kind-control-plane-container> du -sh /var/local-path-provisioner/pvc-<loki-pv>_monitoring_storage-loki-0
docker exec <kind-control-plane-container> du -sh /var/local-path-provisioner/pvc-<prometheus-pv>_monitoring_prometheus-server
```
