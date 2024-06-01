# Introduction

![OSCAR-logo](images/oscar3.png)

OSCAR is an open-source platform to support the event-driven serverless computing model for data-processing applications. It can be automatically deployed on multi-Clouds, and even on low-powered devices, to create highly-parallel event-driven data-processing serverless applications along the computing continuum. These applications execute on customized runtime environments provided by Docker containers that run on elastic Kubernetes clusters. It is also integrated with the 
[SCAR framework](https://github.com/grycap/scar), which supports a
[High Throughput Computing Programming Model](https://scar.readthedocs.io/en/latest/prog_model.html)
to create highly-parallel event-driven data-processing serverless applications
that execute on customized runtime environments provided by Docker containers
run on AWS Lambda and AWS Batch.


## Concepts
- **OSCAR Cluster**: A Kubernetes cluster (either fixed size or elastic) configured with the OSCAR services and components. The cluster must have at least one Front-End (FE) node, which executes the OSCAR control plane and, optionally, several Working Nodes (WNs), which execute the OSCAR services and replicated services from the control plane for enhanced fault-tolerance.
- **OSCAR Service**: The execution unit in the OSCAR framework, typically defined in [FDL](fdl.md), by a:
    - Docker image, providing the customized runtime environment for an application.
    - Execution requirements.
    - User-defined shell script that will be executed in a dynamically-provisioned container.
    - (Optional) The object storage that will trigger the execution of the OSCAR service upon a file upload. 
    - (Optional) The object storage(s) on which the output results of the OSCAR service will be stored. 
    - (Optional) Deployment strategy and additional configuration. 


## Rationale

Users create OSCAR services to:

  - Execute a containerized command-line application or web service in response to:
      - a file upload to an object store (e.g. [MinIO](http://min.io)), thus supporting loosely-coupled High-Throughput Computing use cases where many files need to be processed in parallel in a distributed computing platform.
      - a request to a load-balanced auto-scaled HTTP-based endpoints, thus allowing to exposed generic scientific applications as highly-scalable HTTP endpoints.
  - Execute a pipeline of multiple OSCAR service where the output data of one triggers the execution of another OSCAR service, potentially running in different clusters, thus creating event-driven scalable pipelines along the computing continuum.

An admin user can deploy an OSCAR cluster on a Cloud platform so that other users belonging to a Virtual Organization (VO) can create OSCAR services. A VO is a group of people (e.g. scientists, researchers) with common interests and requirements, who need to work collaboratively and/or share resources (e.g. data, software, expertise, CPU, storage space) regardless of geographical location. OSCAR supports the VOs defined in [EGI](https://egi.eu), which are listed in the ['Operations Portal'](https://operations-portal.egi.eu/vo/a/list). EGI is the European's largest federation of computing and storage resource providers united by a mission of delivering advanced computing and data analytics services for research and innovation.


## Architecture & Components

![oscar arch](images/oscar-arch.png)

OSCAR runs on an elastic [Kubernetes](http://kubernetes.io) cluster that is
deployed using:

- [IM](http://www.grycap.upv.es/im), an open-source virtual infrastructure
    provisioning tool for multi-Clouds.

The following components are deployed inside the Kubernetes cluster in order
to support the OSCAR platform:

- [CLUES](http://github.com/grycap/clues), an elasticity manager that
    horizontally scales in and out the number of nodes of the Kubernetes
    cluster according to the workload.
- [MinIO](https://min.io), a high-performance distributed object storage
    server that provides an API compatible with S3.
- [Knative](https://knative.dev), a serverless framework to serve
    container-based applications for synchronous invocations (default
    Serverless Backend).
- [OSCAR Manager](https://docs.oscar.grycap.net/api/), the main API, responsible for the management of the services and the integration of the different components. 
- [OSCAR UI](https://github.com/grycap/oscar-ui), an easy-to-use web-based graphical user interface aimed at end users.

As external storage providers, the following services can be used:

- External [MinIO](https://min.io) servers, which may be in clusters other
    than the platform.
- Amazon [S3](https://aws.amazon.com/s3/), AWS's object storage
    service that offers industry-leading scalability, data availability,
    security, and performance in the public Cloud.
- [Onedata](https://onedata.org/), the global data access solution for science
    used in the [EGI Federated Cloud](https://datahub.egi.eu/).
- Any storage provider that can be accessible through
    [WebDAV](http://www.webdav.org/) protocol. An example of a storage provider
    supporting this protocol is [dCache](https://dcache.org/), a storage
    middleware system capable of managing the storage and exchange of large data
    quantities.

***Note**: All of the mentioned storage providers can be used as output, but
only MinIO can be used as input.*


An OSCAR cluster can be easily deployed via the [IM Dashboard](http://im.egi.eu)
on any major public and on-premises Cloud provider, including the EGI Federated Cloud.

An OSCAR cluster can be accessed via its
[REST API](https://grycap.github.io/oscar/api/), the web-based 
[OSCAR UI](https://github.com/grycap/oscar-ui) and the command-line interface provided by
[OSCAR CLI](https://github.com/grycap/oscar-cli).
