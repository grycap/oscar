#!/bin/bash

#
# This script reads a file from the input path and writes it to the output path.
# It uses environment variables set by the OSCAR framework: 
# - INPUT_FILE_PATH: the path to the input file
# - TMP_OUTPUT_DIR: the directory where output files should be written


FILE_NAME=`basename "$INPUT_FILE_PATH" | cut -d. -f1`  # Get the base name of the input file without extension
OUTPUT_FILE="$TMP_OUTPUT_DIR/$FILE_NAME-out.txt"

cat "$INPUT_FILE_PATH" > "$OUTPUT_FILE"

# Basic text analysis
WORD_COUNT=$(wc -w < "$INPUT_FILE_PATH")
CHAR_COUNT=$(wc -m < "$INPUT_FILE_PATH")

echo "File $FILE_NAME was processed. Output saved in: $OUTPUT_FILE"
echo "\nAnalysis:" >> "$OUTPUT_FILE"
echo "Words: $WORD_COUNT" >> "$OUTPUT_FILE"
echo "Characters: $CHAR_COUNT" >> "$OUTPUT_FILE"

