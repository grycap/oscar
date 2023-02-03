#!/bin/bash
#echo "SCRIPT: Invoked dislib-rf. File available in $INPUT_FILE_PATH"
FILE_NAME=`basename "$INPUT_FILE_PATH" | cut -f 1 -d '.'`
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME.txt"
#mv output.log "$OUTPUT_FILE-output.log"

#unzip $INPUT_FILE_PATH  /opt/folder

#tar -x  /opt/folder

/etc/init.d/ssh start

#runcompss --pythonpath=$(pwd) --python_interpreter=python3 /opt/wordcount_merge.py /opt/folder  > $OUTPUT_FILE
runcompss  --classpath=/opt/simple.jar simple.Simple `cat $INPUT_FILE_PATH` > $OUTPUT_FILE
#mv output.log "$OUTPUT_FILE-output.log"

# --pythonpath=$(pwd) \
# --python_interpreter=python3 \
# /home/user/load_rf_predict.py /home/user/models/rf_model pickle $INPUT_FILE_PATH 1 500 1 &> >(tee output.log)
