# KServe Sklearn Iris Classification Example

This example demonstrates how to deploy a scikit-learn Iris classification model as a KServe InferenceService and integrate it with OSCAR. When a JSON file is uploaded to the configured MinIO bucket, OSCAR triggers `script.sh`, which calls the KServe v1 inference endpoint and writes the predictions back to MinIO.

## Architecture

```
MinIO (input) → OSCAR service → script.sh → KServe (sklearn Iris model) → MinIO (output)
```

**Output per inference:**
- `output_<input-filename>` – JSON file with predicted class labels

## Prerequisites

- OSCAR cluster with KServe installed
- `oscar-cli` configured to point to the cluster or `https://dashboard.oscar.grycap.net`

## Files

| File | Description |
|------|-------------|
| `oscar-svc-sklearn.yaml` | OSCAR FDL definition with embedded KServe configuration |
| `isvc-sklearn.yaml` | Raw KServe `InferenceService` manifest (sklearn format, model from GCS) |
| `script.sh` | Inference script run by OSCAR (sends input JSON to KServe and saves the response) |
| `iris-input.json` | Sample input with two Iris flower measurements |

## Deployment Steps

### 1. Deploy via OSCAR

Deploy the OSCAR service, which also provisions the KServe InferenceService automatically:

```bash
oscar-cli apply oscar-svc-sklearn.yaml
```

Verify the service was created:

```bash
oscar-cli service list
```

### 2. Run an Inference

Upload the sample input file to the OSCAR service input bucket to trigger processing:

```bash
oscar-cli service put-file kserve-isvc-sklearn minio kserve-isvc-sklearn/input iris-input.json
```

> Note: it can take several minutes to deploy the KServe InferenceService and download the model, especially if it's the first time.

### 3. Retrieve the Output

Wait a few seconds for the job to complete, then list and download the output files:

```bash
oscar-cli service list-files kserve-isvc-sklearn minio kserve-isvc-sklearn/output
oscar-cli service get-file kserve-isvc-sklearn minio kserve-isvc-sklearn/output <filename> .
```

The output JSON will contain the predicted class for each input instance:

```json
{"predictions": [1, 1]}
```

You can also browse results through the Dashboard.

## Input Format

The input must be a JSON file following the KServe v1 predict protocol:

```json
{
  "instances": [
    [6.8, 2.8, 4.8, 1.4],
    [6.0, 3.4, 4.5, 1.6]
  ]
}
```
