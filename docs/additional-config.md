# Additional configuration

To configure trusted image origins, OSCAR reads the `allowed_image_repositories` JSON array from an additional ConfigMap. Each entry must be a fully qualified registry and repository path (for example, `ghcr.io/grycap`). OSCAR compares the registry and complete repository path components, not substrings: `ghcr.io/grycap` permits `ghcr.io/grycap/imagemagick` (including tags and digests) but **not** `ghcr.io/grycap-other/image` or `other.example/ghcr.io/grycap/image`. A more specific entry such as `ghcr.io/grycap/imagemagick` permits that image and its subpaths, but not `ghcr.io/grycap/imagemagick-other`.

If the ConfigMap is absent or the array is empty (`[]`), images are unrestricted. If the ConfigMap exists but its policy cannot be read or parsed, service creation and updates fail instead of silently disabling the restriction. Malformed entries in an existing ConfigMap are skipped with a warning, so a single invalid entry does not block services that match other valid entries. If the policy does not contain any valid entry, service creation is not blocked and the restriction is effectively disabled (a warning is logged).

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config.yaml
  namespace: oscar
data:
  allowed_image_repositories: '["ghcr.io/grycap"]'
```

The administrator can also update the policy using `PUT /system/config` with Basic authentication. The endpoint validates that every entry is a fully qualified registry and repository path and rejects the request with a `400` listing the malformed entries (for example, `test` or `test/test` fail because strict validation requires the registry to be explicitly defined). This example allows images from the `ghcr.io/grycap` owner only:

```sh
curl -X PUT -u '<admin-user>:<admin-password>' \
  -H 'Content-Type: application/json' \
  -d '{"allowed_image_repositories":["ghcr.io/grycap"]}' \
  'https://<oscar_endpoint>/system/config'
```

Additionally, this property can be added when creating an OSCAR cluster through the IM, which will automatically create the ConfigMap.

![allowed-prefixes](images/im-dashboard/im-additional-config.png)

Managed volumes do not require extra entries in this additional configuration file. Volume provisioning uses the cluster storage integration already configured for OSCAR.

MinIO bucket quotas also do not require entries in this additional
configuration file. Per-user MinIO quota settings are managed through
`/system/quotas/user/{userId}` and stored by OSCAR in each user's namespace.

## Federation endpoints

To explicitly trust federation destinations, OSCAR reads the
`allowed_federation_endpoints` JSON array from the same additional ConfigMap.
Every entry is a host, a host and a port, or a bracketed IPv6 literal:

- `peer.example.io` — matches any port on that host (comparison is
  case-insensitive);
- `peer.example.io:8443` — matches only that exact port;
- `[2001:db8::1]` or `[2001:db8::1]:8080` — IPv6 literals use the bracket
  form.

Endpoints must also satisfy a set of built-in rules that cannot be disabled:

- only the `http` and `https` schemes are allowed (for example
  `file://`, `ftp://` or `gopher://` are rejected with an explicit error),
  the endpoint must have a host, and it must not contain embedded
  credentials;
- when an endpoint is not in the allowlist, OSCAR resolves the host and
  rejects it when any of the resolved addresses falls inside a loopback,
  link-local (including the cloud metadata address 169.254.169.254) or
  private/unique-local range;
- endpoints listed in the allowlist are trusted: the IP range check is
  skipped for them. This is how operators opt in to internal federation
  peers.

The safe default is fail-closed: when the ConfigMap is absent or the array is
empty, nothing is explicitly trusted, only publicly resolvable endpoints pass
the built-in checks, and caller credentials are never forwarded to any
destination.

Federation requests are also bounded defensively:

- a 30 second request timeout (10 seconds for the credential-verification
  check performed before federated creations, 5 seconds for the DNS
  resolution done while validating an endpoint);
- at most 5 redirects, every redirect `Location` is revalidated with the same
  endpoint rules and the request is aborted when it is not allowed;
- restricted credential forwarding: the caller `Authorization` header is only
  forwarded to allowlisted destinations, every other destination receives
  only the cluster's own Basic authentication credentials (or none when the
  cluster has no credentials configured).

The API boundary rejects unsafe endpoints with a `400` that names the
offending endpoint (service creation and `PUT /federation` validate only the
newly added or changed endpoints, so an empty or partial update body
keeps working). Admins can update the list with `PUT /system/config` using
Basic authentication; invalid entries are rejected with a `400`. The list is
parsed when OSCAR starts, so a `PUT /system/config` update applies to a
running manager after the next restart.

IPv6 endpoints must be written with brackets; an IPv6 literal endpoint that
is not allowlisted fails the DNS resolution step and is rejected.
