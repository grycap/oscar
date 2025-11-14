# OSCAR Service

OSCAR allows the creation of serverless file-processing services based on
container images. These services require a user-defined script with the
commands responsible of the processing. The platform automatically mounts a
volume on the containers with the
[FaaS Supervisor](https://github.com/grycap/faas-supervisor) component, which
is in charge of:

- Downloading the file that invokes the service and make it accessible through
    the `INPUT_FILE_PATH` environment variable.
- Execute the user-defined script.
- Upload the content of the output folder accessible via the `TMP_OUTPUT_DIR`
    environment variable.



### FaaS Supervisor

[FaaS Supervisor](https://github.com/grycap/faas-supervisor), the component in
charge of managing the input and output of services, allows JSON or base64
encoded body in service requests. The body of these requests will be
automatically decoded into the invocation's input file available from the
script through the `$INPUT_FILE_PATH` environment variable.

The output of synchronous invocations will depend on the application itself:

1. If the script generates a file inside the output dir available through the
    `$TMP_OUTPUT_DIR` environment variable, the result will be the file encoded in
    base64.
1. If the script generates more than one file inside `$TMP_OUTPUT_DIR`, the
    result will be a zip archive containing all files encoded in base64.
1. If there are no files in `$TMP_OUTPUT_DIR`, FaaS Supervisor will return its
    logs, including the stdout of the user script run.
    **To avoid FaaS Supervisor's logs, you must set the service's `log_level`
    to `CRITICAL`.**

This way users can adapt OSCAR's services to their own needs.

The FaaS Supervisor supports the following storage back-ends:

  * [MinIO](https://min.io)
  * [Amazon S3](https://aws.amazon.com/s3/)
  * Webdav (and, therefore, [dCache](https://dcache.org))
  * Onedata (and, therefore, [EGI DataHub](https://www.egi.eu/service/datahub/))

### Container images

Container images on asynchronous services use the tag `imagePullPolicy: Always`, which means that Kubernetes will check for the image digest on the image registry and download it if it is not present.
So, if you are using an image without a specific tag or with the latest tag, the service will automatically download and use the most recent version of the image on its executions, whenever the image is updated.

You can follow one of the
[examples](https://github.com/grycap/oscar/tree/master/examples)
in order to test the OSCAR framework for specific applications. We recommend
you to start with the
[plant classification example](https://github.com/grycap/oscar/tree/master/examples/plant-classification-sync).


### External Storage Providers

[FaaS Supervisor](https://github.com/grycap/faas-supervisor), download the input file and upload the output file. There are multiple Storage available to interact with. Each [Storage](/fdl/#storageproviders) has parameters, an authentication process, and particularities.

#### MinIO

OSCAR installs a MinIO inside the cluster. This default MinIO can be referenced as `minio` or `minio.default` in the input and output parameters of the [OSCAR service](/fdl/#service). Suppose you define a new MinIO cluster in the [storage provider](/fdl/#minioprovider). You can use a MinIO outside the cluster. MinIO Storage works as input or output without any other third-party software. Also, MinIO can make a [bucket replication](/minio-bucket-replication/) between two buckets.


#### dCache

[dCache](https://dcache.org/) use [WebDAV protocol](/fdl/#webdavprovider). dCache works as an output storage without any other third-party software. However, [DCNiOS](https://intertwin-eu.github.io/dcnios/) is necessary for input storage.

The EGI check-in authentication process is not available.


#### S3

You have to pass the AWS credentials to the FaaS Supervisor by defining them in the environment variables with the keys `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_DEFAULT_REGION`. S3 works as an output storage system without any third-party software. But [DCNiOS](https://intertwin-eu.github.io/dcnios/) is necessary as input storage.


#### ONEDATA

[ONEDATA](https://onedata.org) is a global data access solution for science that only works as an output storage provider.
Check the [ONEDATA Storage provider](/fdl/#onedataprovider) for more information.

![Storage](/images/storage.png)
