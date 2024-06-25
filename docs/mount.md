# Mount

This feature mounts a folder (in MinIO, called a bucket) of a storage service inside the OSCAR service. This mount feature can only be used in the OSCAR service execution type of asynchronous and exposed service. So, the invocation of an OSCAR service will see the content of the storage services under the folder `/mnt/`. OSCAR service can read and write inside the folder. In case the mounted folder is an input for another OSCAR service. Writing a file inside these mounted folders will trigger the second OSCAR service.

If the storage provider is not the MinIO default `minio.default`, the credentials provider must be defined in [FDL](/fdl). These are the providers available for the mount feature:

 - [MinIO provider](/fdl/#minioprovider)
 - [WebDav provider](/fdl/#webdavprovider)

Let's explore these with an FDL example:

```
mount:
  storage_provider: minio.default
  path: /body-pose-detection-async
```

The example above means that OSCAR mounts the `body-pose-detection-async` bucket of the default MinIO inside the OSCAR services. So, the content of the `body-pose-detection-async` bucket will be found in `/mnt/body-pose-detection-async` folder inside the execution of OSCAR services.
