#!/bin/sh

echo "Invoked isvc yolo. File available in $INPUT_FILE_PATH"

echo "Preparing tensor for $INPUT_FILE_PATH"

apt-get update && apt-get install -y \
    python3 python3-pip curl \
    libjpeg-turbo8 zlib1g

pip install pillow numpy pyyaml

python3 - <<EOF > /tmp/input.json
import numpy as np
from PIL import Image
import json

# Load and preprocess image
img = Image.open("$INPUT_FILE_PATH").convert("RGB")
img = img.resize((640, 640))   # adjust if your YOLO TF model uses a different size
arr = np.array(img).tolist()

# TF Serving expects: { "instances": [ <tensor> ] }
payload = {"instances": [arr]}

print(json.dumps(payload))
EOF

echo "Sending request to TensorFlow Serving..."

curl -v \
  -H "Content-Type: application/json" \
  -d @/tmp/input.json \
  http://yolo.oscar-svc.svc.cluster.local/v1/models/yolo:predict > /tmp/predictions.json

echo "Drawing bounding boxes using metadata.yaml..."

python3 <<EOF
import json
import yaml
from PIL import Image, ImageDraw

INPUT_FILE_PATH = "$INPUT_FILE_PATH"
PRED_PATH = "/tmp/predictions.json"
META_PATH = "$MOUNT_PATH/metadata.yaml"
OUT_PATH = "$TMP_OUTPUT_DIR/${INPUT_FILE_PATH##*/}_annotated.png"

# Load metadata
with open(META_PATH, "r") as f:
    meta = yaml.safe_load(f)

class_names = meta.get("names", {})

# Load original image
img = Image.open(INPUT_FILE_PATH).convert("RGB")
draw = ImageDraw.Draw(img)

# Load predictions
with open(PRED_PATH) as f:
    data = json.load(f)

preds = data["predictions"][0]

# Draw boxes
for det in preds:
    xmin, ymin, xmax, ymax, conf, cls = det

    if conf < 0.25:
        continue

    cls = int(cls)
    label = f"{class_names.get(cls, str(cls))} {conf:.2f}"

    draw.rectangle([xmin, ymin, xmax, ymax], outline="red", width=2)

    text_w, text_h = draw.textsize(label)
    draw.rectangle([xmin, ymin - text_h, xmin + text_w, ymin], fill="red")
    draw.text((xmin, ymin - text_h), label, fill="white")

img.save(OUT_PATH)
print(f"Saved annotated image to: {OUT_PATH}")
EOF

echo "Done."
