#!/bin/bash

OUTPUT_DIR="/tmp/output"
echo "SCRIPT: Invoked classify_image.py. File available in $SCAR_INPUT_FILE"
FILE_NAME=`basename $SCAR_INPUT_FILE`
OUTPUT_FILE=$OUTPUT_DIR/$FILE_NAME

python2 /opt/plant-classification-theano/classify_image.py $SCAR_INPUT_FILE -o $OUTPUT_FILE