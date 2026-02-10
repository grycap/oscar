# KServe Sklearn Iris Classification Example

This example demonstrates how to deploy a machine learning inference service using KServe and OSCAR. The sklearn model classifies iris flowers based on their measurements.

## Prerequisites

- OSCAR cluster with KServe installed
- `oscar-cli` configured
- `kubectl` configured to access the cluster

## Files

- `sklearn-iris.yaml` - KServe InferenceService definition
- `isvc-sklearn.yaml` - OSCAR service definition
- `script.sh` - Script that sends requests to the InferenceService
- `iris-input.json` - Sample input data for inference

## Deployment Steps

### 1. Deploy the KServe InferenceService

First, deploy the sklearn model as a KServe InferenceService:

```bash
kubectl apply -f sklearn-iris.yaml
```

Wait for the InferenceService to be ready:

```bash
kubectl get inferenceservices sklearn-iris -n kserve-test
```

Expected output:
```
NAME           URL                                                      READY   PREV   LATEST   PREVROLLEDOUTREVISION   LATESTREADYREVISION
sklearn-iris   http://sklearn-iris.kserve-test.svc.cluster.local       True           100                              sklearn-iris-predictor-default-xxxxx
```

### 2. Deploy the OSCAR Service

Deploy the OSCAR service that will invoke the InferenceService:

```bash
oscar-cli apply isvc-sklearn.yaml
```

Verify the service is created:

```bash
oscar-cli service list
```

### 3. Upload the Input File

Upload the sample JSON file to trigger the inference:

```bash
oscar-cli service put-file isvc-sklearn-request minio isvc-sklearn-request/input iris-input.json
```

Alternatively, you can upload through the OSCAR GUI, the MinIO console or using the `mc` client.

### 4. Retrieve and View the Output

Wait a few seconds for the service to process the request, then retrieve the output log:

```bash
oscar-cli service get-file isvc-sklearn-request minio isvc-sklearn-request/output iris-input.json.out
```

The output should show the inference results with predictions like:

```json
{
  "predictions": [1, 1]
}
```

You can also check the logs:

```bash
oscar-cli service logs list isvc-sklearn-request
oscar-cli service logs get isvc-sklearn-request <job-id>
```

## How It Works

1. When you upload `iris-input.json` to the input bucket, OSCAR triggers the service
2. The service runs the `script.sh` which:
   - Reads the input file from `$INPUT_FILE_PATH`
   - Sends an HTTP POST request to the KServe InferenceService endpoint
   - The model predicts the iris species based on the measurements
3. The response is captured and stored in the output bucket

## Input Format

The input JSON should contain iris flower measurements in the format:

```json
{
  "instances": [
    [sepal_length, sepal_width, petal_length, petal_width],
    ...
  ]
}
```

## Model Information

- **Model Type**: Scikit-learn classifier
- **Model Location**: `gs://kfserving-examples/models/sklearn/1.0/model`
- **Endpoint**: `http://sklearn-iris.kserve-test.svc.cluster.local/v1/models/sklearn-iris:predict`
- **Predictions**: Returns integer class labels (0, 1, or 2 representing iris species)

## Troubleshooting

If the service fails to invoke the model:

1. Check that the InferenceService is ready:
   ```bash
   kubectl get inferenceservices -n kserve-test
   ```

2. Check OSCAR service logs:
   ```bash
   oscar-cli service logs list isvc-sklearn-request
   ```

3. Verify network connectivity between OSCAR and KServe namespaces

4. Check KServe predictor pod logs:
   ```bash
   kubectl logs -n kserve-test -l serving.kserve.io/inferenceservice=sklearn-iris
   ```
