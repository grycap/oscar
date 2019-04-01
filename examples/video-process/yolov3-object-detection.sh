#!/bin/bash

FILENAME="`basename $INPUT_FILE_PATH`"
# Remove extension from filename
FILENAME=${FILENAME%.*}
RESULT="$TMP_OUTPUT_DIR/$FILENAME.out"
OUTPUT_IMAGE="$TMP_OUTPUT_DIR/$FILENAME"

echo "SCRIPT: Analyzing file '$INPUT_FILE_PATH', saving the result in '$RESULT' and the output image in '$OUTPUT_IMAGE'"

cd /opt/darknet
./darknet detect cfg/yolov3.cfg yolov3.weights $INPUT_FILE_PATH -out $OUTPUT_IMAGE > $RESULT
