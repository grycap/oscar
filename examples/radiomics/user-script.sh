#!/bin/bash

echo "SCRIPT: Invoked video_classification.py. File available in $INPUT_FILE_PATH"
FILE_NAME=`basename "$INPUT_FILE_PATH"`
FILE_NAME="${FILE_NAME%.*}"
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME"
python3 /opt/radiomics/video_classification.py "$INPUT_FILE_PATH" -o "$OUTPUT_FILE.txt"
