# OSCAR CLI

To deploy a service on an OSCAR cluster using the [OSCAR-CLI](https://docs.oscar.grycap.net/oscar-cli), the first step is to add a cluster so you can then manage it. To do this, use the [add](https://docs.oscar.grycap.net/oscar-cli/#add) command. In this case, we will call it `oscar-cluster`, so from now on, when working with our cluster using OSCAR-CLI, we will refer to it as `oscar-cluster`. Use the username and password you obtained when creating the OSCAR cluster. To use the OSCAR-CLI in a local deployment, you must set the `--disable-ssl` flag at the end to disable the verification of self-signed certificates.

``` bash
oscar-cli cluster add oscar-cluster https://localhost $OSCARuser $OSCARpass
```

If you want to use a remote OSCAR cluster that includes access via [EGI credentials](https://docs.oscar.grycap.net/integration-egi), use the following commands.

Via [OIDC agent](https://docs.oscar.grycap.net/integration-egi/#integration-with-oscar-cli-via-oidc-agent):

```bash
oscar-cli cluster add oscar-cluster https://oscar-cluster-remote -o oidc-account-name
```

Via [Access Token](https://docs.oscar.grycap.net/integration-egi/#obtaining-an-access-token):

```bash
oscar-cli cluster add oscar-cluster https://oscar-cluster-remote -t access-token
```

The next step is to create the [FDL](https://docs.oscar.grycap.net/fdl) file, which contains all the service configuration. Next, using this `.yaml` file, you can deploy the service(s) with the following [apply](https://docs.oscar.grycap.net/oscar-cli/#apply) command:

``` bash
oscar-cli apply $yaml_file
```

Using the [list](https://docs.oscar.grycap.net/oscar-cli/#list) command, you can verify if the service was deployed correctly to the cluster.

``` bash
oscar-cli service list -c oscar-cluster
```

This returns a list of all services deployed in the cluster.

```
NAME			IMAGE					CPU	 MEMORY
cowsay			ghcr.io/grycap/cowsay	1	 1Gi
...             ....                    ...  ...
```
With this, the service is deployed and ready to run (see [Service Execution](https://docs.oscar.grycap.net/invoking) section). Alternatively, you can remove the cluster from the OSCAR-CLI tool with the following command:

``` bash
oscar-cli cluster remove oscar-cluster
```
