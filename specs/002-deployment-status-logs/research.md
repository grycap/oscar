# Phase 0 Research: Service Deployment Visibility

## Decision 1: Expose deployment visibility through additive service-scoped endpoints

- **Decision**: Add deployment visibility under the existing
  `/system/services/{serviceName}` API surface instead of extending
  `/system/status` or reusing the job-log routes.
- **Rationale**: The repository already models service-specific reads and logs
  by `serviceName`, and those routes align with existing auth and
  namespace-resolution patterns. Keeping deployment visibility beside service
  resources avoids mixing per-service runtime status with cluster-wide admin
  status.
- **Alternatives considered**:
  - Extend `/system/status`: rejected because it is cluster-wide and partially
    admin-oriented.
  - Overload `/system/logs/{serviceName}`: rejected because those routes
    already mean job execution logs and statuses.

## Decision 2: Reuse the existing service authorization flow

- **Decision**: Reuse `back.ReadService("", serviceName)` together with the
  current `getAuthorizedService`, `authorizeRequest`, and
  `resolveServiceNamespace` flow used by the existing log handlers.
- **Rationale**: Current service-scoped handlers already enforce visibility for
  bearer tokens while preserving the current basic-auth behavior. Reusing those
  helpers keeps deployment visibility aligned with existing service ownership
  and allowed-user rules.
- **Alternatives considered**:
  - Introduce a separate deployment-visibility authorization layer: rejected
    because it duplicates existing service-scoped logic.
  - Restrict deployment visibility to admins only: rejected because the feature
    targets users who can already view the service.

## Decision 3: Cover all OSCAR service types, with explicit unavailable fallback

- **Decision**: Apply the feature at the API level to all OSCAR service types,
  but return `unavailable` when a service does not expose a current deployment
  or runtime representation suitable for deployment visibility.
- **Rationale**: The spec was clarified to cover OSCAR services uniformly while
  still being honest about backend/runtime differences. This avoids narrowing
  the API to exposed services only without falsely implying that every service
  type always has inspectable deployment state.
- **Alternatives considered**:
  - Exposed services only: rejected because the user wants broader OSCAR
    coverage.
  - Guarantee deployment visibility for every service type: rejected because
    some service/runtime combinations may only expose job-level evidence.

## Decision 4: Model deployment state from live backend-specific runtime resources

- **Decision**: Build the deployment summary from the live runtime objects that
  already represent a service in each backend and exposure mode. For exposed
  services, inspect the Kubernetes `Deployment` and related pods created by
  `pkg/backends/resources/expose.go`. For backend-managed runtimes, inspect the
  current backend representation and associated pods where present. When there
  is no current representation, return `unavailable`.
- **Rationale**: OSCAR uses more than one backend model today. The plan must
  reflect real runtime objects without pretending that every service is always
  backed by a Kubernetes `Deployment`.
- **Alternatives considered**:
  - Require a Kubernetes `Deployment` for every service: rejected because Kube
    and Knative backends do not model every service that way.
  - Fabricate deployment status from static service metadata only: rejected
    because it would not expose actual readiness or failure causes.

## Decision 5: Keep visibility at the service-summary level

- **Decision**: Return a service-level deployment summary and service-level
  current or recent deployment logs only; do not expose an instance list or
  instance-specific log retrieval in this phase.
- **Rationale**: The feature was simplified to keep the first slice focused and
  additive. Service-level visibility still lets users determine whether a
  deployment is healthy, partially affected, failed, or unavailable without
  committing the API to backend-specific instance shapes.
- **Alternatives considered**:
  - Service summary plus per-instance log access: rejected because it adds
    endpoint and schema complexity that is no longer desired for the first
    phase.
  - Per-instance logs only: rejected because the API should remain service
    oriented.

## Decision 6: Limit the feature to current deployment state and recent deployment logs

- **Decision**: Expose only the current deployment snapshot and a bounded
  current or recent log window; do not add deployment history, rollout diffing,
  or audit-trail reporting in this phase.
- **Rationale**: The spec is explicitly scoped to current visibility and recent
  evidence. This keeps the API fast, avoids retention commitments, and limits
  implementation complexity.
- **Alternatives considered**:
  - Short recent history of attempts: rejected because it requires a larger
    status model and more storage/query rules.
  - Full audit trail: rejected as out of scope.

## Decision 7: Return raw deployment logs to authorized service viewers

- **Decision**: Return raw deployment log content to users who are already
  authorized to view the service; do not add redaction in this phase.
- **Rationale**: The clarified spec assumes any sensitive values visible in
  deployment logs are already known to the authorized user. Avoiding redaction
  also keeps the first implementation smaller and avoids introducing partial or
  misleading log output transformations.
- **Alternatives considered**:
  - Redact sensitive values: rejected because the clarified requirement prefers
    raw logs for authorized viewers.
  - Restrict deployment logs to admins only: rejected because it would not
    match existing service-view permissions.

## Decision 8: Allow last-attempt logs only when they are already cheaply available

- **Decision**: When a service has no current deployment or runtime
  representation, the status endpoint still returns `unavailable`, but the log
  endpoint may return recent logs from the last runtime attempt only when those
  logs are already available through the existing operational log source
  without introducing a separate complex retrieval path.
- **Rationale**: This preserves useful failure evidence after a startup crash
  while avoiding additional technical debt or a second log-retrieval pipeline.
- **Alternatives considered**:
  - Return `unavailable` for both status and logs whenever the current runtime
    is absent: rejected because it can discard still-available recent failure
    evidence.
  - Always retrieve last-attempt logs through new fallback logic: rejected
    because it increases implementation complexity and maintenance cost.
