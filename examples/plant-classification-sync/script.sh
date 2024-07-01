#!/bin/bash

IMAGE_NAME=`basename $INPUT_FILE_PATH`
OUTPUT_FILE="$TMP_OUTPUT_DIR/$IMAGE_NAME"

mv $INPUT_FILE_PATH "$INPUT_FILE_PATH.jpg"

echo "SCRIPT: Invoked deepaas-cli command." 
deepaas-cli --deepaas_method_output $OUTPUT_FILE predict --files "$INPUT_FILE_PATH.jpg"
