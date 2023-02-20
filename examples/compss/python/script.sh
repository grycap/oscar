#!/bin/bash
FILE_NAME=`basename "$INPUT_FILE_PATH" | cut -f 1 -d '.'`
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME.txt"

tar --extract -f  $INPUT_FILE_PATH  -C /opt/folder


/etc/init.d/ssh start

runcompss --pythonpath=$(pwd) --python_interpreter=python3 /opt/wordcount_merge.py /opt/folder  > $OUTPUT_FILE