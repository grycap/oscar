#!/bin/bash

FILENAME="`basename $SCAR_INPUT_FILE`"
RESULT="$SCAR_OUTPUT_FOLDER/$FILENAME.out"
OUTPUT_IMAGE="$SCAR_OUTPUT_FOLDER/$FILENAME"

echo "SCRIPT: Analyzing file '$SCAR_INPUT_FILE', saving the result in '$RESULT' and the output image in '$OUTPUT_IMAGE.png'"

cd /opt/darknet
./darknet detect cfg/yolov3.cfg yolov3.weights $SCAR_INPUT_FILE -out $OUTPUT_IMAGE > $RESULT