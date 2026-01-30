# Quickstart: Federated OSCAR Service Replicas

## Prerequisites
- OIDC bearer token valid across target OSCAR clusters.
- An OSCAR service definition (FDL) you can deploy.
- Input data stored in the origin cluster MinIO; target replicas retrieve
  credentials via `/system/config` using the delegated bearer token.

## 1) Create a federated service (coordinator FDL)

Create a service FDL with federation settings (example placeholders):

```yaml
functions:
  oscar:
    - name: grayifyr0
      image: ghcr.io/grycap/imagemagick
      cpu: "0.5"
      memory: 0.5Gi
      script: script.sh
      federation:
        topology: mesh
        delegation: random
        members:
          - type: oscar
            cluster_id: oscar-cluster-a
            service_name: grayifyr1
          - type: oscar
            cluster_id: oscar-cluster-b
            service_name: grayifyr2
      input:
        - storage_provider: minio.default
          path: grayifyr0/in
      output:
        - storage_provider: minio.shared
          path: grayifyr0/out
```

Submit the FDL to the coordinator cluster (via existing OSCAR create service API
or CLI). OSCAR Manager expands the FDL and deploys replicas to all clusters.

## 2) Verify replicas

Use the replicas API to confirm topology and members:

```bash
curl -H "Authorization: Bearer <SERVICE_TOKEN>" \
  https://<cluster-endpoint>/system/replicas/grayifyr0
```

## 3) Add or update replicas

Add a replica:

```bash
curl -X POST -H "Authorization: Bearer <SERVICE_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"replicas":[{"type":"oscar","cluster_id":"oscar-cluster-c","service_name":"grayifyr3"}]}' \
  https://<cluster-endpoint>/system/replicas/grayifyr0
```

Updates apply to the whole topology and are propagated by OSCAR Manager.

## Notes
- Deployment is best-effort: unreachable clusters are reported but do not block
  successful deployments to reachable clusters.
- Delegation uses bearer tokens that are valid across federation clusters.
