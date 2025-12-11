# OSCAR PYTHON

From Python, you can deploy a service using the [OSCAR API](deployment-api.md). However, a client has also been developed that can interact with OSCAR more easily.

> ❗️
>All customer information is described in the following repositories: [OSCAR-Python](https://github.com/grycap/oscar_python) and [oscar client](https://pypi.org/project/oscar-python/). 

Once the library is installed (`pip install oscar-python`), you can begin developing and deploying a service. First, you must initialize the client using some form of authentication.

Initialize a client with basic authentication

```
options_basic_auth = {'cluster_id':'cluster-id',
                'endpoint':'https://oscar-cluster-remote',
                'user':'username',
                'password':'password',
                'ssl':'True'}

client = Client(options = options_basic_auth)

```
If you already have a valid token, you can use the parameter [oidc_token](integration_egi.md) instead. 

```
options_oidc_auth = {'cluster_id':'cluster-id',
                'endpoint':'https://oscar-cluster-remote',
                'oidc_token':'token',
                'ssl':'True'}
                
client = Client(options = options_oidc_auth)

```
Then we use the `client.create_service` function to deploy the service. The only parameter to keep in mind is the location of the FDL file containing the service configuration. After that, everything is ready to invoke the deployed service.

```
try:
    client.create_service("cowsay.yaml")
except Exception as err:
    print("Failed with: ", err)

```