# Deployment visibility

Deployment visibility endpoints expose the current deployment summary and recent
deployment logs for a specific service:

- `/system/services/{serviceName}/deployment`
- `/system/services/{serviceName}/deployment/logs`

The service list endpoint can also include a per-service deployment summary by
requesting:

- `/system/services?include=deployment`

The service detail endpoint can embed the same deployment summary in the
service response by requesting:

- `/system/services/{serviceName}?include=deployment`

These endpoints are additive and service-scoped. They reuse the existing
service-access rules, return `unavailable` when OSCAR cannot inspect a current
runtime representation, and keep deployment evidence separate from job
execution logs under `/system/logs/...`.

