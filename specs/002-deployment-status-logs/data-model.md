# Data Model: Service Deployment Visibility

## Entity: ServiceDeploymentStatus

- Description: Current deployment summary returned for one OSCAR service.
- Fields:
  - `service_name` (string)
  - `namespace` (string)
  - `state` (enum): `pending`, `ready`, `degraded`, `failed`, `unavailable`
  - `reason` (string, optional): human-readable current-state explanation
  - `last_transition_time` (datetime string, optional)
  - `active_instances` (integer): aggregate count of currently observed active
    runtime instances when that count is observable for the selected runtime
    source
  - `affected_instances` (integer): aggregate count of currently affected
    runtime pods when that count is observable for the selected runtime source
  - `resource_kind` (enum): `exposed_service`, `knative_service`,
    `unavailable`
- Relationships:
  - Belongs to one `Service`.

Validation rules:
- `service_name` is required.
- `state` is required.
- `affected_instances` must be between `0` and `active_instances`.
- `reason` is required when `state` is `failed`, `degraded`, or `unavailable`.
- `resource_kind=unavailable` implies `state=unavailable`.
- `resource_kind=knative_service` with a ready Knative service reports
  `affected_instances=0`; normal autoscaling churn does not, by itself, imply a
  partially affected service.

## Entity: DeploymentLogStream

- Description: Response wrapper for service-level deployment log retrieval.
- Fields:
  - `service_name` (string)
  - `available` (boolean)
  - `message` (string, optional): explicit reason when logs are unavailable or
    when the returned logs come from the last runtime attempt after current
    runtime state became unavailable
  - `entries` (array of `DeploymentLogEntry`)
- Relationships:
  - Belongs to one `Service`.
  - Contains zero or more `DeploymentLogEntry` records.

Validation rules:
- `service_name` is required.
- `message` is required when `available=false`.
- `entries` may be empty when logs are not yet available.

## Entity: DeploymentLogEntry

- Description: A recent deployment log line returned to the caller.
- Fields:
  - `timestamp` (datetime string, optional)
  - `message` (string)
- Relationships:
  - Belongs to one `DeploymentLogStream`.

Validation rules:
- `message` is required.

## State Transitions

### ServiceDeploymentStatus

- `pending -> ready`: current runtime resources become healthy.
- `pending -> failed`: startup or readiness cannot complete.
- `ready -> degraded`: only a subset of active instances remain healthy.
- `degraded -> ready`: all active instances recover.
- `ready|degraded|failed -> unavailable`: the service no longer has a current
  deployment representation or runtime lookup fails.
- For `knative_service`, normal autoscaling churn does not create a
  `ready -> degraded` transition while the Knative service condition remains
  ready.

### DeploymentLogStream

- `available=false -> available=true`: recent deployment logs become retrievable.
- `available=true -> available=false`: the current or recent log window is no
  longer retrievable.
- `available=true` may coexist with `ServiceDeploymentStatus.state=unavailable`
  when recent last-attempt logs still exist in the current operational log
  source.
