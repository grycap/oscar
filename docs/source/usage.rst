Usage
=====

Default Service Endpoints
-------------------------
Once the OSCAR framework is running on the Kubernetes cluster, the endpoints described in the following table should be available.
Most of the passwords/tokens are dynamically generated at deployment time and made available in the `/var/tmp` folder of the front-end node of the cluster.

+-----------------+-------------------------+--------------+------------------+
| Service         | Endpoint                | Default User |  Password File   |
+=================+=========================+==============+==================+
| OSCAR UI        | https://{KUBE}          | admin / admin|                  |
+-----------------+-------------------------+--------------+------------------+
| OSCAR Manager   | https://{KUBE}/oscar/   |              |                  |
| (Swagger UI)    | https://{KUBE}/oscar/ui/|              |                  |
+-----------------+-------------------------+--------------+------------------+
| Minio UI        | https://{KUBE}/minio/   |    minio     | minio_secret_key |
+-----------------+-------------------------+--------------+------------------+
| OpenFaaS UI     | https://{KUBE}/openfaas/|    admin     |  gw_password     |
+-----------------+-------------------------+--------------+------------------+
| Kubernetes API  | https://{KUBE}:6443     |              |  tokenpass       |
+-----------------+-------------------------+--------------+------------------+
| Kube. Dashboard | https://{KUBE}:30443    |              | dashboard_token  |
+-----------------+-------------------------+--------------+------------------+
| Prometheus      | http://{KUBE}:31119     |              |                  |
+-----------------+-------------------------+--------------+------------------+

Note that `{KUBE}` refers to the public IP of the front-end of the Kubernetes cluster. 

Getting Started
---------------

You can follow one of the `examples <https://github.com/grycap/oscar/tree/master/examples>`_ in order to use the OSCAR framework for specific applications. 
We recommend you start with the `Event-Driven Image Conversion via ImageMagick <https://github.com/grycap/oscar/tree/master/examples/imagemagick>`_ example.