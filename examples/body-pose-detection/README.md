# Body Pose Invocation asynchronous invocation

This example uses the pre-trained classification model by DEEP-Hybrid-DataCloud
[Body Pose Detection](https://marketplace.deep-hybrid-datacloud.eu/modules/deep-oc-posenet-tf.html)
and is prepared to be used with asynchronous invocations.


In order to invoke the function, first you have to do is create a service,
either by the OSCAR UI or by using the FLD within the following command.

``` sh
oscar-cli apply body-pose-detection-async.yaml
```

Once the service is created you can make the invocation with the following
command, which will store the output on a minio bucket.

``` sh
oscar-cli service put-file body-pose-detection-async minio images/001.jpg body-pose-detection-async/input/001.jpg
```

