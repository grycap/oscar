# Integration with SCONE

SCONE is a tool that allows confidential computing on the cloud thus protecting the data, code and application secrets on a Kubernetes cluster (More info about SCONE and Kubernetes [here](https://sconedocs.github.io/k8s_concepts/)). 

To use SCONE on a Kubernetes cluster Intel SGX has to be enabled on the machines, and for these, the SGX Kubernetes plugin needs to be present on the cluster. Once the plugin is installed you only need to specify the parameter `enable_sgx` on the FDL of the services that are going to use a secured container image like in the following example.

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