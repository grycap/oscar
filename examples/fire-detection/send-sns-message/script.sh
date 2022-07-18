#!/bin/bash

#Result processing
unzip $INPUT_FILE_PATH
result=`cat aux_output/output-*.txt`
if [ $result == "FIRE" ]; then
	echo "Fire detected. Sending message..."
	aws sns publish --topic-arn $TOPIC_ARN --message "Fire detected!" --subject "Fire detection service"
else
	echo "pass..."
fi
