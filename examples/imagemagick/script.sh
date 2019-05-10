#!/bin/bash

echo "SCRIPT: Invoked Image Grayifier. File available in $INPUT_FILE_PATH"
FILE_NAME=`basename "$INPUT_FILE_PATH"`
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME"
echo "SCRIPT: Converting input image file $INPUT_FILE_PATH to grayscale to output file $OUTPUT_FILE"
convert "$INPUT_FILE_PATH" -type Grayscale "$OUTPUT_FILE"
