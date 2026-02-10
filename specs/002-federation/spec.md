# Feature Specification: Federated OSCAR Service Replicas (Topology: star/mesh)

**Feature Branch**: `002-federation`  
**Created**: 2026-01-29  
**Status**: Draft  
**Input**: User description: "Enable federated replicas for OSCAR services across multiple clusters, with star/mesh topologies and delegation policies."

**Federation definition**: A federation is a logical group of OSCAR services
across multiple clusters that cooperate for delegated execution under shared
authentication, identified by a common `group_id`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Deploy a federated replica network (Priority: P1)

As a user, I want to deploy a service on multiple OSCAR clusters as a
federation so that jobs can be delegated across the service replicas deployed
across multiple clusters.

**Why this priority**: This is the core capability that enables multi-cluster
service replication.

**Independent Test**: Provide a valid FDL with federation settings for 2+ clusters
and confirm all services are created with correct topology and federation metadata.

**Acceptance Scenarios**:

1. **Given** a valid FDL with federation enabled and `topology=star`, **When** I
   submit it to the coordinator cluster, **Then** OSCAR Manager creates the
   coordinator service and deploys worker replicas to the specified clusters
   with `federation.members` cleared and appropriate FDL rewrites.
2. **Given** a valid FDL with federation enabled and `topology=mesh`, **When** I
   submit it to the coordinator cluster, **Then** OSCAR Manager creates services
   in all target clusters and each service has replicas for all other clusters.

---

### User Story 2 - Manage replicas via API (Priority: P2)

As a user, I want to add, update, or remove replicas for a service via the
`/system/replicas` API so that I can maintain the federation without
re-deploying everything.

**Why this priority**: Operational changes must be possible after initial
creation.

**Independent Test**: Use the replicas API to add a replica and verify the
service replica list changes accordingly.

**Acceptance Scenarios**:

1. **Given** an existing federated service, **When** I call
   `POST /system/replicas/{serviceName}` with a new replica definition,
   **Then** the replica is added and reflected in `GET /system/replicas/{serviceName}`.
2. **Given** an existing replica, **When** I call
   `PUT /system/replicas/{serviceName}` with an update payload,
   **Then** the replica’s attributes (e.g., priority) are updated.

---

### User Story 3 - Delegate jobs based on policy (Priority: P3)

As a service operator, I want jobs to be delegated according to a chosen policy
(static, random, or load-based) so that execution uses the most appropriate
cluster.

**Why this priority**: Delegation policy determines performance and reliability
of federated execution.

**Independent Test**: Configure a service with `delegation=random`, submit
multiple jobs, and verify that delegation targets vary across available clusters.

**Acceptance Scenarios**:

1. **Given** `delegation=static` with fixed priorities, **When** a job is
   scheduled for delegation, **Then** the highest-priority reachable cluster is
   selected.
2. **Given** `delegation=load-based`, **When** a job is delegated, **Then** the
   system queries `/system/status` and selects the cluster with the lowest
   CPU-based priority score (CPU-only ranking with per-node fit check).
3. **Given** an async job delegated to another cluster, **When** the target
   replica executes the job, **Then** it uses the delegated bearer token to call
   `/system/config` and obtain MinIO credentials for reading the origin input.

---

### Edge Cases

- A target cluster is unreachable during deployment (see FR‑020).
- A replica service does not yet exist in its cluster (see FR‑021). 
- Delegation selects a cluster that lacks required CPU/memory (must be excluded
  or deprioritized).
- Jobs are delegated but input data is not accessible in the target cluster
  (must use OIDC-backed bearer token to retrieve MinIO creds via `/system/config`).
- Credential distribution for multi-cluster access must not require embedding
  all cluster secrets in every FDL (needs secure approach).

### Additional Requirements (Edge Case Resolution)

- **FR-020**: If a target cluster is unreachable or returns non‑2xx during
  federation expansion on create, the operation MUST fail and trigger rollback
  (per FR‑019). The API response MUST include failed replicas and rollback status.
- **FR-021**: If a replica service does not exist in a target cluster during
  expansion, OSCAR Manager MUST create it; if it exists, OSCAR Manager MUST
  update it in-place.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST accept federation configuration in service FDLs and
  enable multi-cluster replica deployment.
- **FR-002**: System MUST support `topology` values of `none`, `star`, and `mesh`.
- **FR-003**: OSCAR Manager MUST expand federation only when
  `federation.members` is non-empty; worker replicas MUST carry federation
  metadata with empty `members` to avoid recursive expansion.
- **FR-004**: For `topology=star`, OSCAR Manager MUST deploy worker services with
  `federation.members` cleared, remove replica definitions in worker FDLs, and
  avoid embedding other cluster credentials.
- **FR-005**: For `topology=mesh`, OSCAR Manager MUST deploy worker services with
  replicas referencing all other clusters in the federation.
- **FR-006**: System MUST provide a replicas API at `/system/replicas/{serviceName}`
  with GET, POST, PUT, and DELETE operations for replica management.
- **FR-006a**: The replicas API MUST operate on the same underlying service
  definitions used by `/system/services` (no HTTP round-trips required).
- **FR-007**: System MUST support delegation policies `static`, `random`, and
  `load-based`.
- **FR-008**: For `load-based` delegation, system MUST query `/system/status`
  for candidate clusters and rank them using total free CPU only. The current
  algorithm maps `total_free_cores` (millicores) to a priority in the range
  0–100 using a linear scale from 0..32000 millicores; lower priority is better.
  A cluster is eligible only if `max_free_on_node_cores` is sufficient for the
  service CPU request.
- **FR-009**: System MUST expose `/system/status` with cluster CPU/memory metrics
  and node details sufficient to evaluate delegation fitness.
- **FR-010**: System MUST write all service outputs to a single shared output
  storage as defined in the federation configuration (same bucket/path across
  all replicas). For federated services, outputs MUST use `minio.default` with
  an origin endpoint override in `storage_providers.minio.default.endpoint` to
  route data to the origin cluster MinIO without embedding credentials.
- **FR-010b**: When a federated service uses `minio.default` with origin
  override, OSCAR Manager MUST normalize the output bucket using the origin
  service name (coordinator), not the replica service name.
- **FR-011**: System MUST preserve per-service input storage configuration as
  defined in the FDL.
- **FR-011a**: For MinIO/S3 inputs and outputs, if `path` omits the bucket (no
  `/` present), the system MUST default the bucket to the service name (e.g.,
  `input` → `<service-name>/input`).

*Example of marking unclear requirements:*

- **FR-012**: Federation identifier MUST be named `group_id`. If omitted, the
  system MUST default it to the service name.
- **FR-013**: Replica add/update/delete MUST apply to the whole topology
  (all services in the federation) to keep replica graphs consistent.
- **FR-015**: Federated services MUST define a refresh token as a service
  `secret` named `refresh_token`; OSCAR Manager MUST store it in the user's
  service namespace (OSCAR uses one namespace per user) and MUST NOT mount it
  into service pods.
- **FR-016**: Inter-cluster delegation MUST obtain a fresh OIDC bearer token
  using the refresh token before delegating a job.
- **FR-017**: Delegated jobs MUST retrieve MinIO credentials via `/system/config`
  using the fresh bearer token for the origin MinIO (via `minio.default`),
  then access input/output data in the origin cluster MinIO.
- **FR-018**: Any authenticated user MUST be allowed to create federations
  across clusters they are authenticated to. For federation creation, the
  request MUST use either (a) a Bearer token whose issuer is listed in
  `OIDC_ISSUERS`, or (b) BasicAuth credentials. OSCAR Manager MUST treat
  clusters as "authenticated to" only if the provided credentials are accepted
  by the target cluster during federation expansion. Acceptance is defined as
  a successful authenticated request to `GET /system/services/{serviceName}` on
  the target cluster (HTTP 200). Failed authentication MUST block federation
  creation.
- **FR-019**: Deployment MUST be transactional across target clusters **only
  during initial service creation**. If any target replica fails to deploy at
  create-time, OSCAR Manager MUST roll back by removing the coordinator service
  and any replicas that were already created. The API response MUST surface the
  failure and rollback status. Replica add/update/remove operations via
  `/system/replicas` MUST be best-effort per replica and MUST NOT delete
  existing healthy replicas on failure.

### Key Entities *(include if feature involves data)*

- **Federation**: The logical group of services participating in a federation
  (id, topology, delegation policy, members).
- **Replica**: A service instance in a specific cluster that is part of a
  federation (cluster_id, service_name, priority).
- **Delegation Policy**: The rule set controlling how jobs are routed across
  replicas (static/random/load-based).
- **Cluster Status**: Metrics returned by `/system/status` used to rank
  delegation targets (cpu/memory availability, node metrics).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Deploying a federation across N clusters results in N services
  created with correct topology within a single create request.
- **SC-002**: `GET /system/replicas/{serviceName}` returns consistent topology
  and replica lists after add/update/delete operations.
- **SC-003**: In `delegation=random`, at least two different clusters are chosen
  across 10 successive delegations when multiple clusters are available.
- **SC-004**: `load-based` delegation selects a cluster that meets CPU
  constraints in 100% of tested cases.
- **SC-005**: Outputs from any cluster replica are written to the shared output
  storage and are accessible from all clusters.

## Clarifications

### Session 2026-01-29

- Q: What is the federation identifier name? → A: `group_id` (defaults to service name if omitted).
- Q: Do replica add/update/delete operations apply to a single service or the whole topology? → A: Whole topology.
- Q: Where is FDL expansion performed? → A: OSCAR Manager (expands only when `federation.members` is non-empty).
- Q: How are inter-cluster credentials handled? → A: Federated services define a `secrets.refresh_token`; OSCAR Manager exchanges it for fresh OIDC bearer tokens when delegating.
- Q: How is input data handled for delegated jobs? → A: Use a fresh bearer token with `/system/config` to obtain MinIO credentials for origin cluster access.
- Q: Who can create a federation across clusters? → A: Any authenticated user can create a federation across clusters they are authenticated to, provided the auth check defined in FR‑018 succeeds.
- Q: How should unreachable target clusters be handled during deployment? → A: Deployment is transactional; any replica failure triggers rollback of the coordinator and created replicas, and the failure/rollback status is reported.
- Q: How are output buckets named for replicas using origin MinIO? → A: Use the origin service name for bucket prefixing.
