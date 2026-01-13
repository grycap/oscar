# OSCAR API

To facilitate interaction and integration with OSCAR services, a high-performance and secure [REST API](api.md) is available. This API can be used with any programming language (Python, C, Java, etc.) or API interaction tool such as Postman, cURL, etc.

To create a service, the POST method is used as shown in the following figure:

![API-create-service](images/deployment-service/api-service.png)

Two simple alternatives will be given on how to interact with the API to deploy a service on OSCAR.

- `cURL`

[cURL](https://curl.se/), a well-known tool in technical circles, sits at the intersection of web development and system administration. This powerful command allows users to query URLs from the command line, offering a quick and efficient way to interact with websites and web services without the need for a graphical interface.

To deploy a service, you must have the [FDL](fdl.md) file that defines the service and the script that will be executed on it.

First, you need the credentials to access the cluster. This can be via an [OIDC Token](integration-egi.md) or basic authentication.

In that case, we give an example of creating the [cowsay service](https://github.com/grycap/oscar/tree/master/examples/cowsay) using a cURL request with an [OIDC Token](integration-egi.md). Basically, it involves putting the FDL file and the script inside a simple POST request.

```bash
curl -X POST "https://oscar-cluster-remote/system/services" \
     -H "Authorization: Bearer YOUR_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
  "name": "cowsay",
  "cpu": "1.0",
  "memory": "1Gi",
  "image": "ghcr.io/grycap/cowsay",
  "script": "#!/bin/sh\n\nif [ \"$INPUT_TYPE\" = \"json\" ]\nthen\n    jq \".message\" \"$INPUT_FILE_PATH\" -r | /usr/games/cowsay\nelse\n    cat \"$INPUT_FILE_PATH\" | /usr/games/cowsay\nfi",
  "log_level": "CRITICAL",
  "vo": "vo.ai4eosc.eu",
  "environment": { "Variables": { "INPUT_TYPE": "json" } }
}'
```
To see if the service is active and review its current configuration.

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
     "https://oscar-cluster-remote/system/services/cowsay"
```
> ❗️
> If you have basic authentication, replace `-H "Authorization: Bearer ..."` with `-u "user:password"`, cURL automatically generates the `Authorization: Basic [Base64]` header.

- `POSTMAN`

[Postman](https://www.postman.com) is one of the most popular tools developers use to test, document, and collaborate on APIs, especially REST APIs. The following is a brief example of deploying a service on an OSCAR cluster using its API.

To deploy a service, in this case the [cowsay service](https://github.com/grycap/oscar/tree/master/examples/cowsay), first a POST request is created and the API address for service deployment is entered. In this example, a remote cluster called `oscar-cluster-remote` is used.

![API-Postman](images/deployment-service/api-postman-init.png)

The request must be configured for the type of authentication used. The figure shows both basic authentication (username and password) and authentication with an [OIDC Token](integration-egi.md).

![API-Postman-auth](images/deployment-service/api-postman-cred.png)

The request body is also configured, where the definition of the service to be deployed is entered. This information is taken from both the FDL file and the script. Once configured, the request can be sent to the OSCAR cluster.

![API-Postman-body](images/deployment-service/api-postman-body.png)


With this, the service is deployed and ready to run (see [Service Execution](invoking.md) section)

 