#!/bin/bash
FILE_NAME=`basename "$INPUT_FILE_PATH" | cut -f 1 -d '.'`
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME.txt"
/etc/init.d/ssh start
runcompss  --classpath=/opt/simple.jar simple.Simple `cat $INPUT_FILE_PATH` > $OUTPUT_FILE