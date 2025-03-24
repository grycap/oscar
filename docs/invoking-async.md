# Asynchronous invocations

For event-driven file processing, OSCAR automatically manages the creation
and [notification system](https://docs.min.io/minio/baremetal/monitoring/bucket-notifications/bucket-notifications.html#minio-bucket-notifications)
of MinIO buckets in order to allow the event-driven invocation of services
using asynchronous requests, generating a Kubernetes job for every file to be
processed.


![oscar-async.png](images/oscar-async.png)


 Note that if your OSCAR service runs an AI model for inference, each job will load the AI model weights before performing the inference. You can mitigate this penalty by adjusting the inference code to process a compressed file with several images.

If you want to process a large number of data files, consider using [OSCAR Batch](https://github.com/grycap/oscar-batch), a tool designed to perform batch-based processing in OSCAR clusters. It includes a coordinator tool where the user provides a MinIO bucket containing files for processing. This service calculates the optimal number of parallel service invocations that can be accommodated within the cluster, according to its current status, and distributes the image processing workload accordingly among the service invocations. This is mainly intended to process large amounts of files, for example, historical data.
