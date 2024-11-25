# Object Detection with YOLOv8

Detect objects in images using the state-of-the-art YOLOv8 model.

## About YOLO

This node utilizes the YOLOv8 (You Only Look Once version 8) model to detect objects within images. YOLOv8 is a cutting-edge, real-time object detection system known for its speed and accuracy, capable of identifying thousands of object categories efficiently.

## About YOLOV8 Service in OSCAR

This service uses the pre-trained YOLOv8 model provided by DEEP-Hybrid-DataCloud for object detection. It is designed to handle synchronous invocations and real-time image processing with high scalability, managed automatically by an elastic Kubernetes cluster.

In order to invoke the function, first you have to create a service, either by the OSCAR UI or by using the FDL within the following command.


``` sh
oscar-cli apply yolov8.yaml
```

Once the service is created you can make the invocation with the following
command, which will store the output on a minio bucket.

``` sh
oscar-cli service put-file yolov8.yaml minio img/cat.jpg yolov8/input/cat.jpg