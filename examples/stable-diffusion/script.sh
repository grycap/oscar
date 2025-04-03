#!/bin/bash

echo "SCRIPT: Invoked stable diffusion text to image."
FILE_NAME=`basename "$INPUT_FILE_PATH"`
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME.png"

prompt=`cat "$INPUT_FILE_PATH"`
echo "SCRIPT: Converting input prompt '$INPUT_FILE_PATH' to image :)"
python3 text2image.py --prompt="$prompt" --output=$OUTPUT_FILE