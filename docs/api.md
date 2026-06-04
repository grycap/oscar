# OSCAR API 

OSCAR exposes a secure REST API available at the Kubernetes master's node IP
through an Ingress Controller or a Gateway API HTTPRoute. This API has been described following the
[OpenAPI Specification](https://www.openapis.org/) and it is available below.

> ℹ️
>
> The bearer token used to run a service can be either the OSCAR [service access token](invoking-sync.md#service-access-tokens) or the [user's Access Token](integration-egi.md#obtaining-an-access-token) if the OSCAR cluster is integrated with EGI Check-in.

The `/system/quotas` endpoints report CPU, memory, volume, and MinIO bucket
quota information when those subsystems are enabled. Administrators can update
per-user MinIO bucket limits through `/system/quotas/user/{userId}` by setting
the `minio.buckets` and `minio.storage_per_bucket` fields.

!!swagger swagger.yaml!!
