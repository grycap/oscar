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

- `GET /system/secrets?key=API_KEY` returns the value of a secret key stored in
  the current user's secret (404 if it does not exist). The user secret lives
  in the user namespace and is named after the formatted user UID.
- `PUT /system/secrets` merges the given key-value pairs into the current
  user's secret. The secrets are provided as a JSON object and the response is
  `204 No Content` on success.
- `GET /system/services/{serviceName}/secrets?key=API_KEY` returns the value of
  the requested secret key of a specific service (404 if it does not exist).
- `PUT /system/services/{serviceName}/secrets` merges the given key-value pairs
  into the service secrets. Keys that do not exist yet are created and keys not
  present in the request are preserved. The secrets are provided as a JSON
  object and the response is `204 No Content` on success.

The reserved keys `refresh_token`, `accessKey`, `secretKey`, and `oidc_uid`
cannot be modified through any of the PUT endpoints.

```bash
# Get a secret of the current user
curl -H "Authorization: Bearer YOUR_TOKEN" \
     "https://oscar-cluster-remote/system/secrets?key=API_KEY"

# Add or update secrets of the current user
curl -X PUT "https://oscar-cluster-remote/system/secrets" \
     -H "Authorization: Bearer YOUR_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"API_KEY":"new-value","ANOTHER_KEY":"another-value"}'

# Get a secret of a specific service
curl -H "Authorization: Bearer YOUR_TOKEN" \
     "https://oscar-cluster-remote/system/services/cowsay/secrets?key=API_KEY"

# Add or update secrets of a specific service
curl -X PUT "https://oscar-cluster-remote/system/services/cowsay/secrets" \
     -H "Authorization: Bearer YOUR_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"API_KEY":"new-value","ANOTHER_KEY":"another-value"}'
```

> ❗️
> If you have basic authentication, replace `-H "Authorization: Bearer ..."` with `-u "user:password"`, cURL automatically generates the `Authorization: Basic [Base64]` header.

!!swagger swagger.yaml!!
