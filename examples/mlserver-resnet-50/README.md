# MLServer ResNet-50 Image Classification Tutorial

This tutorial demonstrates how to deploy a ResNet-50 image classification model using MLServer and OSCAR. The example shows how to create an exposed service that can classify images using the Microsoft ResNet-50 model from Hugging Face.

## Overview

This example creates an OSCAR service that:
- Runs a ResNet-50 model for image classification
- Uses MLServer as the model serving framework
- Exposes an HTTP API endpoint for inference in OSCAR

## Files Description

### Core Files

- **`fdl.yaml`** - OSCAR Function Definition Language file that defines the service
- **`Dockerfile`** - Container image definition with MLServer and ResNet-50 model
- **`script.sh`** - Entry point script that starts nginx and MLServer
- **`model-settings.json`** - MLServer model configuration
- **`nginx.conf`** - Universal Nginx revers-proxy configuration for API routing and documentation for MLServer in OSCAR.

### Model Configuration

The `model-settings.json` file configures the ResNet-50 model:

```json
{
  "name": "resnet-50",
  "implementation": "mlserver_huggingface.HuggingFaceRuntime",
  "parameters": {
    "extra": {
      "task": "image-classification",
      "optimum_model": true,
      "pretrained_model": "microsoft/resnet-50",
      "device": -1
    }
  }
}
```

Key parameters:
- `task`: Defines the ML task type (image classification)
- `pretrained_model`: Specifies the Hugging Face model to use
- `device`: Set to -1 for CPU inference

## Deployment Steps

### 1. Deploy the Service

Deploy the service using the OSCAR CLI:

```bash
oscar-cli apply fdl.yaml
```

### 2. Verify Deployment

Check that the service is running:

```bash
oscar-cli service list
```

You should see the `resnet-50` service in the list.

## Usage

### API Endpoints

The service exposes several endpoints:

1. **Main API Endpoint**: `/system/services/resnet-50/exposed/v2`
2. **API Documentation**: `/system/services/resnet-50/exposed/v2/docs`
3. **Model-specific docs**: `/system/services/resnet-50/exposed/v2/models/resnet-50/docs`

### Making Predictions

#### How to know what to send

To make a request, you need to know what the model expects to receive; for that purpose, there is a metadata endpoint.  
- `https://<YOUR_CLUSTER>/system/services/resnet-50/exposed/v2/models/resnet-50`

#### Using curl

To classify an image, send a POST request to the inference endpoint:

```bash
curl -X POST "https://<YOUR_CLUSTER>/system/services/resnet-50/exposed/v2/models/resnet-50/infer" \
  -H "Content-Type: application/json" \
  -d '{
        "inputs": [
            {
            "name": "images",
            "shape": [0],
            "datatype": "BYTES",
            "data": "<IMAGE BASE64>"
            }
        ]
    }'
```

### Example Response

```json
{
  "model_name": "resnet-50",
  "id": "a22304ef-fa6d-44e3-9d87-2b69eb5fdc97",
  "parameters": {},
  "outputs": [
    {
      "name": "output",
      "shape": [
        1,
        1
      ],
      "datatype": "BYTES",
      "parameters": {
        "content_type": "hg_jsonlist"
      },
      "data": [
        "[{\"label\": \"tabby, tabby cat\", \"score\": 0.8528618812561035}, {\"label\": \"Egyptian cat\", \"score\": 0.07394339144229889}, {\"label\": \"tiger cat\", \"score\": 0.054769568145275116}, {\"label\": \"lynx, catamount\", \"score\": 0.0045639281161129475}, {\"label\": \"space heater\", \"score\": 0.00020687644428107888}]"
      ]
    }
  ]
}
```

## Service Configuration

### FDL Configuration

The `fdl.yaml` file defines the service parameters:

```yaml
functions:
  oscar:
  - oscar-cluster:
      name: resnet-50
      cpu: '1.0'
      memory: 2Gi
      image: ghcr.io/rk181/mlserver:resnet-50
      script: script.sh
      expose:
        min_scale: 1
        max_scale: 1
        api_port: 80
        cpu_threshold: 90
        rewrite_target: true
```

Key settings:
- **Resources**: 1 CPU core, 2GB memory
- **Scaling**: Min/max scale of 1 (always one instance)
- **Port**: Service listens on port 80
- **Auto-scaling**: Triggers at 90% CPU usage

### Nginx Configuration

The nginx configuration handles:
- API routing to the MLServer backend
- Documentation serving with proper URL rewriting
- HTTPS redirects and security headers

## Troubleshooting

### Common Issues

**Service fails to start**:
- Check resource limits in `fdl.yaml`
- Verify the Docker image is accessible

## Additional Resources

- [MLServer Documentation](https://mlserver.readthedocs.io/)
- [OSCAR Documentation](https://oscar.readthedocs.io/)
- [Hugging Face Models](https://huggingface.co/models)
- [ResNet-50 Model](https://huggingface.co/microsoft/resnet-50)
