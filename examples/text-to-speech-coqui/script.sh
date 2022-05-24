#!/bin/bash
#FILE_NAME=
FILE_NAME=`basename $INPUT_FILE_PATH`
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME"

text=$(cat $INPUT_FILE_PATH)

tts --text "$text" --model_name "tts_models/en/ljspeech/tacotron2-DDC" --vocoder_name "vocoder_models/en/ljspeech/hifigan_v2" --out_path $OUTPUT_FILE
