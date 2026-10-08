# OSCAR CLI

OSCAR CLI provides a command-line interface for configuring OSCAR clusters, deploying services from [FDL](fdl.md) files, invoking services, managing storage, and inspecting deployments. See the [`oscar-cli` repository](https://github.com/grycap/oscar-cli) for the source.

## Download

Prebuilt binaries for supported platforms are available on the [releases page](https://github.com/grycap/oscar-cli/releases). With [Go](https://go.dev/doc/install) installed, you can instead run:

```sh
go install github.com/grycap/oscar-cli/v2@latest
```

## Authentication and configuration

Clusters are registered in `~/.oscar-cli/config.yaml` by default. Use `--config FILE` to select another YAML or JSON configuration file. Commands that target a cluster use the configured default unless you pass `-c, --cluster CLUSTER` (where supported).

```sh
oscar-cli cluster add IDENTIFIER ENDPOINT USERNAME --password-stdin
oscar-cli cluster add IDENTIFIER ENDPOINT --oidc-account-name SHORTNAME
oscar-cli cluster add IDENTIFIER ENDPOINT --oidc-refresh-token TOKEN
oscar-cli cluster default --set IDENTIFIER
```

The OIDC account-name form requires a configured [oidc-agent](https://indigo-dc.gitbook.io/oidc-agent/intro); `cluster add` also accepts `--disable-ssl` for clusters with untrusted certificates; leave certificate verification enabled when possible.

## Available commands

The commands and flags below correspond to the `oscar-cli`. Use `oscar-cli COMMAND --help` for full argument and alias details. `-h, --help` is available throughout; flags shown for a parent command are inherited by its children. Commands marked **changes state** create, update, invoke, or delete resources; inspect your target cluster before using them.

## Command index

Select a command to jump to its description and options.

- [FDL workflows](#fdl-workflows)
  - [`apply`](#cmd-apply)
  - [`delete`](#cmd-delete)
- [Cluster](#cluster)
  - [`cluster add`](#cmd-cluster-add)
  - [`cluster default`](#cmd-cluster-default)
  - [`cluster info`](#cmd-cluster-info)
  - [`cluster status`](#cmd-cluster-status)
  - [`cluster list`](#cmd-cluster-list)
  - [`cluster delete`](#cmd-cluster-delete)
- [OSCAR Hub](#oscar-hub)
  - [`hub list`](#cmd-hub-list)
  - [`hub deploy`](#cmd-hub-deploy)
  - [`hub validate`](#cmd-hub-validate)
- [Service](#service)
  - [`service get`](#cmd-service-get)
  - [`service list`](#cmd-service-list)
  - [`service delete`](#cmd-service-delete)
  - [`service run`](#cmd-service-run)
  - [`service job`](#cmd-service-job)
  - [`service get-file`](#cmd-service-get-file)
  - [`service put-file`](#cmd-service-put-file)
  - [`service list-files`](#cmd-service-list-files)
  - [`service delete-file`](#cmd-service-delete-file)
  - [`service system-logs`](#cmd-service-system-logs)
  - [`service deployment status`](#cmd-service-deployment-status)
  - [`service deployment logs`](#cmd-service-deployment-logs)
- [Service job logs](#service-job-logs)
  - [`service logs list`](#cmd-service-logs-list)
  - [`service logs get`](#cmd-service-logs-get)
  - [`service logs delete`](#cmd-service-logs-delete)
- [Bucket](#bucket)
  - [`bucket list`](#cmd-bucket-list)
  - [`bucket get`](#cmd-bucket-get)
  - [`bucket create`](#cmd-bucket-create)
  - [`bucket update`](#cmd-bucket-update)
  - [`bucket delete`](#cmd-bucket-delete)
  - [`bucket put-file`](#cmd-bucket-put-file)
  - [`bucket delete-file`](#cmd-bucket-delete-file)
  - [`bucket presign`](#cmd-bucket-presign)
- [Managed volumes](#managed-volumes)
  - [`volume list`](#cmd-volume-list)
  - [`volume get`](#cmd-volume-get)
  - [`volume create`](#cmd-volume-create)
  - [`volume delete`](#cmd-volume-delete)
- [User quotas](#user-quotas)
  - [`quota get`](#cmd-quota-get)
  - [`quota update`](#cmd-quota-update)
- [Metrics](#metrics)
  - [`metrics summary`](#cmd-metrics-summary)
  - [`metrics breakdown`](#cmd-metrics-breakdown)
  - [`metrics service`](#cmd-metrics-service)
- [Federation](#federation)
  - [`federation get`](#cmd-federation-get)
  - [`federation add-member`](#cmd-federation-add-member)
  - [`federation update`](#cmd-federation-update)
  - [`federation delete`](#cmd-federation-delete)
- [Other commands](#other-commands)
  - [`health`](#cmd-health)
  - [`interactive`](#cmd-interactive)
  - [`version`](#cmd-version)
  - [`completion`](#cmd-completion)
  - [`help`](#cmd-help)

### FDL workflows

| Command | Purpose and options |
| --- | --- |
| <a id="cmd-apply"></a>`apply FDL_FILE` | Create or update services (**changes state**). `-c, --cluster CLUSTER` overrides the FDL cluster, `-n, --name SERVICE_NAME` overrides the service and primary bucket names, `--env-file FILE` loads environment variables, and `--default` selects the configured default cluster instead of the FDL cluster. |
| <a id="cmd-delete"></a>`delete FDL_FILE` | Delete services defined in the FDL (**destructive**). `--default` selects the configured default cluster instead of the FDL cluster. |

```sh
oscar-cli apply service.yaml --cluster CLUSTER --name SERVICE_NAME
```

### Cluster

| Command | Purpose and options |
| --- | --- |
| <a id="cmd-cluster-add"></a>`cluster add IDENTIFIER ENDPOINT ...` | Register a cluster (**changes local configuration**). Authenticate with `USERNAME PASSWORD`, `USERNAME --password-stdin`, `-o, --oidc-account-name ACCOUNT`, or `-t, --oidc-refresh-token TOKEN`; optional `--disable-ssl`. |
| <a id="cmd-cluster-default"></a>`cluster default` | Show the default cluster or set it with `-s, --set IDENTIFIER` (**changes local configuration** when setting). |
| <a id="cmd-cluster-info"></a>`cluster info` | Show cluster information; `-c, --cluster CLUSTER`. |
| <a id="cmd-cluster-status"></a>`cluster status` | Show OSCAR Manager status and readiness; `-c, --cluster CLUSTER`, `-o, --output json` for structured output. |
| <a id="cmd-cluster-list"></a>`cluster list` | List configured clusters. |
| <a id="cmd-cluster-delete"></a>`cluster delete IDENTIFIER` | Remove a cluster from the **local CLI configuration**, not from OSCAR (**changes local configuration**); `remove` is an alias. |

```sh
oscar-cli cluster status --cluster CLUSTER --output json
```

### OSCAR Hub

Curated services are read from `grycap/oscar-hub` at reference `main`, under `crates`, unless overridden with `--owner`, `--repo`, `--ref`, or `--path`. `--local-path DIRECTORY` for `deploy` and `validate` selects a local directory containing the service crates.

| Command | Purpose and options |
| --- | --- |
| <a id="cmd-hub-list"></a>`hub list` | List curated services; `--json`, `--owner`, `--repo`, `--ref`, `--path`. |
| <a id="cmd-hub-deploy"></a>`hub deploy SERVICE-SLUG` | Deploy a service (**changes state**); `-c, --cluster CLUSTER`, `-n, --name SERVICE_NAME`, `--env-file FILE`, `--local-path DIRECTORY`, and source flags above. The name override also changes the primary bucket name. |
| <a id="cmd-hub-validate"></a>`hub validate SERVICE_SLUG` | Run RO-Crate acceptance tests (**may invoke services and write objects**); `-c, --cluster CLUSTER`, `-n, --name SERVICE_NAME`, `-H, --header HEADER`, `--local-path DIRECTORY`, and source flags above. `--print-acceptance-commands` prints commands **without running them**. |

```sh
oscar-cli hub list --json
oscar-cli hub deploy SERVICE-SLUG --cluster CLUSTER
oscar-cli hub validate SERVICE-SLUG --print-acceptance-commands
oscar-cli hub validate SERVICE-SLUG --cluster CLUSTER
```

### Service

`STORAGE_PROVIDER` has the form `minio.NAME`, `s3.NAME`, or `onedata.NAME`, matching the provider in the service definition. File operations can fail if an internal storage endpoint is not reachable from the CLI host; in local deployments, check the storage endpoint exposed to the host before using a storage client as a fallback.

Also note that the `REMOTE_FILE` must include the bucket name, since a service can have more than one bucket (e.g., `bucket-name/folder/new-file-name`).

| Command | Purpose and options |
| --- | --- |
| <a id="cmd-service-get"></a>`service get SERVICE_NAME` | Show a service definition; `-c, --cluster CLUSTER`. Treat the output as potentially sensitive. |
| <a id="cmd-service-list"></a>`service list` | List services; `-c, --cluster CLUSTER`. |
| <a id="cmd-service-delete"></a>`service delete SERVICE_NAME...` | Delete one or more services (**destructive**); `-c, --cluster CLUSTER`. `remove` is an alias. |
| <a id="cmd-service-run"></a>`service run SERVICE_NAME` | Synchronous invocation (**changes state**; requires a serverless backend). `-f, --file-input FILE` or `-i, --text-input TEXT`; `-o, --output FILE`, `--decode-output`, `-H, --header HEADER`, `-e, --endpoint URL`, `-t, --token TOKEN`, `-c, --cluster CLUSTER`. |
| <a id="cmd-service-job"></a>`service job SERVICE_NAME` | Asynchronous invocation (**changes state**; MinIO provider required). `-f, --file-input FILE` or `-i, --text-input TEXT`; `-e, --endpoint URL`, `-t, --token TOKEN`, `-c, --cluster CLUSTER`. |
| <a id="cmd-service-get-file"></a>`service get-file SERVICE_NAME [STORAGE_PROVIDER] [REMOTE_PATH] [LOCAL_FILE]` | Download from a service output provider; by default uses the first output provider. `--download-latest-into [DESTINATION]` downloads the newest file (the remote path can then be omitted); `--no-progress`, `-c, --cluster CLUSTER`. |
| <a id="cmd-service-put-file"></a>`service put-file SERVICE_NAME [STORAGE_PROVIDER] LOCAL_FILE [REMOTE_FILE]` | Upload to a service input path (**changes state**). Defaults to `minio.default`; when `REMOTE_FILE` is omitted, uses the configured input path and local filename. `--no-progress`, `-c, --cluster CLUSTER`. |
| <a id="cmd-service-list-files"></a>`service list-files SERVICE_NAME STORAGE_PROVIDER REMOTE_PATH` | List files; `-c, --cluster CLUSTER`. |
| <a id="cmd-service-delete-file"></a>`service delete-file SERVICE_NAME STORAGE_PROVIDER REMOTE_FILE` | Delete a stored file (**destructive**); `-c, --cluster CLUSTER`. |
| <a id="cmd-service-system-logs"></a>`service system-logs` | Read OSCAR Manager logs (Basic Auth only); `-o, --output text\|json\|csv`, `-p, --previous`, `-t, --timestamps`, `-c, --cluster CLUSTER`. |
| <a id="cmd-service-deployment-status"></a>`service deployment status SERVICE_NAME` | Show deployment status; `-o, --output yaml\|json\|table`, `-c, --cluster CLUSTER`. |
| <a id="cmd-service-deployment-logs"></a>`service deployment logs SERVICE_NAME` | Show deployment logs; `-o, --output yaml\|json\|table\|csv`, `-c, --cluster CLUSTER`. |

```sh
oscar-cli service run SERVICE_NAME --cluster CLUSTER --text-input 'hello'
oscar-cli service job SERVICE_NAME --cluster CLUSTER --text-input 'hello'
oscar-cli service deployment status SERVICE_NAME --cluster CLUSTER
```

#### Service job logs

| Command | Purpose and options |
| --- | --- |
| <a id="cmd-service-logs-list"></a>`service logs list SERVICE_NAME` | List jobs/logs; `-s, --status STATUS` filters by Pending, Running, Succeeded, or Failed (comma-separated values supported). |
| <a id="cmd-service-logs-get"></a>`service logs get SERVICE_NAME [JOB_NAME]` | Show job logs; `-l, --latest` selects the newest job, `-t, --show-timestamps` includes timestamps. |
| <a id="cmd-service-logs-delete"></a>`service logs delete SERVICE_NAME {JOB_NAME... \| --succeeded \| --all}` | Delete jobs and logs (**destructive**); `-s, --succeeded` or `-a, --all`. `remove` is an alias. |

These log commands inherit `-c, --cluster CLUSTER` from `service logs`. For asynchronous file inputs, upload the file to the service's input path before `service job`; then inspect logs and the output bucket.

### Bucket

Bucket commands inherit `-c, --cluster CLUSTER`. `bucket get` **lists objects**; it does not download their contents. Use service file operations to retrieve service output files.

| Command | Purpose and options |
| --- | --- |
| <a id="cmd-bucket-list"></a>`bucket list` | List visible buckets; `-o, --output table\|json`. No bucket name argument. |
| <a id="cmd-bucket-get"></a>`bucket get BUCKET_NAME` | List objects in a bucket; `--prefix PREFIX`, `--limit NUMBER`, `--page TOKEN`, `--all` to fetch all pages, `-o, --output table\|json`. |
| <a id="cmd-bucket-create"></a>`bucket create BUCKET_NAME` | Create a bucket (**changes state**); `--visibility public\|private`, `--allowed-users USER1,USER2`. |
| <a id="cmd-bucket-update"></a>`bucket update BUCKET_NAME` | Update bucket access (**changes state**); `--visibility public\|private`, `--allowed-users USER1,USER2`. |
| <a id="cmd-bucket-delete"></a>`bucket delete BUCKET_NAME` | Delete a bucket (**destructive**). |
| <a id="cmd-bucket-put-file"></a>`bucket put-file BUCKET_NAME LOCAL_FILE REMOTE_PATH` | Upload a file (**changes state**); `--no-progress`. |
| <a id="cmd-bucket-delete-file"></a>`bucket delete-file BUCKET_NAME REMOTE_PATH` | Delete an object (**destructive**). |
| <a id="cmd-bucket-presign"></a>`bucket presign BUCKET_NAME FILE_NAME` | Generate a presigned URL; `-X, --operation get\|put\|head\|delete`, `--expires SECONDS`, `--content-type TYPE`, `--extra-headers KEY=VALUE,...`. The URL may grant object access: handle it as a secret. |

```sh
oscar-cli bucket list --cluster CLUSTER
oscar-cli bucket get BUCKET_NAME --cluster CLUSTER --prefix output/ --all --output json
```

### Managed volumes

Volume commands inherit `-c, --cluster CLUSTER`.

| Command | Purpose and options |
| --- | --- |
| <a id="cmd-volume-list"></a>`volume list` | List managed volumes; `-o, --output table\|json`. |
| <a id="cmd-volume-get"></a>`volume get VOLUME_NAME` | Show volume details; `-o, --output yaml\|json`. |
| <a id="cmd-volume-create"></a>`volume create VOLUME_NAME` | Create a managed volume (**changes state**); `--size SIZE` (for example `1Gi`). |
| <a id="cmd-volume-delete"></a>`volume delete VOLUME_NAME` | Delete a managed volume (**destructive**). |

### User quotas

Quota commands inherit `-c, --cluster CLUSTER`. Updating quotas requires appropriate administrative authorization.

| Command | Purpose and options |
| --- | --- |
| <a id="cmd-quota-get"></a>`quota get [USER_ID]` | Show the quota of the current user or an explicitly identified user. |
| <a id="cmd-quota-update"></a>`quota update USER_ID` | Update quota (**changes state**); `--cpu QUANTITY`, `--memory QUANTITY`, `--volume-count COUNT`, `--volume-disk SIZE`, `--volume-max-disk SIZE`, `--volume-min-disk SIZE`. |

### Metrics

Metrics commands inherit `-c, --cluster CLUSTER` and accept `--start RFC3339` and `--end RFC3339` time filters.

| Command | Purpose and options |
| --- | --- |
| <a id="cmd-metrics-summary"></a>`metrics summary` | Show cluster-wide totals. |
| <a id="cmd-metrics-breakdown"></a>`metrics breakdown` | Group cluster metrics; `--group-by service\|user\|country`. |
| <a id="cmd-metrics-service"></a>`metrics service SERVICE_NAME` | Show metrics for a service. |

### Federation

Federation commands inherit `-c, --cluster CLUSTER`. For `--type oscar`, specify `--cluster-id` and `--service-name`; for `--type endpoint`, use `--url`. `--priority` controls delegation priority (0 is highest).

| Command | Purpose and options |
| --- | --- |
| <a id="cmd-federation-get"></a>`federation get SERVICE_NAME` | List federation members; `-o, --output text\|json\|table`. |
| <a id="cmd-federation-add-member"></a>`federation add-member SERVICE_NAME` | Add a member (**changes state**); `--type oscar\|endpoint`, `--cluster-id ID`, `--service-name NAME`, `--url URL`, `--priority NUMBER`. |
| <a id="cmd-federation-update"></a>`federation update SERVICE_NAME` | Update a member (**changes state**); `--type`, `--cluster-id`, `--service-name`, `--url`, `--priority`. |
| <a id="cmd-federation-delete"></a>`federation delete SERVICE_NAME MEMBER_NAME` | Delete a member (**destructive**); `--cluster-id ID`, `--service-name NAME`. |

### Other commands

| Command | Purpose and options |
| --- | --- |
| <a id="cmd-health"></a>`health` | Check the health endpoint; `-c, --cluster CLUSTER`, `-o, --output json` for structured output. |
| <a id="cmd-interactive"></a>`interactive` | Launch the terminal UI for browsing and managing cluster resources. |
| <a id="cmd-version"></a>`version` | Print the CLI version. |
| <a id="cmd-completion"></a>`completion` | Generate shell completion; run `oscar-cli completion --help` for supported shells. |
| <a id="cmd-help"></a>`help [command]` | Show help for any command or subcommand. |
