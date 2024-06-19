# Plants Classifitaction (Yolo v8)

This example uses the pre-trained classification model by DEEP-Hybrid-DataCloud
[Plants Classification](https://dashboard.cloud.ai4eosc.eu/marketplace/modules/plants-classification)
and is prepared to be used with asynchronous invocations.


In order to invoke the function, first you have to do is create a service,
either by the OSCAR UI or by using the FDL within the following command.

``` sh
oscar-cli apply plants-classification-yolov8.yaml
```

Once the service is created you can make the invocation with the following
command, which will store the output on a minio bucket.

``` sh
oscar-cli service put-file plants-classification minio images/plants.png plants-classification/input/plants.png
```