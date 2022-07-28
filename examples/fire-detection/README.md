# Fire detection

This example is extracted from the Github repository [efficient-compact-fire-detection-cnn](https://github.com/NeelBhowmik/efficient-compact-fire-detection-cnn), which implements a real-time fire detection based on convolutional neural networks.

The inference file has been modified so the output can be an image of the superpixel fire prediction and a textfile with only the prediction as text (FIRE/NO_FIRE). 

The image for the service has been build an pushed to a public github registry but you can see the Dockerfile that generates it [here](Dockerfile).

Because this example is an asynchronous one, once you have the function deployed into an OSCAR cluster, you can upload to the specified input bucket the image that you want to process, through oscar-ui or whichever storage provider that you are using. 

As mentioned, the output would be an image and a textfile like the ones shown next:

![Prediction output](readme-images/prediction-output.png)




