# Frequently Asked Questions (FAQ)

## Troubleshooting

- **Sometimes, when trying to deploy the cluster locally, it tells **me that the :80 port** is already in use.**

You may have a server running on the :80 port, such as Apache, while the deployment is trying to use it for the OSCAR UI. Restarting it would solve this problem.

- **I get the following error message: "Unable to communicate with the cluster. Please make sure that the endpoint is well-typed and accessible."**

When using oscar-cli, you can get this error if you try to run a service that is not present on the cluster set as default.
You can check if you are using the correct default cluster with the following command,

`oscar-cli cluster default`

and set a new default cluster with the following command:

`oscar-cli cluster default -s CLUSTER_ID`

- **How Do I use a secret image?**

In case it is required, the use of secret images. It should create a [secret with the docker login configuration](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#registry-secret-existing-credentials) with a structure like this:

```
apiVersion: v1
kind: Secret
metadata:
  name: dockersecret
  namespace: oscar-svc
data:
  .dockerconfigjson: {base64 .docker/config.json}
type: kubernetes.io/dockerconfigjson
```

Apply the file through kubectl into the Kubernetes OSCAR cluster to create the secret. To use it in OSCAR services, you must add the secret name (`dockersecret` in this example) in the definition of the service, using the API or a FDL, under the `image_pull_secrets` parameter, or through the "Docker secret" field in OSCAR-UI.