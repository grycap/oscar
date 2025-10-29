# Plants classification synchronous invocation

This example uses the pre-trained classification model by DEEP-Hybrid-DataCloud
[Plants species classifier](https://marketplace.deep-hybrid-datacloud.eu/modules/deep-oc-plants-classification-tf.html)
and is prepared to be used with synchronous invocations.

**Note: To use this example, you must enable a ServerlessBackend (Knative or OpenFaaS).**

In order to invoke the function, first you have to do is create a service,
either by the OSCAR UI or by using the FLD within the following command.

``` sh
oscar-cli apply plant-classification-sync.yaml
```

**Note: if you create the service via the OSCAR UI you have to select the
option LOG LEVEL: CRITICAL on the creation panel.**

Once the service is created you can make the invocation with the following
command, which will store the output on a minio bucket and also print it on
the console.

``` sh
oscar-cli service run plant-classification-sync -i images/image1.jpg
```
