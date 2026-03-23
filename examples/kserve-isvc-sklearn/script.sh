#!/bin/sh

echo "Invoked isvc sklearn. File available in $INPUT_FILE_PATH"
cat "$INPUT_FILE_PATH"

curl -v -H "Content-Type: application/json" http://$KSERVE_HOST/v1/models/kserve-isvc-sklearn:predict -d @/$INPUT_FILE_PATH
