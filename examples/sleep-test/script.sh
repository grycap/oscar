#!/bin/bash

#
# This script reads a file from the input path, copies it to the output path,
# and sleeps for the requested number of seconds.
# It uses environment variables set by OSCAR:
# - INPUT_FILE_PATH: path to the input file
# - TMP_OUTPUT_DIR: directory for output files
# - SLEEP_SECONDS: how many seconds to sleep (default: 30)

SLEEP_SECONDS="${SLEEP_SECONDS:-30}"

FILE_NAME=$(basename "$INPUT_FILE_PATH" | cut -d. -f1)  # Base name without extension
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME-out.txt"

cat "$INPUT_FILE_PATH" > "$OUTPUT_FILE"

echo "Sleeping for ${SLEEP_SECONDS}s..." | tee -a "$OUTPUT_FILE"
sleep "$SLEEP_SECONDS"
echo "Finished sleeping for ${SLEEP_SECONDS}s." | tee -a "$OUTPUT_FILE"
