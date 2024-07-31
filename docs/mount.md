# Mounting external storage on service volumes

This feature enables the mounting of a folder from a storage provider, such as MinIO or dCache, into the service container. As illustrated in the following diagram, the folder is placed inside the /mnt directory on the container volume, thereby making it accessible to the service. This functionality can be utilized with exposed services, such as those using a Jupyter Notebook, to make the content of the storage bucket accessible directly within the Notebook.

![mount-diagram](images/mount.png)

As OSCAR has the credentials of the default MinIO instance internally, if you want to use a different one or a different storage provider, you need to set these credentials on the service [FDL](/fdl). Currently, the storage providers supported on this functionality are:

 - [MinIO provider](/fdl/#minioprovider)
 - [WebDav provider](/fdl/#webdavprovider)

Let's explore these with an FDL example:

```
mount:
  storage_provider: minio.default
  path: /body-pose-detection-async
```

The example above means that OSCAR mounts the `body-pose-detection-async` bucket of the default MinIO inside the OSCAR services. So, the content of the `body-pose-detection-async` bucket will be found in `/mnt/body-pose-detection-async` folder inside the execution of OSCAR services.
