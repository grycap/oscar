#!/bin/bash

mkdir -p /tmp/data/srv/thermal-bridges-rooftops-detector
cp -r /srv/thermal-bridges-rooftops-detector/* /tmp/data/srv/thermal-bridges-rooftops-detector/
chmod +x /tmp/data/srv/thermal-bridges-rooftops-detector/tbbrdet_api/scripts/execute_inference.sh

FILE_NAME=`basename "$INPUT_FILE_PATH"`

CONFIG_FILE_PATH="/tmp/data/srv/thermal-bridges-rooftops-detector/models/swin/coco/2023-12-07_130038/mask_rcnn_swin-t-p4-w7_fpn_fp16_ms-crop-3x_coco.pretrained.py"

mkdir -p /tmp/data

CHECKPOINT_FILE_PATH="/tmp/data/srv/thermal-bridges-rooftops-detector/models/swin/coco/2023-12-07_130038/best_AR@1000_epoch_33.pth"
OUTPUT_DIR="/tmp/data/srv/thermal-bridges-rooftops-detector/models/swin/coco/2023-12-07_130038/predictions"

mkdir -p /tmp/data/srv/thermal-bridges-rooftops-detector/tbbrdet_api/scripts

cp srv/thermal-bridges-rooftops-detector/tbbrdet_api/scripts/execute_inference.sh /tmp/data/srv/thermal-bridges-rooftops-detector/tbbrdet_api/scripts/execute_inference.sh
chmod +x /tmp/data/srv/thermal-bridges-rooftops-detector/tbbrdet_api/scripts/execute_inference.sh

mkdir -p "$OUTPUT_DIR"

./tmp/data/srv/thermal-bridges-rooftops-detector/tbbrdet_api/scripts/execute_inference.sh \
  --input "$INPUT_FILE_PATH" \
  --config-file "$CONFIG_FILE_PATH" \
  --ckp-file "$CHECKPOINT_FILE_PATH" \
  --score-threshold 0.5 \
  --channel both

RESULT_FILE=$(find "$OUTPUT_DIR" -type f -name "*${FILE_NAME%.*}*.png" | head -n 1)

if [ -f "$RESULT_FILE" ]; then
  cp "$RESULT_FILE" "$TMP_OUTPUT_DIR/"
else
  echo "The result file was not found in $OUTPUT_DIR" >&2
  exit 1
fi
