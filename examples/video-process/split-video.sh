#!/bin/sh

FILE_NAME=`basename "$INPUT_FILE_PATH"`
echo "SCRIPT: Splitting video file $INPUT_FILE_PATH in images and storing them in $TMP_OUTPUT_DIR. One image taken each second"
ffmpeg -loglevel info -nostats -i "$INPUT_FILE_PATH" -q:v 1 -vf fps=1 "$TMP_OUTPUT_DIR/$FILE_NAME-out%03d.jpg" < /dev/null
