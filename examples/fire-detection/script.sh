#!/bin/bash

echo "SCRIPT: Invoked inference_ff_oscar.py with file $INPUT_FILE_PATH"
FILE_NAME=`basename $INPUT_FILE_PATH`
filename_wo_extension="${FILE_NAME%.*}"
mv $INPUT_FILE_PATH "$INPUT_FILE_PATH.jpeg"

python3 efficient-compact-fire-detection-cnn/inference_ff_oscar.py --image "$INPUT_FILE_PATH.jpeg" --output "$TMP_OUTPUT_DIR" --model shufflenetonfire --weight efficient-compact-fire-detection-cnn/weights/shufflenet_ff.pt

zip -r $filename_wo_extension $TMP_OUTPUT_DIR
