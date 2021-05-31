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

storage_providers:
  onedata:
    my_onedata:
      oneprovider_host: my_provider.com
      token: my_very_secret_token
      space: my_onedata_space
```

## Top level parameters

| Field                                                             | Description                                                                                                                                                                                              |
|-------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `functions` </br> *[Functions](#functions)*                       | Mandatory parameter to define a Functions Definition Language file. Note that "functions" instead of "services" has been used in order to keep compatibility with [SCAR](https://github.com/grycap/scar) |
| `storage_providers` </br> *[StorageProviders](#storageproviders)* | Parameter to define the credentials for the storage providers to be used in the services                                                                                                                 |

## Functions

| Field                                | Description                                                                                                                                                                                                                                                           |
|---------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `oscar` </br> *map[string][Service](#service) array* | Main object with the definition of the OSCAR services to be deployed. The components of the array are Service maps, where the key of every service is the identifier of the cluster where the service (defined as the value of the entry on the map) will be deployed. |

## Service

| Field                                                      | Description                                                                                                                                                                                        |
|------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `name` </br> *string*                                      | The name of the service                                                                                                                                                                            |
| `image` </br> *string*                                     | Docker image for the service                                                                                                                                                                       |
| `script` </br> *string*                                    | Local path to the user script to be executed in the service container                                                                                                                              |
| `memory` </br> *string*                                    | Memory limit for the service following the [kubernetes format](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-memory). Optional (default: 256Mi) |
| `cpu` </br> *string*                                       | CPU limit for the service following the [kubernetes format](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu). Optional (default: 0.2)         |
| `log_level` </br> *string*                                 | Log level for the FaaS Supervisor. Available levels: NOTSET, DEBUG, INFO, WARNING, ERROR and CRITICAL. Optional (default: INFO)                                                                    |
| `input` </br> *[StorageIOConfig](#storageioconfig) array*  | Array with the input configuration for the service. Optional                                                                                                                                       |
| `output` </br> *[StorageIOConfig](#storageioconfig) array* | Array with the output configuration for the service. Optional                                                                                                                                      |
| `environment` </br> *[EnvVarsMap](#envvarsmap)*            | The user-defined environment variables assigned to the service. Optional                                                                                                                           |

## StorageIOConfig

| Field                             | Description                                                                                                                                                                                                                                      |
|-----------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `storage_provider` </br> *string* | Reference to the storage provider defined in [storage_providers](#storage_providers). This string is composed by the provider's name (minio, s3, onedata) and identifier (defined by the user), separated by a point (e.g. "minio.myidentifier") |
| `path` </br> *string*             | Path in the storage provider. In MinIO and S3 the first directory of the specified path is translated into the bucket's name (e.g. "bucket/folder/subfolder")                                                                                    |
| `suffix` </br> *string array*     | Array of suffixes for filtering the files to be uploaded. Only used in the `output` field. Optional                                                                                                                                              |
| `prefix` </br> *string array*     | Array of prefixes for filtering the files to be uploaded. Only used in the `output` field. Optional                                                                                                                                              |

## EnvVarsMap

| Field                                 | Description                                                                             |
|---------------------------------------|-----------------------------------------------------------------------------------------|
| `Variables` </br> *map[string]string* | Map to define the environment variables that will be available in the service container |

## StorageProviders

| Field                                                            | Description                                                                                                                |
|------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------|
| `minio` </br> *map[string][MinIOProvider](#minioprovider)*       | Map to define the credentials for a MinIO storage provider, being the key the user-defined identifier for the provider     |
| `s3` </br> *map[string][S3Provider](#s3provider)*                | Map to define the credentials for a Amazon S3 storage provider, being the key the user-defined identifier for the provider |
| `onedata` </br> *map[string][OnedataProvider](#onedataprovider)* | Map to define the credentials for a Onedata storage provider, being the key the user-defined identifier for the provider   |

## MinIOProvider

| Field                       | Description                                           |
|-----------------------------|-------------------------------------------------------|
| `endpoint` </br> *string*   | MinIO endpoint                                        |
| `verify` </br> *bool*       | Verify MinIO's TLS certificates for HTTPS connections |
| `access_key` </br> *string* | Access key of the MinIO server                        |
| `secret_key` </br> *string* | Secret key of the MinIO server                        |
| `region` </br> *string*     | Region of the MinIO server                            |

## S3Provider

| Field                       | Description                      |
|-----------------------------|----------------------------------|
| `access_key` </br> *string* | Access key of the AWS S3 service |
| `secret_key` </br> *string* | Secret key of the AWS S3 service |
| `region` </br> *string*     | Region of the AWS S3 service     |

## OnedataProvider

| Field                             | Description                 |
|-----------------------------------|-----------------------------|
| `oneprovider_host` </br> *string* | Endpoint of the Oneprovider |
| `token` </br> *string*            | Onedata access token        |
| `space` </br> *string*            | Name of the Onedata space   |
