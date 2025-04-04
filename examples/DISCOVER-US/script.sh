#!/bin/sh

FILE_NAME=`basename "$INPUT_FILE_PATH"`
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME"

python3 fish_detector.py -i "$INPUT_FILE_PATH" -o "$OUTPUT_FILE"

echo  $?
