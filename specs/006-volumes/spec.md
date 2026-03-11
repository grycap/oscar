# Feature Specification: Managed Volumes for OSCAR Services

**Feature Branch**: `006-volumes`  
**Created**: 2026-03-09  
**Status**: Draft  
**Input**: User description: "Support for persistent volumes for OSCAR services"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create or Attach a Volume During Service Deployment (Priority: P1)

As an OSCAR user, I want to declare a volume in a service definition so stateful workloads can mount persistent POSIX-style storage at deployment time.

**Why this priority**: This is the core capability that enables stateful services without forcing users to manage storage outside the service deployment flow.

**Independent Test**: Submit a service definition with a `volume` block that either creates a new volume or references an existing volume by name, deploy the service, write test files, restart the service, and verify files remain available at the configured mount path.

**Acceptance Scenarios**:

1. **Given** a service definition with a `volume` block that requests a new volume, **When** the service is deployed, **Then** the system creates the volume, assigns it a name derived from the service name unless the user overrides it, and mounts it at the requested path.
2. **Given** a service definition with a `volume` block that names an existing volume in the caller namespace, **When** the service is deployed, **Then** the system mounts that existing volume at the requested path without creating a new one.
3. **Given** a running service with an attached volume, **When** the service is restarted or redeployed without changing its volume configuration, **Then** files previously written in the volume remain available.

---

### User Story 2 - Manage Volumes Through a Dedicated API (Priority: P1)

As an OSCAR user, I want to manage volumes through `/system/volumes` so I can inspect, create, list, and delete persistent storage independently from any single service definition.

**Why this priority**: A first-class volume API is required once volumes are shared resources that may outlive or be reused across services.

**Independent Test**: Create a volume through `/system/volumes`, verify it appears in list and read operations for the same user namespace, and delete it successfully when it has no attached services.

**Acceptance Scenarios**:

1. **Given** an authenticated user, **When** the user creates a volume through `/system/volumes`, **Then** the system stores it in that user's namespace and returns its assigned name and status.
2. **Given** existing volumes in the caller namespace, **When** the user lists `/system/volumes`, **Then** only volumes in that namespace are returned.
3. **Given** a volume in the caller namespace with no attached services, **When** the user deletes it through `/system/volumes`, **Then** the system removes the volume and its backing storage.

---

### User Story 3 - Control Volume Lifecycle Policy (Priority: P2)

As an OSCAR user, I want to choose whether a service-created volume is deleted or retained when the service is removed so the storage lifecycle matches my workload needs.

**Why this priority**: Lifecycle policy determines whether users can safely preserve data after service deletion or rely on automatic cleanup for temporary workloads.

**Independent Test**: Deploy one service with `volume.lifecycle_policy: retain` and another with `volume.lifecycle_policy: delete`, remove both services, and verify only the retained volume remains available for later reuse.

**Acceptance Scenarios**:

1. **Given** a service definition that creates a new volume with `lifecycle_policy: retain`, **When** the service is deleted, **Then** the volume remains available in `/system/volumes` and can be mounted by another service in the same namespace.
2. **Given** a service definition that creates a new volume with `lifecycle_policy: delete`, **When** the service is deleted, **Then** the system removes that volume and its backing storage.
3. **Given** a service definition that mounts an existing named volume, **When** the service is deleted, **Then** the system does not delete the shared volume because the service did not create it.

---

### User Story 4 - Keep Existing Services Compatible (Priority: P2)

As an OSCAR user, I want existing service definitions to continue working unchanged so volume support is additive and safe to adopt incrementally.

**Why this priority**: Backward compatibility protects current users and reduces rollout risk.

**Independent Test**: Deploy a service definition without volume configuration and verify behavior matches pre-feature deployments.

**Acceptance Scenarios**:

1. **Given** a service definition without volume settings, **When** it is deployed, **Then** deployment behavior is unchanged from current behavior.
2. **Given** mixed deployments with and without volumes, **When** they run concurrently, **Then** volume-enabled behavior applies only to services that explicitly request it.

### Edge Cases

- Volume configuration is present but required fields are missing or invalid.
- A user-specified volume name collides with an existing volume in the same namespace.
- An auto-generated volume name derived from the service name collides with an existing volume in the same namespace.
- A service references a volume name that does not exist in the caller namespace.
- A service attempts to mount a volume that belongs to a different user namespace.
- A deletion request targets a volume that is still attached to one or more services.
- Configured volume size is smaller than actual workload storage demand.
- Users expect dashboard volume browsing in this feature, but dashboard integration is handled in a different repository.
- Existing services that rely on object storage or external mounts must continue to function as-is.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose a dedicated `/system/volumes` API for listing, creating, reading, and deleting managed volumes.
- **FR-002**: The system MUST allow service definitions to include an optional top-level `volume` configuration.
- **FR-003**: The system MUST allow a service definition to request creation of a new volume by providing at least `volume.mount_path` and `volume.size`.
- **FR-004**: The system MUST assign a deterministic default volume name derived from the service name when a service definition creates a new volume and the user does not provide an explicit volume name.
- **FR-005**: The system MUST allow the user to specify a volume name in the service FDL when creating a new volume; otherwise, the system MUST generate an easily recognizable name derived from the service name. Volume creation through `/system/volumes` requires an explicit name.
- **FR-006**: The system MUST allow a service definition to mount an existing volume by specifying its name.
- **FR-007**: The system MUST provide persistent POSIX-style storage semantics for services with attached volumes.
- **FR-008**: The system MUST automatically provision and attach a new volume when deploying a service that requests volume creation.
- **FR-009**: The system MUST preserve volume data across service restarts and redeployments when the volume attachment configuration is unchanged.
- **FR-010**: The system MUST restrict volume visibility and access to the namespace of the authenticated user who created the volume.
- **FR-011**: The system MUST reject any attempt to mount or manage a volume that belongs to a different user namespace.
- **FR-012**: The system MUST allow `volume.lifecycle_policy` values `retain` and `delete` for service-created volumes.
- **FR-013**: The system MUST keep a service-created volume after service deletion when `volume.lifecycle_policy` is `retain`.
- **FR-014**: The system MUST remove a service-created volume and its backing storage after service deletion when `volume.lifecycle_policy` is `delete`.
- **FR-015**: The system MUST NOT delete an existing referenced volume when a consumer service is deleted.
- **FR-016**: The system MUST reject invalid volume configurations before deployment and return clear validation messages.
- **FR-017**: The system MUST reject service definitions that request creation of a new volume when the chosen or generated `volume.name` already exists in the caller namespace.
- **FR-018**: The system MUST reject volume names that do not satisfy Kubernetes DNS-1123 compatible naming rules.
- **FR-019**: The system MUST expose basic volume status and clear volume-related error information in `/system/volumes` responses and in service state responses when a service has a volume attachment.
- **FR-020**: The system MUST keep volume support independent from object-storage-based workflows.
- **FR-021**: The system MUST maintain compatibility with existing service definitions that do not use volumes.
- **FR-022**: The system MUST keep the volume configuration model simple and declarative rather than exposing full low-level storage configuration.

### Key Entities *(include if feature involves data)*

- **Volume**: A named persistent POSIX filesystem resource managed by OSCAR and scoped to a single user namespace.
- **Volume Attachment**: A relationship between a service and a volume, including the target `mount_path` used by the service.
- **Volume Lifecycle Policy**: The policy applied to a service-created volume when its creating service is deleted; allowed values are `retain` and `delete`.
- **User Namespace**: The logical ownership scope derived from the authenticated user; volumes are visible and reusable only inside this scope.
- **Volume Status**: Basic indication of volume readiness, attachment state, and volume-related error details.

## Assumptions

- Volume support is optional and disabled by default.
- Auto-generated volume names are derived deterministically from the service name and normalized to Kubernetes DNS-1123 compatible naming rules.
- User-provided volume names must also satisfy the same Kubernetes DNS-1123 compatible naming rules as generated names.
- The default `volume.lifecycle_policy` is `delete` when a service creates a new volume and the user does not specify a policy.
- This default does not apply to volumes created through `/system/volumes`, which are deleted only through explicit volume API requests.
- Volume lifecycle policy applies only when the service creates the volume; services that mount an existing named volume do not control that volume's deletion.
- Cross-namespace volume sharing is out of scope for this feature.
- Advanced multi-phase volume state tracking is out of scope for this feature.
- Dashboard access to volume files is out of scope in this repository and handled in `oscar-dashboard`.

## Non-Goals

- Exposing full low-level storage platform configuration in service definitions or `/system/volumes`.
- Replacing existing object-storage triggers or external mount workflows.
- Implementing volume file access flows in the dashboard repository (`oscar-dashboard`).
- Supporting cross-namespace volume sharing.
- Introducing advanced storage features such as snapshots, cloning, or topology controls.
- Introducing advanced quota, ACL, or per-directory permission management inside a volume.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: At least 95% of services with attached volumes retain test files after restart in validation tests.
- **SC-002**: At least 95% of services with attached volumes retain test files after redeployment with unchanged volume settings.
- **SC-003**: 100% of invalid volume configurations and cross-namespace volume references are rejected before deployment with a human-readable reason.
- **SC-004**: In validation runs, 100% of `/system/volumes` list responses return only volumes owned by the caller namespace.
- **SC-005**: In validation runs, 100% of service-created volumes with `lifecycle_policy: retain` remain available after service deletion.
- **SC-006**: In validation runs, 100% of service-created volumes with `lifecycle_policy: delete` are removed after service deletion.
- **SC-007**: In validation runs, 100% of existing service definitions without volume settings deploy successfully with no behavior regression.
