This is a sample OSCAR service which provides basic text analysis. It is an easy example to showcase OSCAR's ability to execute services both synchronously and asynchronously.

## Overview

This service reads a text file from the input path, performs basic analysis (counting words, and characters), and writes the results to the output path. It demonstrates how OSCAR services can process input and generate output using environment variables set by the OSCAR framework.

## Files

- [`simple-test.yaml`](simple-test.yaml): OSCAR Function Definition Language (FDL) file describing the service.
- [`script.sh`](script.sh): Bash script executed by the service.
- [`input.txt`](input.txt): Example input file.

## Deployment

To deploy the service, use [OSCAR-CLI](https://github.com/grycap/oscar-cli):

```sh
oscar-cli apply simple-test.yaml
```

This will create the service in your OSCAR cluster.

## Invocation

### Option 1. Synchronously:

```sh
oscar-cli service run simple-test --text-input "The quick brown fox jumped over the lazy dog"
```
This a blocking call until the output is obtained:
```
The quick brown fox jumped over the lazy dog
Analysis:
Words: 9
Characters: 44
```


### Option 2. Asynchronously 
You can invoke the service and provide an input file using OSCAR-CLI:

```sh
oscar-cli service put-file simple-test minio.default input.txt simple-test/input/input00.txt
```

This uploads `input.txt` to the MinIO bucket at the indicated input path to trigger the asynchronous execution of the OSCAR service.

Once the execution is finished, the output will be stored in the MinIO bucket at `simple-test/output/output.txt`.

You can transfer the output file from MinIO to a local path as follows:
```sh
oscar-cli service get-file simple-test minio.default simple-test/output/input00-out.txt /tmp/input00-out.txt
```
And view the contents of the file:

```sh
cat /tmp/input00-out.txt
```

## Script Details

The script uses the following environment variables:

- `INPUT_FILE_PATH`: Path to the input file.
- `TMP_OUTPUT_DIR`: Directory for output files.

It performs:

- Copying the input file to the output directory.
- Counting words, and characters.
- Writing the analysis to the output file.

See [`script.sh`](script.sh) for implementation details.

## Output

The output file will contain:

```
Analysis:
Words: <number>
Characters: <number>
```

## Synchronous and Asynchronous Execution

This service can be executed both synchronously (immediate result) and asynchronously (result available after processing), depending on how you invoke it via OSCAR.

For more information, see the [OSCAR documentation](https://docs.oscar.grycap.net).



