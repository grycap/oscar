# Model Serving Sklearn Iris Classification Example

This example demonstrates how to deploy a scikit-learn Iris classification model with OSCAR Model Serving. Model Serving uses KServe underneath, which must be enabled in the cluster to deploy and use this example. When a JSON file is uploaded to the configured MinIO bucket, OSCAR triggers `script.sh`, which calls the Model Serving v1 inference endpoint and writes the predictions back to MinIO.

## Architecture

```
MinIO (input) → OSCAR service → script.sh → Model Serving (KServe underneath; sklearn Iris model) → MinIO (output)
```

**Output per inference:**
- `output_<input-filename>` – JSON file with predicted class labels

## Prerequisites

- OSCAR cluster with KServe enabled (required by Model Serving)
- `oscar-cli` configured to point to the cluster or `https://dashboard.oscar.grycap.net`

## Files

| File | Description |
|------|-------------|
| `oscar-svc-sklearn.yaml` | OSCAR FDL definition with a Model Serving configuration |
| `script.sh` | Inference script run by OSCAR (sends input JSON to the Model Serving endpoint and saves the response) |
| `iris-input.json` | Sample input with two Iris flower measurements |

## Deployment Steps

### 1. Deploy via OSCAR

Deploy the OSCAR service, which also provisions the model-serving deployment using KServe:

```bash
oscar-cli apply oscar-svc-sklearn.yaml
```

> Note: direct apply works only with OSCAR's admin user. If you use OIDC, update `MODEL_SERVING_ENDPOINT` to replace `oscar-svc` with `oscar-svc-{first eight characters of the OIDC ID}`. For example: `http://sklearn-model-predictor.oscar-svc-666d7ac0.svc.cluster.local`.


Verify the service was created:

```bash
oscar-cli service list
```

### 2. Run an Inference

Upload the sample input file to the OSCAR service input bucket to trigger processing:

```bash
oscar-cli service put-file sklearn-input-output minio iris-input.json sklearn-input-output/input/iris-input.json
```

> Note: it can take several minutes to deploy the model-serving deployment and download the model, especially if it's the first time.

### 3. Retrieve the Output

Wait a few seconds for the job to complete, then list and download the output files:

```bash
oscar-cli service list-files sklearn-input-output minio sklearn-input-output/output
oscar-cli service get-file sklearn-input-output minio sklearn-input-output/output/<filename>
```

The output JSON will contain the predicted class for each input instance:

```json
{"predictions": [1, 1]}
```

You can also browse results through the Dashboard.

## Input Format

The input must be a JSON file following the Model Serving v1 predict protocol:

```json
{
  "instances": [
    [6.8, 2.8, 4.8, 1.4],
    [6.0, 3.4, 4.5, 1.6]
  ]
}
```
