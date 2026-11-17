# Model Serving YOLOv8n ONNX Object Detection Example

This example demonstrates how to deploy a YOLOv8n object detection model (exported to ONNX) with OSCAR Model Serving and integrate it with an OSCAR workflow. Model Serving uses KServe underneath, which must be enabled in the cluster to deploy and use this example. When an image is uploaded to the configured MinIO bucket, OSCAR triggers `script.sh`, which preprocesses the image, calls the Model Serving v2 inference endpoint, applies confidence filtering and non-maximum suppression (NMS), and writes the results back to MinIO.

## Architecture

```
MinIO (input) → OSCAR service → script.sh → Model Serving (KServe underneath; Triton + YOLOv8n ONNX) → MinIO (output)
```

**Output files per inference:**
- `*_predictions_raw.json` – Raw Model Serving v2 response
- `*_predictions_filtered.json` – Detections after confidence + NMS filtering
- `*_predictions_summary.txt` – Human-readable detection summary
- `*_annotated.jpg` – Input image with bounding boxes drawn

## Prerequisites

- OSCAR cluster with KServe enabled (required by Model Serving)
- `oscar-cli` configured to point to the cluster or `https://dashboard.oscar.grycap.net`

## Files

| File | Description |
|------|-------------|
| `oscar-svc-yolo8n.yaml` | OSCAR FDL definition with embedded workflow with model serving configuration |
| `script.sh` | Inference script run by OSCAR (preprocessing → model serving call → postprocessing) |
| `Dockerfile` | Builds the model storage image (`busybox` + ONNX model file) |
| `Dockerfile.script` | Builds the script runner image (`python:3.11-slim` + `numpy`, `pillow`, `curl`) |
| `onnx/8/yolov8n.onnx` | YOLOv8n model in ONNX format |
| `image01.png`, `image02.jpeg` | Sample test images |

## Deployment Steps

### 1. Deploy via OSCAR

Deploy the OSCAR service, which also provisions the model-serving deployment using KServe:

```bash
oscar-cli apply oscar-svc-yolo8n.yaml
```

> Note: direct apply works only with OSCAR's admin user. If you use OIDC, update `MODEL_SERVING_ENDPOINT` to replace `oscar-svc` with `oscar-svc-{first eight characters of the OIDC ID}`. For example: `http://yolo8n-onnx-model-predictor.oscar-svc-666d7ac0.svc.cluster.local`.

Verify the service was created:

```bash
oscar-cli service list
```

### 2. Run an Inference

Upload a test image to the OSCAR service input bucket to trigger processing:

```bash
oscar-cli service put-file yolo8n-onnx-input-output minio image01.png yolo8n-onnx-input-output/input/image01.png
```
> Note: it can take several minutes to deploy the model and download it, especially if it's the first time.

### 3. Retrieve the Output

Wait a few seconds for the job to complete, then list and download the output files:

```bash
oscar-cli service list-files yolo8n-onnx-input-output minio yolo8n-onnx-input-output/output
oscar-cli service get-file yolo8n-onnx-input-output minio yolo8n-onnx-input-output/output/<filename>
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
