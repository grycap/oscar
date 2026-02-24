# Deployment with Helm

OSCAR can also be deployed on any existing Kubernetes cluster through its
[helm chart](https://github.com/grycap/helm-charts/tree/master/oscar).
However, to make the platform work properly, the following dependencies must
be satisfied.

- A StorageClass with the `ReadWriteMany` access mode must be configured in
    the cluster for the creation of the persistent volume mounted on the service
    containers. For this purpose, we use the
    [Kubernetes NFS-Client Provisioner](https://github.com/kubernetes-sigs/nfs-subdir-external-provisioner),
    but there are other
    [volume plugins](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#access-modes)
    that support this access mode.
- MinIO must be deployed and properly configured in the cluster. Its
    [helm chart](https://github.com/minio/helm) can be used
    for this purpose. It is important to configure it properly to have access from
    inside and outside the cluster, as the OSCAR's web interface connects directly
    to its API. In the OSCAR helm chart, you must indicate the
    [values](https://github.com/grycap/helm-charts/tree/master/oscar#configuration)
    corresponding to its credentials and endpoint.
