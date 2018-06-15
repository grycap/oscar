#!/bin/bash

OUTPUT_DIR="/tmp/output"
echo "SCRIPT: Invoked Image Grayifier. File available in $SCAR_INPUT_FILE"
FILE_NAME=`basename $SCAR_INPUT_FILE`
OUTPUT_FILE=$OUTPUT_DIR/$FILE_NAME
echo "SCRIPT: Converting input image file $SCAR_INPUT_FILE to grayscale to output file $OUTPUT_FILE"
convert $SCAR_INPUT_FILE -type Grayscale $OUTPUT_FILE