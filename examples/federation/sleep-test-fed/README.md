# Federated sleep test

This example deploys `sleep-test-fed` in an OSCAR cluster and
`sleep-test-fed-replica` in a second cluster. Each invocation requests one CPU
and runs for 90 seconds. Submitting several files concurrently can exhaust the
coordinator quota and trigger job delegation to the member cluster.

The example contains:

- `sleep-test-fed.yaml`: federated service definition.
- `script.sh`: script executed by the coordinator and its replica.

## Requirements

- Two OSCAR clusters accessible through `oscar-cli`.
- Each OSCAR Manager must be able to resolve and reach the other cluster's
  endpoint.
- Both clusters must accept tokens issued by the same OIDC provider.
- An `oscar-cli` profile for each cluster.
- A valid OIDC refresh token in an environment file:

  ```dotenv
  refresh_token=YOUR_OIDC_REFRESH_TOKEN
  ```


## Deployment with regular DNS names

Production or externally accessible OSCAR clusters do not need any special
hostname handling. Configure the member endpoint in `sleep-test-fed.yaml` with
a DNS name that the coordinator OSCAR Manager can resolve and reach:

```yaml
clusters:
  local-replica:
    endpoint: https://oscar-member.example.org
    auth_user: ''
    auth_password: ''
    ssl_verify: true
```

The member `cluster_id` must match its key under `clusters`:

```yaml
federation:
  topology: star
  delegation: static
  members:
  - type: oscar
    cluster_id: local-replica
    service_name: sleep-test-fed-replica
    priority: 0
```

The `local` key under `functions.oscar` identifies the deployment target in
the FDL. The `-c` option overrides that key with the selected `oscar-cli`
profile. The CLI adds the profile endpoint to the deployed service definition,
and the member uses it to send delegated results back to the coordinator. The
profile endpoint must therefore be reachable both from the machine running
`oscar-cli` and from the member OSCAR Manager.

The TLS certificate presented by each cluster must match its configured DNS
name when `ssl_verify` is enabled.

## Local kind setup with Docker Desktop

Local kind clusters do not have public DNS names. The test can still exercise
the normal DNS-based federation path by adding explicit hostnames to their
Gateway API `HTTPRoute` resources. These aliases belong to the local cluster
configuration; OSCAR does not rewrite the endpoint or the HTTP `Host` header.

The commands below use the following example environment:

- Coordinator context: `kind-oscar-test-zgf`.
- Member context: `kind-oscar-test-nmn`.
- HTTPS NodePort: `30443` in both clusters.
- Coordinator CLI profile: `localhost-oidc-grycap`.
- Member CLI profile: `localhost-replica-oidc-grycap`.

Adjust the names and ports for your environment.

### 1. Find the kind node addresses

```sh
COORDINATOR_CONTEXT=kind-oscar-test-zgf
MEMBER_CONTEXT=kind-oscar-test-nmn
COORDINATOR_NODE=oscar-test-zgf-control-plane
MEMBER_NODE=oscar-test-nmn-control-plane

COORDINATOR_IP=$(docker inspect \
  -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' \
  "$COORDINATOR_NODE")
MEMBER_IP=$(docker inspect \
  -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' \
  "$MEMBER_NODE")

echo "Coordinator: $COORDINATOR_IP"
echo "Member:      $MEMBER_IP"
```

`nip.io` can provide a DNS name that resolves to the member's internal node
address:

```sh
MEMBER_INTERNAL_DNS="oscar-member.${MEMBER_IP}.nip.io"
```

On Docker Desktop, the host normally cannot connect directly to the internal
Docker subnet. The coordinator endpoint therefore uses
`host.docker.internal`, which is reachable from both macOS and the pods. The
member can use its `nip.io` name because requests to it originate from the
coordinator Manager.

### 2. Add the local HTTPRoute aliases

Record the current hostnames first if you want to restore them after the test:

```sh
kubectl --context "$COORDINATOR_CONTEXT" get httproute oscar -n oscar \
  -o jsonpath='{.spec.hostnames}'
kubectl --context "$MEMBER_CONTEXT" get httproute oscar -n oscar \
  -o jsonpath='{.spec.hostnames}'
```

Replace the hostname lists with the values used by the test. Preserve any
additional hostname required by your local installation:

```sh
kubectl --context "$COORDINATOR_CONTEXT" patch httproute oscar -n oscar \
  --type merge \
  -p '{"spec":{"hostnames":["localhost.direct","host.docker.internal"]}}'

kubectl --context "$MEMBER_CONTEXT" patch httproute oscar -n oscar \
  --type merge \
  -p "{\"spec\":{\"hostnames\":[\"localhost.direct\",\"localhost\",\"${MEMBER_INTERNAL_DNS}\"]}}"
```

Verify that the member DNS name resolves from the coordinator Manager:

```sh
kubectl --context "$COORDINATOR_CONTEXT" exec -n oscar deploy/oscar -- \
  getent hosts "$MEMBER_INTERNAL_DNS"
```

### 3. Prepare a temporary OSCAR CLI configuration

A local coordinator profile commonly points to `https://localhost.direct`.
Using that value directly would store it as the origin endpoint. From the
member cluster, `localhost.direct` would resolve to the member itself instead
of the coordinator.

Create a protected temporary copy of the CLI configuration:

```sh
CLI_CONFIG=/tmp/oscar-cli-federation.yaml
cp "$HOME/.oscar-cli/config.yaml" "$CLI_CONFIG"
chmod 600 "$CLI_CONFIG"
```

Edit `$CLI_CONFIG` and change only the endpoint of the coordinator profile,
keeping its existing OIDC configuration:

```yaml
oscar:
  localhost-oidc-grycap:
    endpoint: https://host.docker.internal
    ssl_verify: false
    # Keep the remaining existing profile settings here.
```

Do not print or copy the tokens from this file into the repository.

### 4. Prepare a temporary FDL

Keep the committed example generic and make local endpoint changes in a
temporary copy.

Edit `sleep-test-fed.yaml` and replace the member endpoint and TLS
setting with the local values:

```yaml
clusters:
  local-replica:
    endpoint: https://oscar-member.MEMBER_IP.nip.io:30443
    auth_user: ''
    auth_password: ''
    ssl_verify: false
```

Replace `MEMBER_IP` with the address obtained above. Disabling TLS
verification is only appropriate for these local clusters with development
certificates. The coordinator endpoint is not duplicated in the FDL: with
`-c localhost-oidc-grycap`, it comes from the temporary CLI profile configured
in the previous step.

## Deploy the federation

Running `oscar-cli` inside the `oscar` repository allows it to resolve the relative
`script.sh` path from the FDL:

```sh
(
  oscar-cli apply "examples/federation/sleep-test-fed/sleep-test-fed.yaml" \
    --env-file "$HOME/.oscar-cli/oscar.env" \
    -c localhost-oidc-grycap
)
```

Verify that both services exist:

```sh
oscar-cli service list  -c localhost-oidc-grycap
oscar-cli service list  \
  -c localhost-replica-oidc-grycap
```

The replica must inherit the coordinator CPU and memory and preserve the same
input and output paths.

## Generate load and verify delegation

Upload several files quickly. With a two-CPU quota and one-CPU jobs, some of
the workload should be delegated:

```sh
for number in 1 2 3 4; do
  oscar-cli bucket put-file sleep-test-fed \
    /dev/null "input/test-${number}.txt" \
    --no-progress \
    -c localhost-oidc-grycap &
done
wait
```

Inspect jobs in both clusters:

```sh
kubectl --context "$COORDINATOR_CONTEXT" get jobs -A \
  -l oscar_service=sleep-test-fed

kubectl --context "$MEMBER_CONTEXT" get jobs -A \
  -l oscar_service=sleep-test-fed-replica
```

Delegation can also be confirmed in the coordinator Manager logs:

```sh
kubectl --context "$COORDINATOR_CONTEXT" logs -n oscar deploy/oscar \
  --since=10m | grep -E 'RESOURCE-DELEGATION|successfully delegated'
```

After approximately 90 seconds, local and delegated outputs should appear in
the same coordinator bucket:

```sh
oscar-cli bucket get sleep-test-fed --prefix output --all \
  --config "$CLI_CONFIG" -c localhost-oidc-grycap
```

Each output contains `Running on <hostname>`, which distinguishes coordinator
pods from member pods.

## Test incremental member deployment

Remove the member from the coordinator definition:

```sh
oscar-cli federation delete sleep-test-fed \
  --cluster-id local-replica \
  --service-name sleep-test-fed-replica \
  --config "$CLI_CONFIG" -c localhost-oidc-grycap
```

If the remote service still exists, delete it explicitly:

```sh
oscar-cli service delete sleep-test-fed-replica \
  --config "$CLI_CONFIG" -c localhost-replica-oidc-grycap
```

Add the member again:

```sh
oscar-cli federation add-member sleep-test-fed \
  --cluster-id local-replica \
  --service-name sleep-test-fed-replica \
  --config "$CLI_CONFIG" -c localhost-oidc-grycap
```

The coordinator first attempts `PUT /system/services`. If the service does not
exist and the member returns `404`, it automatically creates it with
`POST /system/services`.

Verify the result:

```sh
oscar-cli federation get sleep-test-fed \
  --config "$CLI_CONFIG" -c localhost-oidc-grycap
oscar-cli service list --config "$CLI_CONFIG" \
  -c localhost-replica-oidc-grycap
```

## Cleanup

Delete only the services created by this example:

```sh
oscar-cli service delete sleep-test-fed-replica \
  --config "$CLI_CONFIG" -c localhost-replica-oidc-grycap
oscar-cli service delete sleep-test-fed \
  --config "$CLI_CONFIG" -c localhost-oidc-grycap
```

Finally, restore the original `HTTPRoute` hostnames if the local aliases are no
longer needed. OSCAR clusters with correctly configured DNS names do not need
this local setup.
