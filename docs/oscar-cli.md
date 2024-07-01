# OSCAR CLI

OSCAR CLI provides a command line interface to interact with
OSCAR. It supports cluster registrations,
service management, workflows definition from [FDL](fdl.md) files and the ability to manage files from OSCAR's compatible
storage providers (MinIO, AWS S3 and Onedata). The folder
[`example-workflow`](https://github.com/grycap/oscar-cli/tree/main/example-workflow)
contains all the necessary files to create a simple workflow to test the tool.


## Download

### Releases

The easy way to download OSCAR-CLI is through the GitHub
[releases page](https://github.com/grycap/oscar-cli/releases). There are
binaries for multiple platforms and OS. If you need a binary for another
platform, please open an [issue](https://github.com/grycap/oscar-cli/issues).

### Install from source

If you have the [Go](https://golang.org/doc/install) programming language installed and
[configured](https://github.com/golang/go/wiki/SettingGOPATH), you can get it directly
from the source by executing:

```sh
go install github.com/grycap/oscar-cli@latest
```

### OIDC (OpenID Connect)

If your cluster has OIDC avaliable, follow these steps to use `oscar-cli` to interact with it using the OpenID Connect.

- Install [oidc-agent](https://indigo-dc.gitbook.io/oidc-agent/intro)
- Register the [EGI client](https://indigo-dc.gitbook.io/oidc-agent/user/oidc-gen/provider/egi)
- Add a cluster in `oscar-cli` with oidc credentians (More info about the usage of the `cluster add` command [here](#add))

``` bash
oscar-cli cluster add IDENTIFIER ENDPOINT --oidc-account-name SHORTNAME
```


## Available commands


- [OSCAR CLI](#oscar-cli)
  - [Download](#download)
    - [Releases](#releases)
    - [Install from source](#install-from-source)
    - [OIDC (OpenID Connect)](#oidc-openid-connect)
  - [Available commands](#available-commands)
    - [apply](#apply)
    - [cluster](#cluster)
      - [Subcommands](#subcommands)
        - [add](#add)
        - [default](#default)
        - [info](#info)
        - [list](#list)
        - [remove](#remove)
    - [service](#service)
      - [Subcommands of services](#subcommands-of-services)
        - [get](#get)
        - [list services](#list-services)
        - [remove services](#remove-services)
        - [run](#run)
        - [logs list](#logs-list)
        - [logs get](#logs-get)
        - [logs remove](#logs-remove)
        - [get-file](#get-file)
        - [put-file](#put-file)
        - [list-files](#list-files)
    - [version](#version)
    - [help](#help)

### apply

Apply a FDL file to create or edit services in clusters.

```
Usage:
  oscar-cli apply FDL_FILE [flags]

Aliases:
  apply, a

Flags:
      --config string   set the location of the config file (YAML or JSON)
  -h, --help            help for apply
```

### cluster

Manages the configuration of clusters.

#### Subcommands

##### add

Add a new existing cluster to oscar-cli.

```
Usage:
  oscar-cli cluster add IDENTIFIER ENDPOINT {USERNAME {PASSWORD | \
  --password-stdin} | --oidc-account-name ACCOUNT} [flags]

Aliases:
  add, a

Flags:
      --disable-ssl               disable verification of ssl certificates for the
                                  added cluster
  -h, --help                      help for add
  -o, --oidc-account-name string  OIDC account name to authenticate using
                                  oidc-agent. Note that oidc-agent must be
                                  started and properly configured
                                  (See:https://indigo-dc.gitbook.io/oidc-agent/)
      --password-stdin            take the password from stdin

Global Flags:
      --config string   set the location of the config file (YAML or JSON)
```

##### default

Show or set the default cluster.

```
Usage:
  oscar-cli cluster default [flags]

Aliases:
  default, d

Flags:
  -h, --help         help for default
  -s, --set string   set a default cluster by passing its IDENTIFIER

Global Flags:
      --config string   set the location of the config file (YAML or JSON)
```

##### info

Show information of an OSCAR cluster.

```
Usage:
  oscar-cli cluster info [flags]

Aliases:
  info, i

Flags:
  -c, --cluster string   set the cluster
  -h, --help             help for info

Global Flags:
      --config string   set the location of the config file (YAML or JSON)
```

##### list

List the configured OSCAR clusters.

```
Usage:
  oscar-cli cluster list [flags]

Aliases:
  list, ls

Flags:
  -h, --help   help for list

Global Flags:
      --config string   set the location of the config file (YAML or JSON)
```

##### remove

Remove a cluster from the configuration file.

```
Usage:
  oscar-cli cluster remove IDENTIFIER [flags]

Aliases:
  remove, rm

Flags:
  -h, --help   help for remove

Global Flags:
      --config string   set the location of the config file (YAML or JSON)
```

### service

Manages the services within a cluster.

#### Subcommands of services

##### get

Get the definition of a service.

```
Usage:
  oscar-cli service get SERVICE_NAME [flags]

Aliases:
  get, g

Flags:
  -c, --cluster string   set the cluster
  -h, --help             help for get

Global Flags:
      --config string   set the location of the config file (YAML or JSON)
```

##### list services

List the available services in a cluster.

```
Usage:
  oscar-cli service list [flags]

Aliases:
  list, ls

Flags:
  -c, --cluster string   set the cluster
  -h, --help             help for list

Global Flags:
      --config string   set the location of the config file (YAML or JSON)
```

##### remove services

Remove a service from the cluster.

```
Usage:
  oscar-cli service remove SERVICE_NAME... [flags]

Aliases:
  remove, rm

Flags:
  -c, --cluster string   set the cluster
  -h, --help             help for remove

Global Flags:
      --config string   set the location of the config file (YAML or JSON)
```

##### run

Invoke a service synchronously (a Serverless backend in the cluster is required).

```
Usage:
  oscar-cli service run SERVICE_NAME {--input | --text-input} [flags]

Aliases:
  run, invoke, r

Flags:
  -c, --cluster string      set the cluster
  -h, --help                help for run
  -i, --input string        input file for the request
  -o, --output string       file path to store the output
  -t, --text-input string   text input string for the request

Global Flags:
      --config string   set the location of the config file (YAML or JSON)
```

##### logs list

List the logs from a service.

```
Usage:
  oscar-cli service logs list SERVICE_NAME [flags]

Aliases:
  list, ls

Flags:
  -h, --help             help for list
  -s, --status strings   filter by status (Pending, Running, Succeeded or
                         Failed), multiple values can be specified by a
                         comma-separated string

Global Flags:
  -c, --cluster string   set the cluster
      --config string    set the location of the config file (YAML or JSON)
```

##### logs get

Get the logs from a service's job.

```
Usage:
  oscar-cli service logs get SERVICE_NAME JOB_NAME [flags]

Aliases:
  get, g

Flags:
  -h, --help              help for get
  -t, --show-timestamps   show timestamps in the logs

Global Flags:
  -c, --cluster string   set the cluster
      --config string    set the location of the config file (YAML or JSON)
```

##### logs remove

Remove a service's job along with its logs.

```
Usage:
  oscar-cli service logs remove SERVICE_NAME \
   {JOB_NAME... | --succeeded | --all} [flags]

Aliases:
  remove, rm

Flags:
  -a, --all         remove all logs from the service
  -h, --help        help for remove
  -s, --succeeded   remove succeeded logs from the service

Global Flags:
  -c, --cluster string   set the cluster
      --config string    set the location of the config file (YAML or JSON)
```
> **Note**
> The following subcommands will not work with MinIO if you use a local deployment due to DNS resolutions, so if you want to use a command line put/get/list files from your buckets, you can use the [MinIO client](https://min.io/docs/minio/linux/reference/minio-mc.html) command line. <br>
> Once you have the client installed you can define the cluster with the `mc alias` command like it follows:
> ```
> mc alias set myminio https://localhost:30000 minioadminuser minioadminpassword
> ```
> So, instead of the next subcommands, you would use:
> - [`mc cp`](https://min.io/docs/minio/linux/reference/minio-mc/mc-cp.html) to put/get files fron a bucket. <br>
> - [`mc ls`](https://min.io/docs/minio/linux/reference/minio-mc/mc-ls.html) to list files from a bucket.

##### get-file

Get a file from a service's storage provider.

The STORAGE_PROVIDER argument follows the format
STORAGE_PROVIDER_TYPE.STORAGE_PROVIDER_NAME, being the STORAGE_PROVIDER_TYPE
one of the three supported storage providers (MinIO, S3 or Onedata) and the
STORAGE_PROVIDER_NAME is the identifier for the provider set in the service's
definition.

```
Usage:
  oscar-cli service get-file SERVICE_NAME STORAGE_PROVIDER REMOTE_FILE \
   LOCAL_FILE [flags]

Aliases:
  get-file, gf

Flags:
  -c, --cluster string   set the cluster
  -h, --help             help for get-file

Global Flags:
      --config string   set the location of the config file (YAML or JSON)
```

##### put-file

Put a file in a service's storage provider.

The STORAGE_PROVIDER argument follows the format
STORAGE_PROVIDER_TYPE.STORAGE_PROVIDER_NAME, being the STORAGE_PROVIDER_TYPE
one of the three supported storage providers (MinIO, S3 or Onedata) and the
STORAGE_PROVIDER_NAME is the identifier for the provider set in the service's
definition.

**_NOTE:_** This command can not be used in a [local testing deployment](https://docs.oscar.grycap.net/local-testing/).

```
Usage:
  oscar-cli service put-file SERVICE_NAME STORAGE_PROVIDER LOCAL_FILE \
   REMOTE_FILE [flags]

Aliases:
  put-file, pf

Flags:
  -c, --cluster string   set the cluster
  -h, --help             help for put-file

Global Flags:
      --config string   set the location of the config file (YAML or JSON)
```

##### list-files

List files from a service's storage provider path.

The STORAGE_PROVIDER argument follows the format STORAGE_PROVIDER_TYPE.STORAGE_PROVIDER_NAME,
being the STORAGE_PROVIDER_TYPE one of the three supported storage providers
(MinIO, S3 or Onedata) and the STORAGE_PROVIDER_NAME is the identifier for the
provider set in the service's definition.

```
Usage:
  oscar-cli service list-files SERVICE_NAME STORAGE_PROVIDER REMOTE_PATH [flags]

Aliases:
  list-files, list-file, lsf

Flags:
  -c, --cluster string   set the cluster
  -h, --help             help for list-files

Global Flags:
      --config string   set the location of the config file (YAML or JSON)
```

### version

Print the version.

```
Usage:
  oscar-cli version [flags]

Aliases:
  version, v

Flags:
  -h, --help   help for version
```

### help

Help provides help for any command in the application.
Simply type oscar-cli help [path to command] for full details.

```
Usage:
  oscar-cli help [command] [flags]

Flags:
  -h, --help   help for help
```
