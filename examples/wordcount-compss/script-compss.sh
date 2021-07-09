#!/bin/bash

echo "SCRIPT: Invoked wordcount.sh. File available in $INPUT_FILE_PATH"
FILE_NAME=`basename "$INPUT_FILE_PATH"`
OUTPUT_FILE="$TMP_OUTPUT_DIR/result_$FILE_NAME"
/app/wordcount.sh "$INPUT_FILE_PATH" "$OUTPUT_FILE" 1000
