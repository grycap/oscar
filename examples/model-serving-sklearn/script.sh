#!/bin/sh

echo "Invoked sklearn. File available in $INPUT_FILE_PATH"
cat "$INPUT_FILE_PATH"
PAYLOAD_NAME=$(basename "$INPUT_FILE_PATH")
OUTPUT_FILE="$TMP_OUTPUT_DIR/output_$PAYLOAD_NAME"

curl -v -H "Content-Type: application/json" $MODEL_SERVING_ENDPOINT/v1/models/sklearn-model:predict -d @/$INPUT_FILE_PATH > $OUTPUT_FILE
cat $OUTPUT_FILE