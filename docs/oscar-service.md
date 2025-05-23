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
