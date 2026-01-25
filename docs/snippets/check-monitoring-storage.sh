#!/usr/bin/env sh
set -eu

ns="monitoring"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

need_cmd kubectl
need_cmd docker

prom_pod=$(kubectl -n "$ns" get pods -l app.kubernetes.io/name=prometheus,app.kubernetes.io/component=server -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -z "$prom_pod" ]; then
  echo "Prometheus pod not found in namespace $ns" >&2
  exit 1
fi

printf '\nPrometheus data usage (from pod %s):\n' "$prom_pod"
if ! kubectl -n "$ns" exec "$prom_pod" -c prometheus-server -- sh -c 'du -sh /data 2>/dev/null'; then
  echo "Failed to read Prometheus /data usage." >&2
fi
prom_pv=$(kubectl -n "$ns" get pvc prometheus-server -o jsonpath='{.spec.volumeName}' 2>/dev/null || true)
if [ -n "$prom_pv" ]; then
  prom_cap=$(kubectl get pv "$prom_pv" -o jsonpath='{.spec.capacity.storage}' 2>/dev/null || true)
  if [ -n "$prom_cap" ]; then
    echo "Prometheus PV capacity: $prom_cap"
  fi
fi

loki_pvc=$(kubectl -n "$ns" get pvc storage-loki-0 -o jsonpath='{.spec.volumeName}' 2>/dev/null || true)
if [ -z "$loki_pvc" ]; then
  echo "Loki PVC storage-loki-0 not found in namespace $ns" >&2
  exit 1
fi

loki_path=$(kubectl get pv "$loki_pvc" -o jsonpath='{.spec.hostPath.path}' 2>/dev/null || true)
if [ -z "$loki_path" ]; then
  echo "Unable to locate hostPath for PV $loki_pvc" >&2
  exit 1
fi

node=$(kubectl -n "$ns" get pod loki-0 -o jsonpath='{.spec.nodeName}' 2>/dev/null || true)
if [ -z "$node" ]; then
  echo "Unable to determine node for loki-0 pod" >&2
  exit 1
fi

printf '\nLoki data usage (PV %s on node %s):\n' "$loki_pvc" "$node"
if ! docker exec "$node" sh -c "du -sh '$loki_path' 2>/dev/null"; then
  echo "Failed to read Loki data usage from node container $node" >&2
fi
loki_cap=$(kubectl get pv "$loki_pvc" -o jsonpath='{.spec.capacity.storage}' 2>/dev/null || true)
if [ -n "$loki_cap" ]; then
  echo "Loki PV capacity: $loki_cap"
fi
