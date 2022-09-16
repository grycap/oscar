# Functions Definition Language (OSCAR)

Example:

```
functions:
  oscar:
  - oscar-test:
      name: plants
      memory: 2Gi
      cpu: '1.0'
      image: grycap/oscar-theano-plants
      script: plants.sh
      input:
      - storage_provider: minio.default
        path: example-workflow/in
      output:
      - storage_provider: minio.default
        path: example-workflow/med
  - oscar-test:
      name: grayify
      memory: 1Gi
      cpu: '1.0'
      image: grycap/imagemagick
      script: grayify.sh
      input:
      - storage_provider: minio.default
        path: example-workflow/med
      output:
      - storage_provider: minio.default
        path: example-workflow/res
      - storage_provider: onedata.my_onedata
        path: result-example-workflow
      - storage_provider: webdav.dcache
        path: example-workflow/res

storage_providers:
  onedata:
    my_onedata:
      oneprovider_host: my_provider.com
      token: my_very_secret_token
      space: my_onedata_space
  webdav:
    dcache:
      hostname: my_dcache.com
      login: my_username
      password: my_password
```

## Top level parameters

| Field                                                             | Description                                                                                                                                                                                              |
| ----------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `functions` </br> *[Functions](#functions)*                       | Mandatory parameter to define a Functions Definition Language file. Note that "functions" instead of "services" has been used in order to keep compatibility with [SCAR](https://github.com/grycap/scar) |
| `storage_providers` </br> *[StorageProviders](#storageproviders)* | Parameter to define the credentials for the storage providers to be used in the services                                                                                                                 |
| `clusters` </br> *map[string][Cluster](#cluster)*                 | configuration for the OSCAR clusters that can be used as service's replicas, being the key the user-defined identifier for the cluster. Optional                                                         |

## Functions

| Field                                                | Description                                                                                                                                                                                                                                                            |
| ---------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `oscar` </br> *map[string][Service](#service) array* | Main object with the definition of the OSCAR services to be deployed. The components of the array are Service maps, where the key of every service is the identifier of the cluster where the service (defined as the value of the entry on the map) will be deployed. |

## Service

| Field                                                             | Description                                                                                                                                                                                                                                                  |
| ----------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `name` </br> *string*                                             | The name of the service                                                                                                                                                                                                                                      |
| `cluster_id` </br> *string*                                       | Identifier for the current cluster, used to specify the cluster's StorageProvider in job delegations. OSCAR-CLI sets it using the ClusterID from the FDL. Optional. (default: "")                                                                            |
| `image` </br> *string*                                            | Docker image for the service                                                                                                                                                                                                                                 |
| `alpine` </br> *boolean*                                          | Alpine parameter to set if image is based on Alpine. If `true` a custom release of faas-supervisor will be used. Optional (default: false)                                                                                                                   |
| `script` </br> *string*                                           | Local path to the user script to be executed in the service container                                                                                                                                                                                        |
| `image_pull_secrets` </br> *string array*                         | Array of Kubernetes secrets. Only needed to use private images located on private registries.                                                                                                                                                                |
| `memory` </br> *string*                                           | Memory limit for the service following the [kubernetes format](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-memory). Optional (default: 256Mi)                                                           |
| `cpu` </br> *string*                                              | CPU limit for the service following the [kubernetes format](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu). Optional (default: 0.2)                                                                   |
| `enable_gpu` </br> *bool*                                         | Parameter to enable the use of GPU for the service. Requires a device plugin deployed on the cluster (More info: [Kubernetes device plugins](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/#using-device-plugins)). Optional (default: false) |
| `total_memory` </br> *string*                                     | Limit for the memory used by all the service's jobs running simultaneously. Apache YuniKorn scheduler is required to work. Same format as Memory, but internally translated to MB (integer). Optional (default: "")                                          |
| `total_cpu` </br> *string*                                        | Limit for the virtual CPUs used by all the service's jobs running simultaneously. Apache YuniKorn scheduler is required to work. Same format as CPU, but internally translated to millicores (integer). Optional (default: "")                               |
| `synchronous` </br> *[SynchronousSettings](#synchronoussettings)* | Struct to configure specific sync parameters. This settings are only applied on Knative ServerlessBackend. Optional.                                                                                                                                         |
| `replicas` </br> *[Replica](#replica) array*                      | List of replicas to delegate jobs. Optional.                                                                                                                                                                                                                 |
| `rescheduler_threshold` </br> *string*                            | Time (in seconds) that a job (with replicas) can be queued before delegating it. Optional.                                                                                                                                                                   |
| `log_level` </br> *string*                                        | Log level for the FaaS Supervisor. Available levels: NOTSET, DEBUG, INFO, WARNING, ERROR and CRITICAL. Optional (default: INFO)                                                                                                                              |
| `input` </br> *[StorageIOConfig](#storageioconfig) array*         | Array with the input configuration for the service. Optional                                                                                                                                                                                                 |
| `output` </br> *[StorageIOConfig](#storageioconfig) array*        | Array with the output configuration for the service. Optional                                                                                                                                                                                                |
| `environment` </br> *[EnvVarsMap](#envvarsmap)*                   | The user-defined environment variables assigned to the service. Optional                                                                                                                                                                                     |
| `annotations` </br> *map[string]string*                           | User-defined Kubernetes [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/) to be set in job's definition. Optional                                                                                                |
| `labels` </br> *map[string]string*                                | User-defined Kubernetes [labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) to be set in job's definition. Optional                                                                                                          |

## SynchronousSettings

| Field                       | Description                                                                                  |
| --------------------------- | -------------------------------------------------------------------------------------------- |
| `min_scale` </br> *integer* | Minimum number of active replicas (pods) for the service. Optional. (default: 0)             |
| `max_scale` </br> *integer* | Maximum number of active replicas (pods) for the service. Optional. (default: 0 (Unlimited)) |

## Replica

| Field                               | Description                                                                                                                                                                                      |
| ----------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `type` </br> *string*               | Type of the replica to re-send events (can be `oscar` or `endpoint`)                                                                                                                             |
| `cluster_id` </br> *string*         | Identifier of the cluster as defined in the "clusters" FDL field. Only used if Type is `oscar`                                                                                                   |
| `service_name` </br> *string*       | Name of the service in the replica cluster. Only used if Type is `oscar`                                                                                                                         |
| `url` </br> *string*                | URL of the endpoint to re-send events (HTTP POST). Only used if Type is `endpoint`                                                                                                               |
| `ssl_verify` </br> *boolean*        | Parameter to enable or disable the verification of SSL certificates. Only used if Type is `endpoint`. Optional. (default: true)                                                                  |
| `priority` </br> *integer*          | Priority value to define delegation priority. Highest priority is defined as 0. If a delegation fails, OSCAR will try to delegate to another replica with lower priority. Optional. (default: 0) |
| `headers` </br> *map[string]string* | Headers to send in delegation requests. Optional                                                                                                                                                 |

## StorageIOConfig

| Field                             | Description                                                                                                                                                                                                                                      |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `storage_provider` </br> *string* | Reference to the storage provider defined in [storage_providers](#storage_providers). This string is composed by the provider's name (minio, s3, onedata) and identifier (defined by the user), separated by a point (e.g. "minio.myidentifier") |
| `path` </br> *string*             | Path in the storage provider. In MinIO and S3 the first directory of the specified path is translated into the bucket's name (e.g. "bucket/folder/subfolder")                                                                                    |
| `suffix` </br> *string array*     | Array of suffixes for filtering the files to be uploaded. Only used in the `output` field. Optional                                                                                                                                              |
| `prefix` </br> *string array*     | Array of prefixes for filtering the files to be uploaded. Only used in the `output` field. Optional                                                                                                                                              |

## EnvVarsMap

| Field                                 | Description                                                                             |
| ------------------------------------- | --------------------------------------------------------------------------------------- |
| `Variables` </br> *map[string]string* | Map to define the environment variables that will be available in the service container |

## StorageProviders

| Field                                                            | Description                                                                                                                                    |
| ---------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `minio` </br> *map[string][MinIOProvider](#minioprovider)*       | Map to define the credentials for a MinIO storage provider, being the key the user-defined identifier for the provider                         |
| `s3` </br> *map[string][S3Provider](#s3provider)*                | Map to define the credentials for a Amazon S3 storage provider, being the key the user-defined identifier for the provider                     |
| `onedata` </br> *map[string][OnedataProvider](#onedataprovider)* | Map to define the credentials for a Onedata storage provider, being the key the user-defined identifier for the provider                       |
| `webdav` </br> *map[string][WebDavProvider](#webdavprovider)*    | Map to define the credentials for a storage provider accesible via WebDav protocol, being the key the user-defined identifier for the provider |

## Cluster

| Field                          | Description                                                         |
| ------------------------------ | ------------------------------------------------------------------- |
| `endpoint` </br> *string*      | Endpoint of the OSCAR cluster API                                   |
| `auth_user` </br> *string*     | Username to connect to the cluster (basic auth)                     |
| `auth_password` </br> *string* | Password to connect to the cluster (basic auth)                     |
| `ssl_verify` </br> *boolean*   | Parameter to enable or disable the verification of SSL certificates |

## MinIOProvider

| Field                       | Description                                           |
| --------------------------- | ----------------------------------------------------- |
| `endpoint` </br> *string*   | MinIO endpoint                                        |
| `verify` </br> *bool*       | Verify MinIO's TLS certificates for HTTPS connections |
| `access_key` </br> *string* | Access key of the MinIO server                        |
| `secret_key` </br> *string* | Secret key of the MinIO server                        |
| `region` </br> *string*     | Region of the MinIO server                            |

## S3Provider

| Field                       | Description                      |
| --------------------------- | -------------------------------- |
| `access_key` </br> *string* | Access key of the AWS S3 service |
| `secret_key` </br> *string* | Secret key of the AWS S3 service |
| `region` </br> *string*     | Region of the AWS S3 service     |

## OnedataProvider

| Field                             | Description                 |
| --------------------------------- | --------------------------- |
| `oneprovider_host` </br> *string* | Endpoint of the Oneprovider |
| `token` </br> *string*            | Onedata access token        |
| `space` </br> *string*            | Name of the Onedata space   |

## WebDAVProvider

| Field                     | Description               |
| ------------------------- | ------------------------- |
| `hostname` </br> *string* | Provider hostname         |
| `login` </br> *string*    | Provider account username |
| `password` </br> *string* | Provider account password |
