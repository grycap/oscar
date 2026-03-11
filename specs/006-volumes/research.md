# Phase 0 Research: Managed Volumes for OSCAR Services

## Decision 1: Represent service-managed storage as a top-level `volume` block

- Decision: Use an optional top-level `volume` object containing `name`, `size`, `mount_path`, and `lifecycle_policy` in the service FDL.
- Rationale: This keeps the configuration aligned with the current FDL style (`mount`, `expose`, `synchronous`) while making the model reusable beyond a single service. The same object can represent either volume creation or volume attachment without introducing nested sub-objects.
- Alternatives considered:
  - Keep the previous service-scoped storage field name and add new volume APIs: rejected because it would preserve confusing terminology for a now shared resource.
  - Split service config into `create_volume` and `attach_volume`: rejected because it would make the FDL noisier for the common case.

## Decision 2: Use `size` presence to distinguish volume creation from mounting an existing volume

- Decision: In a service definition, `volume.mount_path` is always required. If `volume.size` is present, the service requests creation of a new managed volume. If `volume.size` is absent and `volume.name` is present, the service mounts an existing volume by name. `lifecycle_policy` is valid only for service-created volumes.
- Rationale: This preserves a small declarative schema, avoids a separate mode flag, and keeps validation straightforward.
- Alternatives considered:
  - Add an explicit `mode: create|mount`: rejected because it duplicates information already implied by the field set.
  - Reuse the old `reuse_from_service` field: rejected because volumes are now named resources rather than service-derived references.

## Decision 3: Separate the user-facing volume name from the backing PVC name

- Decision: Treat the logical volume name as the API/FDL identifier and derive the Kubernetes PVC name from it using a deterministic suffix convention.
- Rationale: Users should reference stable logical names, not raw PVC names. This also decouples the API contract from Kubernetes implementation details while keeping resource lookup deterministic.
- Alternatives considered:
  - Expose the PVC name directly as the volume name: rejected because it leaks backend-specific naming and makes future storage abstraction harder.
  - Continue deriving storage identity only from service names: rejected because shared volumes must outlive or outscope a single service.

## Decision 4: Scope volumes to the authenticated user's namespace

- Decision: Volume CRUD and service-time attachment resolve only inside the namespace associated with the authenticated user. For bearer-authenticated users, namespace resolution follows the same `auth.GetUIDFromContext` plus `utils.EnsureUserNamespace` path already used in service creation. For admin/basic-auth flows, the existing service namespace behavior is preserved.
- Rationale: This matches the multitenancy model already present in OSCAR and satisfies the new namespace isolation requirement without inventing a second ownership mechanism.
- Alternatives considered:
  - Global cluster-wide volume names: rejected because they break tenant isolation.
  - Store a separate ownership registry outside Kubernetes namespaces: rejected because it adds state management complexity unnecessarily.

## Decision 5: Add dedicated `/system/volumes` handlers without expanding the backend interface

- Decision: Implement `/system/volumes` through dedicated handlers that use `back.GetKubeClientset()` plus shared resource helpers, rather than expanding `types.ServerlessBackend` with a second CRUD surface.
- Rationale: Managed volumes are backend-independent Kubernetes resources, and the current backend interface already exposes the kube client. This keeps the implementation localized and avoids a wider refactor across Kube, Knative, and fake backends.
- Alternatives considered:
  - Extend `ServerlessBackend` with `ListVolumes/CreateVolume/ReadVolume/DeleteVolume`: rejected because it adds churn to every backend implementation for logic that can live in shared helpers.
  - Implement handlers directly against raw Kubernetes code only: rejected because shared resource helpers remain useful for testability and consistency.

## Decision 6: Apply lifecycle policy only to service-created volumes

- Decision: `volume.lifecycle_policy` applies only when a service creates a new volume. `retain` keeps the volume and backing PVC after service deletion. `delete` removes them. A service that mounts an existing named volume never deletes it on service removal.
- Rationale: This keeps lifecycle rules predictable and avoids allowing consumer services to affect shared storage ownership.
- Alternatives considered:
  - Allow consumer services to override lifecycle on referenced volumes: rejected because it creates unsafe shared-resource semantics.
  - Apply lifecycle policy to volumes created directly through `/system/volumes`: rejected because those volumes are explicitly managed through the volume API instead.

## Decision 7: Block explicit volume deletion while the volume is still attached

- Decision: `/system/volumes/{name}` deletion rejects requests for volumes that are still attached to one or more services.
- Rationale: This avoids surprising data loss and keeps volume deletion behavior conservative by default.
- Alternatives considered:
  - Force delete attached volumes and let mounted services fail later: rejected because it is operationally unsafe.
  - Add a force-delete flag in this phase: rejected as unnecessary scope expansion.

## Decision 8: Preserve the current storage provisioning model for the first volume implementation

- Decision: Reuse the current managed-storage approach of creating ReadWriteMany PVCs with the existing NFS storage class for OSCAR-managed volumes.
- Rationale: This preserves currently validated behavior, avoids introducing new dependencies or low-level storage options, and matches the repository's current managed-volume provisioning approach.
- Alternatives considered:
  - Make storage class configurable per volume now: rejected because the spec explicitly keeps the model simple and low-level-storage-agnostic.
  - Introduce storage-backend pluggability now: rejected because it would materially expand the implementation scope.

## Decision 9: Keep service volume attachment immutable after service creation in this phase

- Decision: Once a service is created, changing its `volume` block through service update is rejected.
- Rationale: The current service-scoped storage implementation already enforces immutability, and keeping that rule avoids ambiguous migration and detach/reattach semantics while `/system/volumes` is introduced.
- Alternatives considered:
  - Allow mount-path-only updates: rejected because it still requires pod spec and reconciliation semantics not needed for the first volume API slice.
  - Allow full attachment swaps between volumes: rejected because it raises data migration and ownership questions.

## Decision 10: Expose simple status instead of a controller-style state machine

- Decision: Volume responses expose basic readiness/error information plus attachment references, and service responses expose whether a service has a volume attachment and its resolved volume name/status.
- Rationale: This gives users the operational visibility required by the spec without designing a new reconciliation controller or complex status graph.
- Alternatives considered:
  - Add a full multi-phase lifecycle/status controller: rejected as overengineering for the current feature scope.
  - Expose status only in logs/events: rejected because API clients need direct visibility.
