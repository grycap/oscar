# OSCAR Dashboard

> ❗️
>
> For simple OSCAR services you may use the Dashboard, but its features may not be on par with the latest changes in the [FDL](fdl.md). 
> Therefore, it is recommended to use [OSCAR CLI](oscar-cli.md) to deploy an OSCAR service.  


This section details the usage of the OSCAR Dashboard with the
[plant classification](https://github.com/grycap/oscar/tree/master/examples/plant-classification-sync) example, from the 
[OSCAR examples](https://github.com/grycap/oscar/tree/master/examples). 

## Login

OSCAR Dashboard is exposed via a Kubernetes ingress and it is accessible via the
Kubernetes master node IP. 

![login](images/usage/usage-01.png)

After a correct login, you should see the main view:
<!-- TODO actualizar foto del login a la nueva interfaz -->
![main view](images/usage/usage-02.png)

## Deploying services

To create a new service, you have two options: using OSCAR-Hub, a catalog of predefined services, or the manual way using an [FDL](fdl.md) definition or the creation form provided by the dashboard.

### OSCAR-Hub

In order to create a new service using OSCAR-Hub, you need to navigate to the corresponding menu in the dashboard. There you will see the catalog of predefined services that you can deploy and use out of the box. You can find all the definitions on its dedicated [GitHub repository](https://github.com/grycap/oscar-hub).

You also can contribute to it following our [contribution guide](https://hub.oscar.grycap.net/guide/) and view the [catalog](https://hub.oscar.grycap.net/) independently from the OSCAR.

![oscar hub](images/usage/oscar-hub-01.png)

Then you can view the description of the service and deploy it.

![oscar hub cowsay info](images/usage/oscar-hub-02.png)

Before deploying it, you can adjust the resources or other parameters depending on the service.

![oscar hub cowsay config](images/usage/oscar-hub-03.png)

For the invocation you can see the [synchronous invocation via dashboard](invoking-sync.md#synchronous-invocations-via-oscar-dashboard) example. It uses the same service.

## Manual deployment

To do the manual deployment, you need to click on "New" on the Service page, which is the first page you see after you log in.
Press the button and follow the wizard. For an [OSCAR Service](oscar-service.md) a script must be provided for the
processing of files. This script must use the environment variables
`INPUT_FILE_PATH` and `TMP_OUTPUT_DIR` to refer to the input file and the
folder where to save the results respectively:


```
#!/bin/bash

echo "SCRIPT: Invoked classify_image.py. File available in $INPUT_FILE_PATH"
FILE_NAME=`basename "$INPUT_FILE_PATH"`
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME"
python2 /opt/plant-classification-theano/classify_image.py \
 "$INPUT_FILE_PATH" -o "$OUTPUT_FILE"
```

You must fill in the fields indicating the container image to use, the name of
the service and the script file. In addition, you can add environment
variables, specify the resources (RAM and CPUs) and choose the log level of
the service.

Note that specifying a tag in the container image used can be convenient to
avoid problems with quotas for certain container registries such as
[Docker Hub](https://docs.docker.com/docker-hub/download-rate-limit/#what-is-the-download-rate-limit-on-docker-hub).
This is due to the fact that Kubernetes defaults the
[`imagePullPolicy`](https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy)
of pods to `Always` when no tag or the `latest` tag is set, which checks the
version of the image in the registry every time a job is launched.

![new service](images/usage/usage-03.png)

Next, we need to specify input and output storage parameters. You can utilize one or multiple storage providers supported by the platform, like MinIO, Onedata, and Amazon S3.

As we use the service creation form provided by the web interface to create the service, we will only have access to the default MinIO storage provided with the platform. In order to configure a more complicated workflow using multiple storage providers, you need to use an FDL file to define the service. 

In this step, you must first choose the paths of the storage provider to be
used as source of events, i.e. the input bucket and/or folder that will
trigger the service.

***Only the `minio.default` provider can be used as input storage provider through the Dashboard UI.*** For additional input providers (e.g., WebDAV/dCache or Rucio), use an FDL file with [OSCAR CLI](oscar-cli.md).

After filling in each path, remember to click on the "Save" button.

![add input 2](images/usage/usage-04.png)

Finally, the same must be done to indicate the output paths to be used in
the desired storage providers. You can also indicate suffixes and/or prefixes
to filter the files uploaded to each path by name.

![add output 1](images/usage/usage-05.png)

> ℹ️
>
> Note that the resulting files can be stored in several storage providers other than MinIO, but in order to do that, you must create the service through the FDL file.

![service created](images/usage/usage-06.png)

After clicking the "Create" button we will see the new service edit menu.

![service created](images/usage/usage-07.png)

Also, we will be able to see it on the main
view after a few seconds.

![service created](images/usage/usage-08.png)


## Triggering the service

### HTTP endpoints

OSCAR services can be invoked through auto-generated HTTP endpoints. Requests
to these endpoints can be made in two ways:

- **Synchronous** through the path `/run/<SERVICE_NAME>`. This delegates the
    request to the Knative service backing the OSCAR function.
- **Asynchronous** through the path `/job/<SERVICE_NAME>`. This mode is used
    to perform file-processing when files are uploaded to the input storage
    provider, creating a Kubernetes job per service invocation.

The content of the HTTP request body will be stored as a file that will be
available via the `INPUT_FILE_PATH` environment variable to process it.

A detailed specification of the OSCAR's API and its different paths can be
found [here](api.md).

## MinIO Storage Tab

MinIO Storage Tab is made to manage buckets without using MinIO UI. It
simplifies the process. From MinIO Storage Tab, buckets can be created or
removed and folders inside them. Furthermore, files can be uploaded to the
buckets and downloaded from them. Each time a service is created or submitted
an edit, the buckets that are not created will be formed.

### Uploading files

Once a service has been created, it can be invoked by uploading files to its
input bucket/folder. This can be done through the MinIO web interface
(accessible from the Kubernetes frontend IP, on port `30300`) or from the
"Buckets" section in the side menu of the OSCAR web interface. Selecting that section will list the buckets created and, by clicking on their name,
you will be able to see their content, upload and download files.

![minio storage](images/usage/usage-09.png)

To upload files, you can simply drag and drop them into the folder in the bucket or through the upload file button.

![upload input file](images/usage/usage-10.png)

The file will be uploaded, raising an event that will trigger the service.

![input file uploaded](images/usage/usage-11.png)

Note that the web interface includes a preview button for some file formats,
such as images.

![input file preview](images/usage/usage-12.png)

## Service status and logs

When files are being processed by a service, it is important to know their
status, as well as to observe the execution logs for testing. For this
purpose, OSCAR includes a log view, accessible by clicking on the "Logs"
button in a service from the main view.

![logs button](images/usage/usage-13.png)

In this view you can see all the jobs created for a service, as well as their
status ("Pending", "Running", "Succeeded" or "Failed") and their creation,
start and finish time.

![logs view 1](images/usage/usage-14.png)

To view the logs generated by a job, simply click on the drop-down button
located on the right.

![logs view 2](images/usage/usage-15.png)

The view also features options to delete one or all logs.

## Downloading files from MinIO

Downloading files from the platform's MinIO storage provider can also be done
using the OSCAR web interface. To do it, simply select one or more files and
click on the button "Download" or click on download icon to download one file.

![output file](images/usage/usage-16.png)

In the following picture you can see the preview of the resulting file after
the execution triggered in the previous step.

![output file preview](images/usage/usage-17.png)

## Deleting services

Services can be deleted by clicking on the trash can icon from the main view.

![deleting function](images/usage/usage-18.png)

Once you have accepted the message shown in the image above, the service will
be deleted after a few seconds.

![function deleted](images/usage/usage-19.png)
