# Data Model: Managed Volumes for OSCAR Services

## Entity: Service

- Description: Existing OSCAR service definition extended with optional managed-volume attachment input.
- Key existing fields: `name`, `image`, `script`, `input`, `output`, `mount`, `expose`, `owner`, `namespace`.
- New or changed fields:
  - `volume` (optional): `ServiceVolumeConfig`
  - `volume_status` (response-only): `ServiceVolumeStatus`
- Relationships:
  - One `Service` may have zero or one `VolumeAttachment`.
  - A `Service` may create a new `ManagedVolume` or reference an existing one.

Validation rules:
- If `volume` is omitted, behavior is unchanged.
- If `volume` is present, `mount_path` is required and must be absolute.
- If `volume.size` is present, the service creates a new volume; `name` is optional and `lifecycle_policy` defaults to `delete` if omitted.
- If `volume.size` is absent, `volume.name` is required and the service mounts an existing volume.
- `lifecycle_policy` is valid only when the service creates a new volume.
- Service update cannot mutate an existing `volume` attachment in this phase.

## Entity: ServiceVolumeConfig

- Description: Declarative FDL/API input used by a service to create or attach a managed volume.
- Fields:
  - `name` (string, optional for create / required for mount): logical volume name. If omitted during create, OSCAR derives one from the service name.
  - `size` (string, optional): requested capacity in Kubernetes quantity format when creating a new volume.
  - `mount_path` (string, required): absolute path inside the service container where the volume is mounted.
  - `lifecycle_policy` (enum, optional): `retain` or `delete`; applies only to service-created volumes.
- Relationships:
  - Belongs to one `Service`.
  - Resolves to one `ManagedVolume`.

Validation rules:
- `mount_path` must be absolute and must not overlap OSCAR reserved internal paths.
- `size`, when set, must be a valid positive Kubernetes quantity.
- `name`, when set, must satisfy the platform naming rules.
- A config that omits both `size` and `name` is invalid.
- A config that sets `lifecycle_policy` without `size` is invalid.

## Entity: ManagedVolume

- Description: Namespace-scoped named persistent filesystem resource managed by OSCAR.
- Fields:
  - `name` (string): logical volume name used in `/system/volumes` and service definitions.
  - `namespace` (string): Kubernetes namespace in which the volume is owned.
  - `pvc_name` (string): derived backing PVC name.
  - `size` (string): requested capacity.
  - `owner_user` (string): authenticated user who owns the namespace-scoped volume.
  - `created_by_service` (string, optional): service name that originally created the volume, when applicable.
  - `creation_mode` (enum): `service` or `api`.
  - `lifecycle_policy` (enum, optional): `retain` or `delete` for service-created volumes; empty for API-created volumes.
  - `status` (`VolumeStatus`): readiness and error state.
- Relationships:
  - One `ManagedVolume` may have zero or more `VolumeAttachment` records.
  - One `ManagedVolume` belongs to exactly one user namespace.

Validation rules:
- `name` must be unique within the namespace.
- `pvc_name` is derived deterministically from `name` and cannot be user-specified.
- `size` is immutable after creation in this phase.

## Entity: VolumeAttachment

- Description: Runtime relationship between a service and a managed volume.
- Fields:
  - `service_name` (string)
  - `namespace` (string)
  - `volume_name` (string)
  - `mount_path` (string)
  - `attachment_mode` (enum): `created` or `mounted`
- Relationships:
  - Each `VolumeAttachment` belongs to one `Service` and one `ManagedVolume`.
  - A `ManagedVolume` can have multiple `VolumeAttachment` records.

Validation rules:
- `volume_name` must resolve inside the same namespace as the service.
- `mount_path` must match the service's declared `volume.mount_path`.
- Duplicate attachments from the same service to different volumes are not allowed.

## Entity: VolumeStatus

- Description: Minimal operational status exposed in `/system/volumes` and service responses.
- Fields:
  - `phase` (enum): `pending`, `ready`, `in_use`, `error`, `deleting`, `deleted`
  - `message` (string, optional): human-readable error or progress detail.
  - `attachment_count` (integer): number of attached services.
  - `last_transition_time` (datetime string, optional)
- Relationships:
  - Associated with one `ManagedVolume`.

Validation rules:
- `phase` must be a valid enum value.
- `message` is required when `phase` is `error`.
- `attachment_count` must be zero or greater.

## Entity: ServiceVolumeStatus

- Description: Response-only service view of the attached volume.
- Fields:
  - `enabled` (boolean)
  - `name` (string, optional): resolved logical volume name.
  - `phase` (enum, optional): mirrored from the attached volume.
  - `error` (string, optional)
- Relationships:
  - Associated with one `Service`.

Validation rules:
- `enabled=false` implies other fields are empty.
- `name` is required when `enabled=true`.

## State Transitions

### ManagedVolume

- `pending -> ready`: volume PVC has been created or resolved successfully.
- `ready -> in_use`: one or more services are attached to the volume.
- `in_use -> ready`: all services detach and the volume remains retained.
- `pending|ready|in_use -> error`: provisioning, resolution, or deletion fails.
- `ready -> deleting`: explicit delete request for a detached volume, or service deletion with `lifecycle_policy=delete`.
- `deleting -> deleted`: backing PVC and metadata removed successfully.

### VolumeAttachment

- `requested -> attached`: service deploy succeeds and the volume is mounted.
- `requested -> error`: referenced volume cannot be resolved or mounted.
- `attached -> detached`: service is deleted.
