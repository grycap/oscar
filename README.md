<h1 align="center">
    <img src="docs/source/images/oscar-logo.png" alt="OSCAR" width="320">
  <br>
  OSCAR - Open Source Serverless Computing for Data-Processing Applications
</h1>

## Introduction

OSCAR is an open-source platform to support the Functions as a Service (FaaS) computing model for file-processing applications. It can be automatically deployed on multi-Clouds in order to create highly-parallel event-driven file-processing serverless applications that execute on customized runtime environments provided by Docker containers than run on an elastic Kubernetes cluster.

[**Deploy**](docs/source/deploy.rst) &nbsp; |
&nbsp; [**Documentation**](https://o-scar.readthedocs.io) &nbsp;

<BR><center><img src="docs/source/images/oscar-components.png" alt="OSCAR Components" width="700"></center>

## Overview

- [**About OSCAR**](#why-oscar)
- [**Components**](#components)
- [**Licensing**](#licensing)
- [**Acknowledgements**](#acknowledgements)

### Why OSCAR?
FaaS platforms are typically oriented to the execution of short-lived functions, coded in a certain programming language, in response to events. Scientific application can greatly benefit from this event-driven computing paradigm in order to trigger on demand the execution of a resource-intensive application that requires processing a certain file that was just uploaded to a storage service. This requires additional support for the execution of generic applications in existing open-source FaaS frameworks.

To this aim, OSCAR supports the [High Throughput Computing Programming Model](https://scar.readthedocs.io/en/latest/prog_model.html) initially introduced by the [SCAR framework](https://github.com/grycap/scar), to create highly-parallel event-driven file-processing serverless applications that execute on customized runtime environments provided by Docker containers run on AWS Lambda.

With OSCAR, users upload files to a data storage back-end and this automatically triggers the execution of parallel invocations to a function responsible for processing each file. Output files are delivered into a data storage back-end for the convenience of the user. The user only specifies the Docker image and the script to be executed, inside a container created out of that image, in order to process a file that will be automatically made available to the container. The deployment of the computing infrastructure and its scalability is abstracted away from the user.

### Components

OSCAR runs on an elastic Kubernetes cluster that is deployed using:

* [EC3](http://www.grycap.upv.es/ec3), an open-source tool to deploy compute clusters that can horizontally scale in terms of number of nodes with multiple plugins.
* [IM](http://www.grycap.upv.es/im), an open-source virtual infrastructure provisioning tool for multi-Clouds.
* [CLUES](http://github.com/grycap/clues), an elasticity manager that horizontally scales in and out the number of nodes of the Kubernetes cluster according to the workload.

The following services are deployed inside the Kubernetes cluster in order to support the OSCAR platform:

* [Minio](http://minio.io), a high performance distributed object storage server that provides an API compatible with S3. 
* [OpenFaaS](https://www.openfaas.com/), a FaaS platform that allows creating functions executed via HTTP requests.
* [Event Gateway](https://serverless.com/event-gateway/), an event router that facilitates wiring functions to HTTP endpoints.
* [OSCAR UI](https://github.com/grycap/oscar-ui), a web-based GUI aimed at end users to facilitate interaction with the OSCAR platform.

Further information is available in the [documentation](https://o-scar.readthedocs.io).

## Licensing

OSCAR is licensed under the Apache License, Version 2.0. See
[LICENSE](https://github.com/grycap/scar/blob/master/LICENSE) for the full
license text.

## Acknowledgements

This development is partially funded by the [EGI Strategic and Innovation Fund](https://www.egi.eu/about/egi-council/egi-strategic-and-innovation-fund/).