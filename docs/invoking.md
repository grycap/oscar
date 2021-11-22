# Invoking services

OSCAR services can be invoked synchronously and asynchronously sending an HTTP POST request to paths `/run/<SERVICE_NAME>` and `/job/<SERVICE_NAME>` respectively. For file processing, OSCAR automatically manages the creation and [notification system](https://docs.min.io/minio/baremetal/monitoring/bucket-notifications/bucket-notifications.html#minio-bucket-notifications) of MinIO buckets in order to allow the event-driven invocation of services using asynchronous requests, generating a Kubernetes job for every file to be processed.

Furthermore, synchronous invocations ...TBC

## Service access tokens

As detailed in the [API specification](api.md), invocation paths require the service access token in the request header for authentication. Service access tokens are auto-generated in service creation and update, and MinIO eventing system is automatically configured to use them for event-driven file processing. Tokens can be obtained through the API, using the [`oscar-cli service get`](https://github.com/grycap/oscar-cli#get) command or directly from the web interface.

![oscar-ui-service-token.png](images/usage/oscar-ui-service-token.png)

TBC