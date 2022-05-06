# Plants classification synchronous invokation

This example uses the pre-trained classification model by DEEP-Hybrid-DataCloud [Plants species classifier](https://marketplace.deep-hybrid-datacloud.eu/modules/deep-oc-plants-classification-tf.html) and is prepared to be used with synchronous invokations. 

**Note: To use this example, your cluster must have KNative support.**

In order to invoke the function, first you have to do is create a service, either by the OSCAR UI or by using the FLD within the followign command.

``` sh
$ oscar-cli apply function.yaml
```

**Note: if you create the service via the OSCAR UI you have to select the option LOG LEVEL: CRITICAL on the creation panel.**

Once the service is created you can make the invokation with the following command, which will store the output on a minio bucket and also print it on the console.

``` sh
$ oscar-cli service run plants-function -i image1.jpg
```


