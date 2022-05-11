#!/bin/bash

IMAGE_NAME=`basename $INPUT_FILE_PATH`
OUTPUT_FILE="$TMP_OUTPUT_DIR/$IMAGE_NAME"

mv $INPUT_FILE_PATH "$INPUT_FILE_PATH.jpg"

echo "SCRIPT: Invoked deepaas-predict command. File available in $INPUT_FILE_PATH." 
deepaas-predict -i "$INPUT_FILE_PATH.jpg" -o $OUTPUT_FILE 
