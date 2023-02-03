#!/bin/bash
#echo "SCRIPT: Invoked dislib-rf. File available in $INPUT_FILE_PATH"
FILE_NAME=`basename "$INPUT_FILE_PATH" | cut -f 1 -d '.'`
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME.txt"
#mv output.log "$OUTPUT_FILE-output.log"

#tar --extract -f  $INPUT_FILE_PATH  -C /opt/folder
#unzip $INPUT_FILE_PATH  /opt/folder

#tar -x  /opt/folder
#input=$(`cat `)
file=$(cat $INPUT_FILE_PATH)
incrementNumber=$(echo "$file" | cut -f 1 -d ';')
counter1=$(echo "$file" | cut -f 2 -d ';')
counter2=$(echo "$file" | cut -f 3 -d ';')
counter3=$(echo "$file" | cut -f 4 -d ';')
/etc/init.d/ssh start
cd /opt/increment
compss_build_app increment
runcompss --lang=c --project=./xml/templates/project.xml  master/increment $incrementNumber $counter1 $counter2  $counter3 > $OUTPUT_FILE
#runcompss --pythonpath=$(pwd) --python_interpreter=python3 /opt/wordcount_merge.py /opt/folder  > $OUTPUT_FILE
#runcompss  --classpath=/opt/wordcount.jar wordcount.multipleFiles.Wordcount /opt/folder > $OUTPUT_FILE
#runcompss --lang=c /opt/ simple <initial_number>
#mv output.log "$OUTPUT_FILE-output.log"

# --pythonpath=$(pwd) \
# --python_interpreter=python3 \
# /home/user/load_rf_predict.py /home/user/models/rf_model pickle $INPUT_FILE_PATH 1 500 1 &> >(tee output.log)
