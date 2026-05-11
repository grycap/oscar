# Quickstart: Service Deployment Visibility

## Goal

Retrieve the current deployment summary for an OSCAR service and inspect current
or recent deployment logs across OSCAR service types.

## Prerequisites

- Access to the OSCAR manager API with credentials that can already read the
  target service.
- A service that either has current runtime resources or recent deployment logs
  available through the existing operational log source.

## 1. Read the current deployment summary

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "https://YOUR_OSCAR_MANAGER/system/services/SERVICE_NAME/deployment"
```

Expected result:
- HTTP `200`.
- The response includes the current deployment state, reason, last transition
  time, and aggregate deployment counts when available.

## 2. Read current or recent service-level deployment logs

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "https://YOUR_OSCAR_MANAGER/system/services/SERVICE_NAME/deployment/logs?timestamps=true&tailLines=200"
```

Expected result:
- HTTP `200`.
- The response states whether deployment logs are available and returns recent
  deployment log entries when they exist.

## 3. Validate partial-failure diagnosis

1. Request the deployment summary for a multi-instance service.
2. Request service-level deployment logs for the same service.

Expected result:
- The summary shows that only a subset of instances is affected.
- The service-level deployment logs return evidence consistent with the reported
  failure.

## 4. Validate unavailable status with last-attempt log fallback

1. Request deployment status for a service that has no current deployment or
   runtime representation.
2. Request deployment logs for the same service.

Expected result:
- The status response returns `unavailable`.
- The log response may still return recent last-attempt logs when those logs are
  already available through the existing operational log source.

## 5. Validate unavailable deployment handling without fallback logs

Request deployment status and logs for a service that has neither a current
runtime representation nor recent last-attempt logs available.

Expected result:
- The status response is explicit about the unavailable state.
- The log response is explicit that logs are unavailable and does not imply a
  healthy deployment.

## 6. Planned validation commands for implementation

Run targeted tests for touched packages:

```bash
go test ./pkg/handlers ./pkg/types ./pkg/backends/...
```

Regenerate Swagger/OpenAPI docs:

```bash
go generate ./...
```

Validate docs rendering when feasible:

```bash
mkdocs serve
```

In non-interactive validation, `mkdocs build -q` is an acceptable substitute
for `mkdocs serve`.
