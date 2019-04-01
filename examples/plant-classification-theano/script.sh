#!/bin/bash

echo "SCRIPT: Invoked classify_image.py. File available in $INPUT_FILE_PATH"
FILE_NAME=`basename $INPUT_FILE_PATH`
OUTPUT_FILE=$TMP_OUTPUT_DIR/$FILE_NAME
python2 /opt/plant-classification-theano/classify_image.py "$INPUT_FILE_PATH" -o "$OUTPUT_FILE"
