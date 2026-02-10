# Research: Federated OSCAR Service Replicas (Topology: star/mesh)

## Decisions

### Decision: FDL expansion happens in OSCAR Manager
- **Rationale**: Centralizes orchestration and keeps CLI/SDK simple; ensures a
  single source of truth for federation topology expansion.
- **Alternatives considered**:
  - CLI/SDK expansion (rejected: more client complexity and drift risk).
  - Dual-mode expansion (rejected: validation ambiguity and higher maintenance).

### Decision: Use `/system/replicas` API for replica lifecycle
- **Rationale**: Replica operations require topology-aware changes and targeted
  updates without full service spec replacement.
- **Alternatives considered**:
  - Use `PUT /system/services` only (rejected: full-spec merges and higher drift
    risk for topology updates).

### Decision: Replica changes apply to whole topology
- **Rationale**: Mesh/star invariants require consistent replica graphs across
  all services in the federation.
- **Alternatives considered**:
  - Single-service updates (rejected: asymmetric replica graphs and drift).

### Decision: Federation identifier name is `group_id` (defaults to service name)
- **Rationale**: Aligns with proposed FDL examples while keeping simple setups
  minimal by defaulting to service name if omitted.
- **Alternatives considered**:
  - `network_id`, `federation_id` (rejected: inconsistent with current draft).

### Decision: Inter-cluster auth uses OIDC bearer tokens
- **Rationale**: Federated clusters share OIDC and bearer tokens are valid across
  clusters, enabling delegation without embedded credentials.
- **Alternatives considered**:
  - Embed cluster credentials in FDL (rejected: security risk).
  - Service tokens for delegation (rejected: user context loss).

### Decision: Create-time transactional deployment across clusters
- **Rationale**: Consistency during initial federation creation; partial
  deployment is rolled back to avoid orphaned replicas.
- **Alternatives considered**:
  - Best-effort deployment (rejected: inconsistent topology at creation time).

### Decision: Any authenticated user can create federations across clusters
- **Rationale**: Matches requirement that any user can deploy a replicated
  within clusters they are authenticated to.
- **Alternatives considered**:
  - Admin-only federation creation (rejected: too restrictive for users).

## Technical Context Resolutions

### Language/Version
- **Decision**: Go 1.25 (repository standard).
- **Alternatives considered**: N/A.

### Primary Dependencies
- **Decision**: gin-gonic, client-go, metrics.k8s.io client (existing stack).
- **Alternatives considered**: N/A.

### Storage
- **Decision**: Kubernetes API resources for service state; output storage via
  MinIO or external providers as configured in FDL.
- **Alternatives considered**: N/A.

### Testing
- **Decision**: `go test ./...` for touched Go packages.
- **Alternatives considered**: N/A.

### Target Platform
- **Decision**: Linux/Kubernetes OSCAR clusters.
- **Alternatives considered**: N/A.

### Project Type
- **Decision**: Single Go service (OSCAR Manager + APIs).
- **Alternatives considered**: N/A.

### Performance Goals
- **Decision**: No new numeric performance targets; must not regress existing
  service scheduling and delegation behavior.
- **Alternatives considered**: Define explicit latency/throughput targets (defer
  to plan if needed).

### Constraints
- **Decision**: No new dependencies or CI/CD changes without approval; do not
  modify `dashboard/dist`; preserve existing behavior unless requested.
- **Alternatives considered**: N/A.

### Scale/Scope
- **Decision**: Multi-cluster federations with unspecified N; design for
  reasonable cluster counts without introducing new infrastructure.
- **Alternatives considered**: Hard caps (defer; not in current requirements).

## Async input access for delegated jobs

### Decision: Use `/system/config` with bearer token to obtain MinIO credentials
- **Rationale**: Clusters support OIDC and bearer tokens are valid across
  clusters. The target replica can use the delegated bearer token to request
  MinIO credentials from the origin cluster and access inputs.
- **Alternatives considered**:
  - Shared input storage across clusters (rejected: per-cluster MinIO).
  - Signed URLs or service-token fetch (rejected: inconsistent with OIDC-based
    access model).

### Decision: Use `minio.default` with origin override to route delegated outputs
- **Rationale**: FaaS Supervisor only reads mounted MinIO credentials for the
  `default` provider. To avoid embedding credentials in the FDL, federated
  services keep `storage_provider: minio.default` and override the default
  MinIO endpoint in `storage_providers.minio.default.endpoint`. When a worker
  receives a delegated job, OSCAR Manager fetches origin credentials via
  `/system/config`, mounts them at `minio.default`, and the supervisor uploads
  outputs to the origin MinIO.
- **Implementation note**: Worker services carry
  `oscar.grycap/origin-service` and `oscar.grycap/origin-cluster` annotations.
  When normalizing output paths for `minio.default` with origin override, OSCAR
  Manager uses the origin service name to keep the bucket consistent across
  replicas.
- **Alternatives considered**:
  - Use explicit providers (e.g., `minio.<cluster_id>`) and read credentials
    from per-provider secrets (rejected: requires faas-supervisor changes).
  - Embed MinIO credentials directly in delegated events (rejected: security
    risk and leakage via logs).

### Decision: Remove `role` and infer worker from empty members
- **Rationale**: Avoids explicit `role` field while still preventing recursive
  expansion. OSCAR Manager will expand only when `federation.members` is
  non-empty; worker replicas carry federation metadata with an empty members
  list.
- **Alternatives considered**:
  - Keep explicit `role` field (rejected: extra user-facing complexity).
  - Add `federation.expanded` flag (rejected: new field still required).

## Delegation authentication and token expiry

### Decision: Require a refresh token in service `secrets` for federated services
- **Rationale**: Rescheduled jobs lack the original user bearer token. A refresh
  token allows OSCAR Manager to mint fresh access tokens when delegating jobs to
  replicas on behalf of the invoking user.
- **Alternatives considered**:
  - Short-lived delegation tokens (rejected: additional token-issuance
    infrastructure and validation flow).
  - Cluster basic auth (rejected: not user-scoped).
  - Per-job access tokens (rejected: expire before reschedule).

### Security implications (explicitly accepted)
- Refresh tokens are long-lived and high-impact if exposed.
- Service secrets are accessible to pods in the service namespace (OSCAR uses
  one namespace per user); compromise of any service pod can leak the refresh
  token.
- Cluster admins can access secrets; this expands trust requirements.
- Rotation and revocation are operational concerns and are out of scope for this
  feature; they may be addressed in a future scope.

### Required mitigations
- Store refresh tokens only in Kubernetes Secrets in the **user namespace**.
- Strict RBAC: only OSCAR Manager (and service account used for delegation)
  can read the refresh-token Secret.
- Do not mount refresh tokens into service pods; read them only from OSCAR
  Manager during delegation.
- Define rotation policy (e.g., 30 days) and revocation on user request.
- Audit access to the Secret (where supported).
