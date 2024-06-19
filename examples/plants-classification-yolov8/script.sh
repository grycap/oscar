#!/bin/bash

IMAGE_NAME=`basename "$INPUT_FILE_PATH"`
OUTPUT_FILE="$TMP_OUTPUT_DIR/output.json"

deepaas-cli predict --files "$INPUT_FILE_PATH" 2>&1 | grep -Po '{.*}' > "$OUTPUT_FILE" 

echo "Prediction was saved in: $OUTPUT_FILE"