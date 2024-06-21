# Integration with SCONE

SCONE is a tool that allows confidential computing on the cloud thus protecting the data, code and application secrets on a Kubernetes cluster.  By leveraging hardware-based security features such as [Intel SGX (Software Guard Extensions)](https://www.intel.com/content/www/us/en/products/docs/accelerator-engines/software-guard-extensions.html), SCONE ensures that sensitive data and computations remain protected even in potentially untrusted environments. This end-to-end encryption secures data both at rest and in transit, significantly reducing the risk of data breaches. Additionally, SCONE simplifies the development and deployment of secure applications by providing a seamless integration layer for existing software, thus enhancing security without requiring major code changes. 

> ⚠️
>
> Please note that the usage of SCONE introduces a non-negligible overhead when executing the container for the OSCAR service.


More info about SCONE and Kubernetes [here](https://sconedocs.github.io/k8s_concepts/). 

To use SCONE on a Kubernetes cluster, Intel SGX has to be enabled on the machines, and for these, the SGX Kubernetes plugin needs to be present on the cluster. Once the plugin is installed you only need to specify the parameter `enable_sgx` on the FDL of the services that are going to use a secured container image like in the following example.

``` yaml
functions:
  oscar:
  - oscar-cluster:
      name: sgx-service
      memory: 1Gi
      cpu: '0.6'
      image: your_image
      enable_sgx: true
      script: script.sh
```

SCONE support was introduced in OSCAR for the [AI-SPRINT](https://ai-sprint-project.eu) project to tackle the [Personalized Healthcare use case](https://ai-sprint-project.eu/use-cases/personalised-healthcare) in which OSCAR is employed to perform the inference phase of pre-trained models out of sensitive data captured from wearable devices. This use case was coordinated by the [Barcelona Supercomputing Center (BSC)](https://www.bsc.es) and [Technische Universität Dresden — TU Dresden](https://tu-dresden.de) was involved for the technical activities regarding SCONE.