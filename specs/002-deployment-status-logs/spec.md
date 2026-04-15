# Feature Specification: Service Deployment Visibility

**Feature Branch**: `002-deployment-status-logs`  
**Created**: 2026-04-04  
**Status**: Draft  
**Input**: User description: "I want to obtain information about the status of the OSCAR service deployment and the logs of the deployment so that the user has visibility on the status and possible causes of failures. Create a new branch and start drafting a spec."

## Clarifications

### Session 2026-04-04

- Q: Which interface should expose deployment status and logs in this phase? → A: API only
- Q: How much deployment history should this phase cover? → A: Current deployment status and current/recent deployment logs only
- Q: Should this feature expose an instance list or instance-specific log access? → A: No; service-level summary only
- Q: Which OSCAR service types should this feature cover? → A: All service types; return unavailable when deployment visibility does not apply
- Q: How should deployment logs handle sensitive values? → A: Return raw deployment logs to authorized service viewers
- Q: What should happen when no current deployment/runtime exists but recent logs from the last attempt may still exist? → A: Status stays unavailable; logs may return recent last-attempt logs only when already available through existing log sources

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inspect Deployment Health (Priority: P1)

As an authorized OSCAR user, I want to request deployment status for any OSCAR service through a programmatic interface and immediately understand whether its deployment is ready, still starting, degraded, failed, or unavailable so that I can decide whether the service can be used.

**Why this priority**: Users cannot act on a service until they know whether the deployment itself is healthy. This is the minimum slice that delivers operational visibility.

**Independent Test**: Can be fully tested by viewing a service in multiple lifecycle states and confirming that the displayed deployment state, time of last change, and current reason match the actual service condition.

**Acceptance Scenarios**:

1. **Given** a service whose deployment is still starting, **When** the user requests its deployment status, **Then** the system returns a non-ready state with the most recent status change time and a human-readable reason.
2. **Given** a service whose deployment is fully available, **When** the user requests its deployment status, **Then** the system returns the deployment as ready and does not display stale failure information as current status.
3. **Given** a service whose deployment resources cannot be found, **When** the user requests its deployment status, **Then** the system reports the deployment as unavailable instead of implying that the service is healthy.
4. **Given** an OSCAR service type for which there is no current deployment or runtime representation to inspect, **When** the user requests deployment status, **Then** the system returns an explicit unavailable state for that service.

---

### User Story 2 - Review Deployment Logs (Priority: P2)

As an authorized OSCAR user, I want to retrieve deployment logs for a service through a programmatic interface so that I can understand startup progress and identify the messages associated with a failed or stalled deployment.

**Why this priority**: Once a deployment is not healthy, logs are the fastest way to confirm the likely cause and reduce trial-and-error.

**Independent Test**: Can be fully tested by requesting deployment logs for a service with recent startup activity and confirming that the user can access ordered log entries without relying on job execution logs.

**Acceptance Scenarios**:

1. **Given** a service with available deployment logs, **When** the user requests deployment logs, **Then** the system returns recent log entries with timestamps in a consistent order.
2. **Given** a service with no deployment logs yet, **When** the user requests deployment logs, **Then** the system explains that logs are not yet available instead of returning an empty or misleading success state.
3. **Given** a user who can view the service, **When** the user requests deployment logs, **Then** the system grants access without requiring a separate permission model for the same service.
4. **Given** a service with no current deployment/runtime representation but recent logs from the last runtime attempt remain available through the existing log source, **When** the user requests deployment logs, **Then** the system may return those recent logs while the deployment status remains unavailable.

---

### User Story 3 - Diagnose Failures Faster (Priority: P3)

As an authorized OSCAR user, I want deployment status and deployment logs to reinforce each other so that I can quickly understand the likely cause of a failure and choose the next troubleshooting step.

**Why this priority**: Visibility is only useful if it shortens diagnosis time. Correlating the status summary with the relevant log evidence increases user confidence and reduces support effort.

**Independent Test**: Can be fully tested by reproducing a failed deployment, reviewing the status summary and deployment logs together, and confirming that the likely cause of failure can be identified without inspecting cluster internals directly.

**Acceptance Scenarios**:

1. **Given** a deployment that fails during startup, **When** the user reviews the deployment status and logs, **Then** the system presents a failure reason that is consistent with the recent log evidence.
2. **Given** a deployment with multiple active service instances and only some are failing, **When** the user reviews deployment status, **Then** the system indicates that the deployment is only partially healthy and shows how many instances are affected.

### Edge Cases

- A deployment changes state while the user is viewing it; refreshed data must show the newest state without mixing old and new evidence.
- A deployment is healthy now but has very recent failure messages; the current state must remain accurate while recent logs remain available for diagnosis.
- Deployment logs are temporarily unavailable because log data has not yet been produced or can no longer be retrieved; the user must receive an explicit explanation.
- A service is visible to the user, but the deployment metadata cannot be resolved; the system must show an unavailable state rather than a generic success response.
- A service has multiple instances and only one instance is failing; the system must report the deployment as partially affected at the service-summary level even though it does not expose an instance list.
- A Knative-managed service is scaling up or down and some pods are temporarily not ready; the system must not report a degraded state when the Knative service itself is still ready.
- A service has no current deployment/runtime representation, but recent logs from the last runtime attempt still exist; the system must keep status unavailable and only return those logs when they are already available through the existing operational log source.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide deployment visibility for each OSCAR service that a user is authorized to view, including returning an explicit unavailable state when deployment visibility does not apply to the current runtime representation.
- **FR-002**: The system MUST present a clear current deployment state for a service using user-understandable status values such as pending, ready, degraded, failed, or unavailable.
- **FR-003**: The system MUST show the most recent known deployment state change time for the service.
- **FR-004**: The system MUST show a human-readable reason or summary for the current deployment state whenever such evidence is available.
- **FR-005**: The system MUST allow authorized users to access deployment logs for a service from the same programmatic service context as the deployment status.
- **FR-006**: Deployment logs MUST be distinguishable from service invocation or job execution logs so users do not confuse startup failures with runtime job failures.
- **FR-007**: Deployment log entries MUST include timestamps and be presented in a consistent order.
- **FR-008**: When a service has multiple active instances, the system MUST indicate at the service-summary level whether the deployment issue affects all instances or only a subset, including the count of affected instances when available.
- **FR-009**: When deployment status or logs are unavailable, the system MUST return an explicit unavailable message and MUST NOT imply that the deployment is healthy.
- **FR-010**: The system MUST respect the existing service access rules so that users only see deployment status and logs for services they are already allowed to access.
- **FR-011**: Users MUST be able to request refreshed deployment status and deployment logs so they can observe recovery or failure progression over time.
- **FR-012**: In this phase, deployment visibility MUST be exposed through a programmatic interface only.
- **FR-013**: Dashboard and CLI exposure for deployment visibility are out of scope for this phase.
- **FR-014**: In this phase, deployment visibility MUST cover only the current deployment snapshot and current or recent deployment logs.
- **FR-015**: Full deployment history, historical rollout comparisons, and audit trail reporting are out of scope for this phase.
- **FR-016**: In this phase, the API MUST NOT expose an instance list or instance-specific deployment log endpoint.
- **FR-017**: The feature MUST apply across OSCAR service types, but MAY return an unavailable state for services that do not expose a current deployment or runtime representation suitable for deployment visibility.
- **FR-018**: In this phase, the API MUST return raw deployment log content to authorized service viewers and MUST NOT redact log values.
- **FR-019**: When a service has no current deployment or runtime representation, the status endpoint MUST return `unavailable`; the log endpoint MAY still return recent logs from the last runtime attempt only when those logs are already available through the existing log source.
- **FR-020**: For `knative_service` runtimes, the top-level deployment state MUST follow the Knative service readiness condition, and transient autoscaling pod churn MUST NOT by itself cause the state to become `degraded` while the Knative service remains ready.

### Key Entities *(include if feature involves data)*

- **Service Deployment Snapshot**: A user-facing summary of one service's current deployment condition, including service identity, current state, latest reason, time of last change, and whether all or only some active instances are affected at an aggregate level.
- **Deployment Log Entry**: A single time-stamped recent message produced during service startup, readiness checks, shutdown, or failure handling and associated with the current service deployment.
- **Deployment Visibility Record**: The combined view of deployment state and recent deployment log evidence that helps a user determine whether a service is usable and why it may have failed.

### Assumptions

- The feature applies to users who are already allowed to view the target OSCAR service.
- Deployment visibility refers to service startup, readiness, and availability, not to asynchronous job execution history.
- For `knative_service` runtimes, aggregate pod counters may still provide context, but a ready Knative service does not report affected instances during normal autoscaling.
- Existing retention rules for operational logs remain unchanged; this feature exposes the recent deployment evidence that is already available.
- The feature spans OSCAR service types uniformly at the API level, but runtime coverage depends on whether the current service implementation exposes deployment-visible resources.
- Deployment-log access follows existing service-view permissions; no additional log-specific redaction is required in this phase.
- Last-attempt deployment logs are returned only when they can be obtained from the same existing operational log source without introducing a separate complex retrieval path.
- This phase does not include a dashboard workflow; any graphical presentation is deferred to a later stage.
- This phase does not introduce a historical deployment timeline or audit trail beyond current state and recent deployment evidence.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In at least 95% of evaluated cases, an authorized user can determine within 30 seconds whether a service deployment is ready, pending, degraded, failed, or unavailable.
- **SC-002**: In at least 90% of evaluated failed deployments with recent log evidence, users can retrieve the relevant deployment logs with no more than two programmatic requests from the initial deployment-status request.
- **SC-003**: In at least 90% of evaluated failure scenarios, the system shows either a human-readable failure cause or an explicit explanation that the cause is currently unavailable.
- **SC-004**: At least 90% of evaluated users correctly distinguish deployment failures from job execution failures on their first attempt.
- **SC-005**: Support requests caused by unclear service deployment state decrease by at least 50% for users adopting this feature.
- **SC-006**: In at least 90% of evaluated partial-failure scenarios, users can determine from the service summary that only a subset of the deployment is affected without needing an instance list.
