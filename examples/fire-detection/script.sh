#!/bin/bash

echo "SCRIPT: Invoked inference_ff.py. File available in $INPUT_FILE_PATH"
FILE_NAME=`basename $INPUT_FILE_PATH`
filename_wo_extension="${FILE_NAME%.*}"
cp "$INPUT_FILE_PATH" $TMP_OUTPUT_DIR"/input-"$FILE_NAME

python3 efficient-compact-fire-detection-cnn/inference_ff_oscar.py --image $INPUT_FILE_PATH --output $TMP_OUTPUT_DIR --model shufflenetonfire --weight efficient-compact-fire-detection-cnn/weights/shufflenet_ff.pt

zip -r -j "output-$filename_wo_extension.zip" $TMP_OUTPUT_DIR
mv output-$filename_wo_extension.zip $TMP_OUTPUT_DIR

if $SEND_SNS ; then
   result=`cat $TMP_OUTPUT_DIR/output-*.txt`
   event_time=`echo $EVENT | jq .Records[0].eventTime`
   if [ $result = "FIRE" ]; then
	echo "Fire detected. Sending message..."
	aws sns publish --topic-arn $TOPIC_ARN --message "Fire detected on image $FILE_NAME at $event_time" --subject "Fire detection service"
   fi
fi

rm "$TMP_OUTPUT_DIR/output-$filename_wo_extension.txt" "$TMP_OUTPUT_DIR/output-$FILE_NAME" "$TMP_OUTPUT_DIR/input-$FILE_NAME"