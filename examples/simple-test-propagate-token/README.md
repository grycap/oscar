This example extends the `simple-test` service to showcase the `propagate_token` functionality. When `propagate_token` is set, OSCAR injects the service access token into the container as the `ACCESS_TOKEN` environment variable. The script prints the token so you can confirm it is available.

## Overview

The service reads a text file from the input path, performs basic analysis (counting words and characters), and writes the results to the output path. In addition, it captures the propagated access token and writes it both to the output file and the container logs.

## Files

- [`simple-test-propagate-token.yaml`](simple-test-propagate-token.yaml): OSCAR FDL describing the service with `propagate_token: true`.
- [`script.sh`](script.sh): Bash script executed by the service that prints the access token.
- [`input.txt`](input.txt): Example input file.

## Deployment

Deploy the service with [OSCAR-CLI](https://github.com/grycap/oscar-cli):

```sh
oscar-cli apply simple-test-propagate-token.yaml
```

This creates the service in your OSCAR cluster.

## Invocation

### Option 1. Synchronous

```sh
oscar-cli service run simple-test-propagate-token --text-input "The quick brown fox jumped over the lazy dog"
```

Sample output:

```
The quick brown fox jumped over the lazy dog
Analysis:
Words: 9
Characters: 44
ACCESS_TOKEN: <token-value>
```

The token is also echoed to the container logs for easy inspection.

### Option 2. Asynchronous

Upload an input file:

```sh
oscar-cli service put-file simple-test-propagate-token minio.default input.txt simple-test-propagate-token/input/input00.txt
```

When the execution finishes, the result is stored at `simple-test-propagate-token/output/input00-out.txt`. Retrieve it with:

```sh
oscar-cli service get-file simple-test-propagate-token minio.default simple-test-propagate-token/output/input00-out.txt /tmp/input00-out.txt
cat /tmp/input00-out.txt
```

## Script Details

The script relies on these environment variables:

- `INPUT_FILE_PATH`: Path to the input file.
- `TMP_OUTPUT_DIR`: Directory for output files.
- `ACCESS_TOKEN`: Service access token injected when `propagate_token` is enabled.

It performs:

- Copying the input file to the output directory.
- Counting words and characters.
- Writing the analysis and the propagated token to the output file (and logging the token).

## Security Note

Printing the token is useful for demonstration, but avoid exposing tokens in production workflows.
