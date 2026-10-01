#!/bin/bash

SLEEP_SECONDS="${SLEEP_SECONDS:-90}"
FILE_NAME=$(basename "$INPUT_FILE_PATH" | cut -d. -f1)
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME-out.txt"

cat "$INPUT_FILE_PATH" > "$OUTPUT_FILE"
echo "Running on $(hostname) for ${SLEEP_SECONDS}s..." | tee -a "$OUTPUT_FILE"
sleep "$SLEEP_SECONDS"
echo "Finished sleeping for ${SLEEP_SECONDS}s." | tee -a "$OUTPUT_FILE"
