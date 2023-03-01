#!/bin/bash

IMAGE_NAME=`basename "$INPUT_FILE_PATH"`
OUTPUT_IMAGE="$TMP_OUTPUT_DIR/"

deepaas-predict -i "$INPUT_FILE_PATH" -ct application/zip -o $OUTPUT_IMAGE
