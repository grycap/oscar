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

loki_host_path=$(kubectl get pv "$loki_pvc" -o jsonpath='{.spec.hostPath.path}' 2>/dev/null || true)
loki_nfs_server=$(kubectl get pv "$loki_pvc" -o jsonpath='{.spec.nfs.server}' 2>/dev/null || true)
loki_nfs_path=$(kubectl get pv "$loki_pvc" -o jsonpath='{.spec.nfs.path}' 2>/dev/null || true)

if [ -n "$loki_host_path" ]; then
  need_cmd docker
  node=$(kubectl -n "$ns" get pod loki-0 -o jsonpath='{.spec.nodeName}' 2>/dev/null || true)
  if [ -z "$node" ]; then
    echo "Unable to determine node for loki-0 pod" >&2
    exit 1
  fi

  printf '\nLoki data usage (PV %s on node %s):\n' "$loki_pvc" "$node"
  if ! docker exec "$node" sh -c "du -sh '$loki_host_path' 2>/dev/null"; then
    echo "Failed to read Loki data usage from node container $node" >&2
  fi
else
  loki_pod=$(kubectl -n "$ns" get pod -l app.kubernetes.io/name=loki,app.kubernetes.io/component=single-binary -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
  if [ -z "$loki_pod" ]; then
    echo "Loki pod not found in namespace $ns" >&2
    exit 1
  fi

  printf '\nLoki data usage (from pod %s):\n' "$loki_pod"
  if ! kubectl -n "$ns" exec "$loki_pod" -c loki -- du -sh /var/loki 2>/dev/null; then
    echo "Loki image missing shell/du; using a temporary debug pod to read usage."
    tmp_pod="loki-du-$$"
    kubectl -n "$ns" run "$tmp_pod" \
      --image=busybox:1.36 \
      --restart=Never \
      --rm \
      --quiet \
      -i \
      --overrides="{\"apiVersion\":\"v1\",\"spec\":{\"volumes\":[{\"name\":\"loki-storage\",\"persistentVolumeClaim\":{\"claimName\":\"storage-loki-0\"}}],\"containers\":[{\"name\":\"loki-du\",\"image\":\"busybox:1.36\",\"command\":[\"sh\",\"-c\",\"du -sh /var/loki\"],\"volumeMounts\":[{\"name\":\"loki-storage\",\"mountPath\":\"/var/loki\"}]}],\"restartPolicy\":\"Never\"}}"
  fi

  if [ -n "$loki_nfs_server" ] || [ -n "$loki_nfs_path" ]; then
    echo "Loki PV NFS location: ${loki_nfs_server}:${loki_nfs_path}"
  fi
fi
loki_cap=$(kubectl get pv "$loki_pvc" -o jsonpath='{.spec.capacity.storage}' 2>/dev/null || true)
if [ -n "$loki_cap" ]; then
  echo "Loki PV capacity: $loki_cap"
fi
