# OSCAR API 

OSCAR exposes a secure REST API available at the Kubernetes master's node IP
through an Ingress Controller. This API has been described following the
[OpenAPI Specification](https://www.openapis.org/) and it is available below.

> ℹ️
>
> The bearer token used to run a service can be either the OSCAR [service access token](invoking-sync.md#service-access-tokens) or the [user's Access Token](integration-egi.md#obtaining-an-access-token) if the OSCAR cluster is integrated with EGI Check-in.

The generated OpenAPI document includes the managed-volume endpoints under `/system/volumes` and the updated service schema with the optional `volume` block.

!!swagger swagger.yaml!!
