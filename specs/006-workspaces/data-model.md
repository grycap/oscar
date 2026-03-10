# Data Model: Persistent Workspaces for OSCAR Services

## Entity: Service

- Description: Existing OSCAR service definition extended with optional workspace configuration.
- Key fields (existing): `name`, `image`, `script`, `input`, `output`, `mount`, etc.
- New field:
  - `workspace` (optional): `WorkspaceConfig`
- Relationships:
  - One `Service` optionally owns exactly one `Workspace`.

Validation rules:
- If `workspace` is omitted, behavior is unchanged.
- If `workspace` is present, `size` and `mount_path` are required and validated.
- Service update cannot mutate existing workspace config in this phase (immutable constraint).

## Entity: WorkspaceConfig

- Description: Declarative user input that requests managed persistent workspace storage.
- Fields:
  - `size` (string): requested capacity in Kubernetes quantity format (example: `10Gi`).
  - `mount_path` (string): absolute path inside the service container where workspace is mounted.
- Relationships:
  - Belongs to one `Service`.

Validation rules:
- `size` must be syntactically valid and above minimal non-zero threshold.
- `mount_path` must be absolute and must not collide with reserved internal mount paths used by OSCAR.

## Entity: Workspace

- Description: Runtime managed persistent storage attachment associated with a service.
- Fields:
  - `service_name` (string)
  - `requested_size` (string)
  - `mount_path` (string)
  - `lifecycle` (enum): `delete_with_service` (current phase only)
- Relationships:
  - Owned by one `Service`.

Validation rules:
- Must only exist when `Service.workspace` exists.
- Must be cleaned up when owning service is deleted.

## Entity: WorkspaceLifecycleStatus

- Description: Operational state exposed in service status metadata.
- Fields:
  - `phase` (enum): `pending`, `provisioning`, `ready`, `error`, `deleting`, `deleted`
  - `message` (string, optional): human-readable detail for failures/progress.
  - `last_transition_time` (datetime string, optional)
- Relationships:
  - Associated with one `Workspace`.

Validation rules:
- `phase` must be a valid enum value.
- `message` required when `phase=error`.

## State Transitions

- `pending -> provisioning`: workspace request accepted.
- `provisioning -> ready`: storage created and attached.
- `provisioning -> error`: provisioning/attachment failure.
- `error -> provisioning`: retry flow on reconcile/update without workspace mutation.
- `ready -> deleting`: service deletion initiated.
- `deleting -> deleted`: storage removed successfully.
