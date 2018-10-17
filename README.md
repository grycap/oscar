# OSCAR - On-Premises Serverless Container-aware ARchitectures

[![License](https://img.shields.io/badge/license-Apache%202-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)


## Introduction

OSCAR is a framework to efficiently support on-premises FaaS (Functions as a Service) for general-purpose file-processing computing applications. It represents the porting of the [SCAR framework](https://github.com/grycap/scar), which supports the execution of generic applications based on Docker images in AWS Lambda, into an on-premises scenario.

### Goal
Users upload files to a bucket and this automatically triggers the execution of parallel invocations to a function responsible for processing each file. Output files are delivered into an output bucket for the convenience of the user. Highly scalable HTTP-based endpoints can also be offered in order to expose a generic application. The deployment of the computing infrastructure and its scalability is abstracted away from the user.

### How?

It deploys a Kubernetes cluster and several other servicies in order to support a FaaS-based file-processing execution model:

  * [CLUES](http://github.com/grycap/clues), an elasticity manager that horizontally scales in and out the number of nodes of the Kubernetes cluster according to the workload.
  * [Minio](http://minio.io), a high performance distributed object storage server that provides an API compatible with S3. 
  * [OpenFaaS](https://www.openfaas.com/), a FaaS platform that allows creating functions executed via HTTP requests.
  * [Event Gateway](https://serverless.com/event-gateway/), an event router that facilitates wiring functions to HTTP endpoints.
  * [OSCAR UI](https://github.com/grycap/oscar-ui), a web-based GUI aimed at end users to facilitate interaction with OSCAR.

## Documentation

OSCAR is under heavy development. Its documentation is available in [readthedocs](http://o-scar.readthedocs.io/en/latest/).

## Licensing

OSCAR is licensed under the Apache License, Version 2.0. See
[LICENSE](https://github.com/grycap/scar/blob/master/LICENSE) for the full
license text.

<a id="acknowledgements"></a>
## Acknowledgements

This development is partially funded by the [EGI Strategic and Innovation Fund](https://www.egi.eu/about/egi-council/egi-strategic-and-innovation-fund/).
