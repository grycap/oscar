# Dog breed detector module

This example uses a pre-trained classification model, which can be found on the DeePaaS marketplace [link]. This example uses a generic script implemented to work with all models that have installed the deepaas-cli command line interface with a version <=2.3.1 instead of a specifically defined script. Moreover, it can be used both with asynchronous and synchronous invocations.

The steps to use this are the same as other examples, first you need to create the service, either using oscar-cli with the following command

``` sh
oscar-cli apply dog-breed-detector.yaml
```

or through the graphical interface of your cluster.

Usually, DeePaaS models need some given parameters to be defined alongside the input of the inference invocation. To solve this, the service receives a JSON type in the following format where you can define, on the one hand, the key of the JSON the name and value of the parameter to be used on the command line and the other hand, inside the array 'oscar-files' each of the files codified as a base64 string, and the extension of it.

``` json
{
    'network': 'Resnet50',
    'oscar-files': [
        {
            'key': 'files',
            'file_format': 'jpg',
            'data': [BASE_64_ENCODED_FILE],
        }
    ]
}
```
As you can see, this example needs to set the parameter `network` and receives a `jpg` file. So, to invoke the service synchronous you should make a call like the following 

``` sh
oscar-cli service run dog-breed-detector --input inputparams.json
```

or uploading the file asynchronous to the MinIO bucket.