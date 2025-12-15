#!/bin/bash

echo "SCRIPT: Rucio example. Files available in $INPUT_FILE_PATH"
echo "-----------"
cat "$INPUT_FILE_PATH"/*
echo "-----------"
echo -n "input  "
echo "$INPUT_FILE_PATH"
echo -n "output  "
echo "$TMP_OUTPUT_DIR  "

for filepath in "$INPUT_FILE_PATH"/*; do
  [ -f "$filepath" ] || continue

  filename=$(basename -- "$filepath")
  extension="${filename##*.}"
  base="${filename%.*}"

  rand=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 8)

  newname="${base}_${rand}"
  newpath="${folder}/${newname}"

  mv "$filepath" "$TMP_OUTPUT_DIR/$newpath"
  echo "Rename: $filename → $newname"
done

ls -la "$TMP_OUTPUT_DIR"/