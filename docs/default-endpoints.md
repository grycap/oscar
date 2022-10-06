# Default Service Endpoints

## In IM deploy

OSCAR user interfaces can be accessed with the cluster endpoint, which only uses user/password credentials. Alternatively, by https://ui.oscar.grycap.net, it can use user/password credentials or EGI Check-in authentication. Both processes must introduce the endpoint host, which can be passed in the URL together with the username. Once the OSCAR framework is running on the Kubernetes cluster, the endpoints that IM provides described in the following table should be available.

| Service                   | Endpoint                                                                        | Default User |  Password File   |
|---------------------------|---------------------------------------------------------------------------------|--------------|------------------|
| OSCAR                     | https://{public_host}                                                           |    oscar     |  oscar_password  |
| OSCAR with EGI check-in   | https://ui.oscar.grycap.net?endpoint=https://{public_host}&username={user_name} |    oscar     |  oscar_password  |
| MinIO                     | https://minio.{public_host}/                                                    |    minio     | minio_secret_key |
| MinIO console             | https://console.{public_host}/                                                  |    minio     | minio_secret_key |
| Dashboard endpoint        | https://{public_host}/dashboard/                                                |              |                  |

## Localhost deploy

The oscar-ui website can also access to localhost endpoint.OSCAR in a local installation has other endpoints:

| Service         | Endpoint                   | Default User |  Password File   |
|-----------------|----------------------------|--------------|------------------|
| OSCAR           | https://localhost          |    oscar     |  oscar_password  |
| MinIO           | https://localhost:30300    |    minio     | minio_secret_key |
| MinIO console   | https://localhost:30301    |    minio     | minio_secret_key |
| OpenFaaS        | http://localhost:31112     |    admin     |  gw_password     |
| Kubernetes API  | https://localhost:6443     |              |  tokenpass       |
| Kube. Dashboard | https://localhost:30443    |              | dashboard_token  |

## OSCAR-CLI

The oscar-cli tool can only access by user/password credentials. EGI Check-In is not allowed in OSCAR-CLI. So configure the localhost access with the URL https://localhost:443 and set the variable ssl_verify to false. If OSCAR is deployed with IM, set the URL https://{public_host}