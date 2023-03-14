#!/bin/bash
FILE_NAME=`basename "$INPUT_FILE_PATH" | cut -f 1 -d '.'`
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME.txt"

file=$(cat $INPUT_FILE_PATH)
incrementNumber=$(echo "$file" | cut -f 1 -d ';')
counter1=$(echo "$file" | cut -f 2 -d ';')
counter2=$(echo "$file" | cut -f 3 -d ';')
counter3=$(echo "$file" | cut -f 4 -d ';')
/etc/init.d/ssh start
cd /opt/increment
compss_build_app increment
runcompss --lang=c --project=./xml/templates/project.xml  master/increment $incrementNumber $counter1 $counter2  $counter3 > $OUTPUT_FILE
