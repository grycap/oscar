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