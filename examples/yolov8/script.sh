#!/bin/bash

IMAGE_NAME=`basename "$INPUT_FILE_PATH"`
OUTPUT_FILE="$TMP_OUTPUT_DIR/output.png"

deepaas-cli --deepaas_method_output="$OUTPUT_FILE" predict --files "$INPUT_FILE_PATH" --accept image/png 2>&1

echo "Prediction was saved in: $OUTPUT_FILE"