# Passing values per invocation

This example shows how an OSCAR service can select a value at invocation time
without updating its persistent environment configuration.

The service defines `MESSAGE=configured-value`. The user script applies the
following precedence, from highest to lowest:

1. The `X-Message` HTTP header in a synchronous invocation.
2. The `environment.MESSAGE` field in a JSON payload.
3. The `MESSAGE` environment variable configured in the service.

HTTP headers are exposed by the watchdog using CGI-style names. Therefore,
`X-Message` is available to the script as `Http_X_Message`; it does not replace
the original `MESSAGE` variable automatically.

## Deployment

Replace `oscar-cluster` in [`pass-envvars.yaml`](pass-envvars.yaml) with the
identifier of your cluster in OSCAR CLI, then deploy the service:

```sh
oscar-cli apply pass-envvars.yaml
```

The example uses `ghcr.io/grycap/cowsay` because it includes `jq`, which the
script uses to read the optional JSON payload.

## Synchronous invocation

Without an invocation-specific value, the script uses the service
configuration:

```sh
curl -H "Authorization: Bearer <SERVICE_TOKEN>" \
  -d '{}' \
  https://<CLUSTER_ENDPOINT>/run/pass-envvars
```

To provide a value for one invocation, add the `X-Message` header:

```sh
curl -H "Authorization: Bearer <SERVICE_TOKEN>" \
  -H "X-Message: synchronous-value" \
  -d '{}' \
  https://<CLUSTER_ENDPOINT>/run/pass-envvars
```

The relevant output is:

```text
Configured MESSAGE: configured-value
Header Http_X_Message: synchronous-value
Payload environment.MESSAGE: <unset>
Effective MESSAGE: synchronous-value
Selected source: HTTP header
```

Please consult the OSCAR-CLI documentation to review the support for this functionality.

## Asynchronous invocation

Arbitrary request headers are not propagated to the Kubernetes Job created for
an asynchronous invocation. Pass invocation-specific values in the JSON
payload instead:

```json
{
  "environment": {
    "MESSAGE": "asynchronous-value"
  }
}
```

Send this payload to `/job/pass-envvars` using the service token:

```sh
curl -H "Authorization: Bearer <SERVICE_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"environment":{"MESSAGE":"asynchronous-value"}}' \
  https://<CLUSTER_ENDPOINT>/job/pass-envvars
```

The execution logs will report `Effective MESSAGE: asynchronous-value` and
`Selected source: invocation payload`.

Do not use either mechanism for secrets. Configure sensitive values as service
secrets instead.
