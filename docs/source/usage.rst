Usage
=====

Default Service Endpoints
-------------------------
Once the OSCAR framework is running on the Kubernetes cluster, the following endpoints should be available:

+-----------------+-----------------------+----------------------+ 
| Service         | Endpoint              | Default Credentials  | 
+=================+=======================+======================+ 
| OSCAR UI        | https://{KUBE}:31114  |     admin/admin      | 
+-----------------+-----------------------+----------------------+ 
| OSCAR Manager   | http://{KUBE}:32112   |                      |
+-----------------+-----------------------+----------------------+ 
| Minio UI        |  http://{KUBE}:31852  |    minio/minio123    | 
+-----------------+-----------------------+----------------------+ 
| OpenFaaS UI     | http://{KUBE}:31112   |                      | 
+-----------------+-----------------------+----------------------+ 
| Kubernetes API  | https://{KUBE}:6443   |                      | 
+-----------------+-----------------------+----------------------+ 

Note that `{KUBE}` refers to the public IP of the front-end of the Kubernetes cluster.

Getting Started
---------------

You can follow one of the `examples <https://github.com/grycap/oscar/tree/master/examples>`_ in order to use the OSCAR framework for specific applications. 