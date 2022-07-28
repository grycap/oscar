#!/bin/bash

echo "SCRIPT: Invoked inference_ff.py. File available in $INPUT_FILE_PATH"
FILE_NAME=`basename $INPUT_FILE_PATH`
tmp_dir=`basename $TMP_OUTPUT_DIR`
filename_wo_extension="${FILE_NAME%.*}"
mv $INPUT_FILE_PATH "$INPUT_FILE_PATH.jpeg"
mkdir aux_output
cp "$INPUT_FILE_PATH.jpeg" aux_output/"input-"$FILE_NAME

python3 efficient-compact-fire-detection-cnn/inference_ff_oscar.py --image "$INPUT_FILE_PATH.jpeg" --output aux_output --model shufflenetonfire --weight efficient-compact-fire-detection-cnn/weights/shufflenet_ff.pt

zip -r "output-$filename_wo_extension-$tmp_dir.zip" aux_output
mv output-$filename_wo_extension-$tmp_dir.zip $TMP_OUTPUT_DIR
