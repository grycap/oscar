# Research: Federated OSCAR Service Replicas (Topology: tree/mesh)

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
- **Rationale**: Mesh/tree invariants require consistent replica graphs across
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

### Decision: Best-effort deployment across clusters
- **Rationale**: Cross-cluster reachability is variable; proceed with reachable
  clusters while reporting failures.
- **Alternatives considered**:
  - Fail-fast on any unreachable cluster (rejected: partial outage blocks all).

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

### Decision: Remove `role` and infer worker from empty members
- **Rationale**: Avoids explicit `role` field while still preventing recursive
  expansion. OSCAR Manager will expand only when `federation.members` is
  non-empty; worker replicas carry federation metadata with an empty members
  list.
- **Alternatives considered**:
  - Keep explicit `role` field (rejected: extra user-facing complexity).
  - Add `federation.expanded` flag (rejected: new field still required).
