# Default Service Endpoints

Once the OSCAR framework is running on the Kubernetes cluster, the endpoints
described in the following table should be available.
Most of the passwords/tokens are dynamically generated at deployment time and
made available in the `/var/tmp` folder of the front-end node of the cluster.

| Service         | Endpoint                | Default User |  Password File   |
|-----------------|-------------------------|--------------|------------------|
| OSCAR           | https://{KUBE}          |    oscar     |  oscar_password  |
| MinIO           | https://{KUBE}:30300    |    minio     | minio_secret_key |
| OpenFaaS        | http://{KUBE}:31112     |    admin     |  gw_password     |
| Kubernetes API  | https://{KUBE}:6443     |              |  tokenpass       |
| Kube. Dashboard | https://{KUBE}:30443    |              | dashboard_token  |

Note that `{KUBE}` refers to the public IP of the front-end of the Kubernetes cluster.