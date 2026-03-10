# Feature Specification: Persistent Workspaces for OSCAR Services

**Feature Branch**: `006-workspaces`  
**Created**: 2026-03-09  
**Status**: Draft  
**Input**: User description: "Support for persistent volumes for OSCAR services"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Request Persistent Workspace in Service Definition (Priority: P1)

As an OSCAR user, I want to declare a persistent workspace in a service definition so stateful workloads can keep POSIX-style file data across restarts.

**Why this priority**: This is the core capability that unlocks workloads that cannot run correctly with object storage alone.

**Independent Test**: Submit a service definition with workspace settings, deploy it, write test files, restart the service, and verify files remain available at the configured mount path.

**Acceptance Scenarios**:

1. **Given** a service definition that includes workspace configuration, **When** the service is deployed, **Then** the service starts with a mounted persistent workspace.
2. **Given** a running service with a workspace, **When** the service is restarted, **Then** files previously written in the workspace remain available.
3. **Given** a service definition that sets `workspace.reuse_from_service`, **When** the service is deployed, **Then** the service mounts the existing workspace PVC of the referenced service.

---

### User Story 2 - Keep Existing Services Compatible (Priority: P2)

As an OSCAR user, I want existing service definitions to continue working unchanged so workspace support is additive and safe to adopt incrementally.

**Why this priority**: Backward compatibility protects current users and reduces rollout risk.

**Independent Test**: Deploy a service definition without workspace configuration and verify behavior matches pre-feature deployments.

**Acceptance Scenarios**:

1. **Given** a service definition without workspace settings, **When** it is deployed, **Then** deployment behavior is unchanged from current behavior.
2. **Given** mixed deployments (with and without workspaces), **When** they run concurrently, **Then** workspace-enabled behavior applies only to services that explicitly request it.

### Edge Cases

- Workspace configuration is present but required fields are missing or invalid.
- Configured workspace size is smaller than actual workload storage demand.
- Service deletion occurs while users still expect workspace data availability.
- A service references `workspace.reuse_from_service` but the referenced workspace PVC does not exist.
- Users expect dashboard workspace browsing in this feature, but dashboard integration is handled in a different repository.
- Existing services that rely on object storage or external mounts must continue to function as-is.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST allow service definitions to include an optional top-level workspace configuration.
- **FR-002**: The system MUST allow users to define `workspace.mount_path` and either `workspace.size` (new PVC) or `workspace.reuse_from_service` (existing PVC).
- **FR-003**: The system MUST provide persistent POSIX-style storage semantics for workspace-enabled services.
- **FR-004**: The system MUST automatically provision and attach workspace storage when deploying a workspace-enabled service.
- **FR-005**: The system MUST preserve workspace data across service restarts and redeployments when workspace configuration is unchanged.
- **FR-006**: The system MUST reject invalid workspace configurations before deployment and return clear validation messages.
- **FR-007**: The system MUST expose basic workspace status and clear workspace-related error information in service state responses.
- **FR-008**: The system MUST remove workspace storage when the associated owner service is removed under the default lifecycle behavior.
- **FR-009**: The system MUST keep workspace support independent from object-storage-based workflows.
- **FR-010**: The system MUST maintain compatibility with existing service definitions that do not use workspaces.
- **FR-011**: The system MUST keep the workspace configuration model simple and declarative rather than exposing full low-level storage configuration.
- **FR-012**: The system MUST reject `workspace` definitions that set both `size` and `reuse_from_service`.
- **FR-013**: The system MUST reject `workspace.reuse_from_service` when it references the same service name.
- **FR-014**: The system MUST skip workspace PVC creation and deletion when `workspace.reuse_from_service` is used, and instead bind to the referenced service workspace PVC.

### Key Entities *(include if feature involves data)*

- **Workspace**: A persistent POSIX filesystem mounted at a configured path, either owned by a service or reused by another service.
- **Workspace Owner Service**: The service definition that creates the workspace PVC.
- **Workspace Consumer Service**: A service definition that mounts an existing workspace PVC via `workspace.reuse_from_service`.
- **Workspace Status**: Basic service-level indication of workspace availability and workspace-related error details.

## Assumptions

- Workspace support is optional and disabled by default.
- Workspace lifecycle default is tied to the service lifecycle only for owner-created workspaces; reused workspaces are not deleted by consumer-service deletion.
- Future lifecycle options (for example, retaining workspace after service deletion) are out of scope for this feature.
- Advanced multi-phase workspace state tracking is out of scope for this feature.
- Dashboard access to workspace files is out of scope in this repository and handled in `oscar-dashboard`.

## Non-Goals

- Exposing full low-level storage platform configuration in service definitions.
- Replacing existing object-storage triggers or external mount workflows.
- Implementing workspace file access flows in the dashboard repository (`oscar-dashboard`).
- Introducing a dedicated workspace API resource separate from existing service APIs.
- Introducing advanced storage features such as snapshots, cloning, or topology controls.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: At least 95% of workspace-enabled services retain test files after restart in validation tests.
- **SC-002**: At least 95% of workspace-enabled services retain test files after redeployment with unchanged workspace settings.
- **SC-003**: 100% of invalid workspace configurations are rejected before deployment with a human-readable reason.
- **SC-004**: In validation runs, 100% of existing service definitions without workspace settings deploy successfully with no behavior regression.
