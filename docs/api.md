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

The `/system/secrets` endpoints manage the service environment secrets without
redeploying the service:

- `GET /system/secrets` lists the secrets of the accessible services. Values
  are only returned for services owned by the caller.
- `GET /system/secrets/{serviceName}` returns the secrets of a specific service.
- `PUT /system/secrets/{serviceName}` merges the given key-value pairs into the
  service secrets. Keys that do not exist yet are created and keys not present
  in the request are preserved. The reserved `refresh_token` key cannot be
  modified.

```bash
# List the secrets of the accessible services
curl -H "Authorization: Bearer YOUR_TOKEN" \
     "https://oscar-cluster-remote/system/secrets"

# Get the secrets of a specific service
curl -H "Authorization: Bearer YOUR_TOKEN" \
     "https://oscar-cluster-remote/system/secrets/cowsay"

# Add or update secrets of a specific service
curl -X PUT "https://oscar-cluster-remote/system/secrets/cowsay" \
     -H "Authorization: Bearer YOUR_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"secrets":{"API_KEY":"new-value","ANOTHER_KEY":"another-value"}}'
```

> ❗️
> If you have basic authentication, replace `-H "Authorization: Bearer ..."` with `-u "user:password"`, cURL automatically generates the `Authorization: Basic [Base64]` header.

!!swagger swagger.yaml!!
