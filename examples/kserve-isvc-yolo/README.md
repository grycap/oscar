# KServe YOLOv8n ONNX Object Detection Example

This example demonstrates how to deploy a YOLOv8n object detection model (exported to ONNX) as a KServe InferenceService and integrate it with OSCAR. When an image is uploaded to the configured MinIO bucket, OSCAR triggers `script.sh`, which preprocesses the image, calls the KServe v2 inference endpoint, applies confidence filtering and non-maximum suppression (NMS), and writes the results back to MinIO.

## Architecture

```
MinIO (input) → OSCAR service → script.sh → KServe (Triton + YOLOv8n ONNX) → MinIO (output)
```

**Output files per inference:**
- `*_predictions_raw.json` – Raw KServe v2 response
- `*_predictions_filtered.json` – Detections after confidence + NMS filtering
- `*_predictions_summary.txt` – Human-readable detection summary
- `*_annotated.jpg` – Input image with bounding boxes drawn

## Prerequisites

- OSCAR cluster with KServe installed
- `oscar-cli` configured to point to the cluster or `https://dashboard.oscar.grycap.net`

## Files

| File | Description |
|------|-------------|
| `oscar-svc-yolo8n.yaml` | OSCAR FDL definition with embedded KServe configuration |
| `script.sh` | Inference script run by OSCAR (preprocessing → KServe call → postprocessing) |
| `Dockerfile` | Builds the model storage image (`busybox` + ONNX model file) |
| `Dockerfile.script` | Builds the script runner image (`python:3.11-slim` + `numpy`, `pillow`, `curl`) |
| `onnx/8/yolov8n.onnx` | YOLOv8n model in ONNX format |
| `image01.png`, `image02.jpeg` | Sample test images |

## Deployment Steps

### 1. Deploy via OSCAR

Deploy the OSCAR service, which also provisions the KServe InferenceService automatically:

```bash
oscar-cli apply oscar-svc-yolo8n.yaml
```

Verify the service was created:

```bash
oscar-cli service list
```

### 2. Run an Inference

Upload a test image to the OSCAR service input bucket to trigger processing:

```bash
oscar-cli service put-file kserve-isvc-yolo8n-onnx minio kserve-isvc-yolo8n-onnx/input image01.png
```
> Note: it can take several minutes to deploy the KServe InferenceService and download the model, especially if it's the first time.

### 3. Retrieve the Output

Wait a few seconds for the job to complete, then list and download the output files:

```bash
oscar-cli service list-files kserve-isvc-yolo8n-onnx minio kserve-isvc-yolo8n-onnx/output
oscar-cli service get-file kserve-isvc-yolo8n-onnx minio kserve-isvc-yolo8n-onnx/output <filename> .
```

You can also browse results through the Dashboard.

## Building the Images

**Model image** (contains the ONNX file, served at startup):

```bash
docker build -t ghcr.io/grycap/kserve-yolo8n-onnx -f Dockerfile .
```

> Note: the image must be built and pushed to a registry before deploying the service

**Script runner image** (Python environment for `script.sh`):

```bash
docker build -t ghcr.io/grycap/kserve-yolo8n-onnx-script -f Dockerfile.script .
```
