#!/bin/sh

echo "Invoked isvc sklearn. File available in $INPUT_FILE_PATH"
cat "$INPUT_FILE_PATH"
#curl -v -H "Content-Type: application/json" http://sklearn-iris.kserve-test.svc.cluster.local/v1/models/sklearn-iris:predict -d @/tmp/tmpzc1b5o4i/iris-input.json
curl -v -H "Content-Type: application/json" http://sklearn-iris.kserve-test.svc.cluster.local/v1/models/sklearn-iris:predict -d @/$INPUT_FILE_PATH
