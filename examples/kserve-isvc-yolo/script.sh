#!/bin/sh

set -e

echo "Invoked YOLO request-only flow"
echo "INPUT_FILE_PATH=$INPUT_FILE_PATH"
echo "TMP_OUTPUT_DIR=$TMP_OUTPUT_DIR"

command -v python3 >/dev/null 2>&1 || { echo "python3 not found"; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "curl not found"; exit 1; }
python3 -c "import numpy, PIL" >/dev/null 2>&1 || { echo "python deps missing (numpy/pillow)"; exit 1; }

INPUT_NAME="$(basename "$INPUT_FILE_PATH")"
INPUT_STEM="${INPUT_NAME%.*}"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT_JSON_PATH="$TMP_OUTPUT_DIR/${INPUT_STEM}_${TS}_predictions_raw.json"

MODEL_INPUT_W="${MODEL_INPUT_W:-640}"
MODEL_INPUT_H="${MODEL_INPUT_H:-640}"
NORMALIZE_INPUT="${NORMALIZE_INPUT:-true}"
USE_BGR="${USE_BGR:-false}"
KSERVE_INPUT_NAME="${KSERVE_INPUT_NAME:-images}"
KSERVE_URL="${KSERVE_URL:-http://yolo-onnx-predictor.oscar-svc.svc.cluster.local/v2/models/yolo-onnx/infer}"
CONF_THRESHOLD="${CONF_THRESHOLD:-0.10}"
NMS_IOU="${NMS_IOU:-0.45}"
MAX_DETECTIONS="${MAX_DETECTIONS:-100}"

echo "Preparing request payload..."
python3 - <<EOF > /tmp/input.json
import json
import numpy as np
from PIL import Image

img = Image.open("${INPUT_FILE_PATH}").convert("RGB").resize((int("${MODEL_INPUT_W}"), int("${MODEL_INPUT_H}")))
x = np.array(img, dtype=np.float32)

if "${USE_BGR}".strip().lower() in ("1", "true", "yes", "on"):
    x = x[..., ::-1]

if "${NORMALIZE_INPUT}".strip().lower() in ("1", "true", "yes", "on"):
    x = x / 255.0

# HWC -> CHW
x = np.transpose(x, (2, 0, 1))
# Add batch dimension: [1,3,H,W]
x = np.expand_dims(x, axis=0)

input_name = "${KSERVE_INPUT_NAME}".strip()
if not input_name:
    raise SystemExit("KSERVE_INPUT_NAME must be set for v2 protocol")

payload = {
    "inputs": [
        {
            "name": input_name,
            "shape": list(x.shape),
            "datatype": "FP32",
            "data": x.tolist(),
        }
    ]
}

print(json.dumps(payload))
EOF

echo "Calling KServe endpoint: $KSERVE_URL"
REQUEST_START_MS="$(date +%s%3N)"
curl \
  --fail \
  --silent --show-error \
  -H "Content-Type: application/json" \
  -d @/tmp/input.json \
  "$KSERVE_URL" > "$OUT_JSON_PATH"
REQUEST_END_MS="$(date +%s%3N)"
REQUEST_DURATION_MS="$((REQUEST_END_MS - REQUEST_START_MS))"
REQUEST_DURATION_S="$(awk "BEGIN {printf \"%.3f\", ${REQUEST_DURATION_MS}/1000}")"

echo "Saved raw response to: $OUT_JSON_PATH"
echo "Request latency: ${REQUEST_DURATION_MS} ms (${REQUEST_DURATION_S} s)"

OUT_FILTERED_JSON_PATH="$TMP_OUTPUT_DIR/${INPUT_STEM}_${TS}_predictions_filtered.json"
OUT_SUMMARY_PATH="$TMP_OUTPUT_DIR/${INPUT_STEM}_${TS}_predictions_summary.txt"

python3 - <<EOF
import json
import sys
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

raw_path = Path("${OUT_JSON_PATH}")
filtered_path = Path("${OUT_FILTERED_JSON_PATH}")
summary_path = Path("${OUT_SUMMARY_PATH}")
annotated_path = Path("${TMP_OUTPUT_DIR}") / f"${INPUT_STEM}_${TS}_annotated.jpg"
conf_threshold = float("${CONF_THRESHOLD}")
nms_iou = float("${NMS_IOU}")
max_detections = int("${MAX_DETECTIONS}")
class_names = {
    0: "person", 1: "bicycle", 2: "car", 3: "motorcycle", 4: "airplane",
    5: "bus", 6: "train", 7: "truck", 8: "boat", 9: "traffic light",
    10: "fire hydrant", 11: "stop sign", 12: "parking meter", 13: "bench", 14: "bird",
    15: "cat", 16: "dog", 17: "horse", 18: "sheep", 19: "cow",
    20: "elephant", 21: "bear", 22: "zebra", 23: "giraffe", 24: "backpack",
    25: "umbrella", 26: "handbag", 27: "tie", 28: "suitcase", 29: "frisbee",
    30: "skis", 31: "snowboard", 32: "sports ball", 33: "kite", 34: "baseball bat",
    35: "baseball glove", 36: "skateboard", 37: "surfboard", 38: "tennis racket", 39: "bottle",
    40: "wine glass", 41: "cup", 42: "fork", 43: "knife", 44: "spoon",
    45: "bowl", 46: "banana", 47: "apple", 48: "sandwich", 49: "orange",
    50: "broccoli", 51: "carrot", 52: "hot dog", 53: "pizza", 54: "donut",
    55: "cake", 56: "chair", 57: "couch", 58: "potted plant", 59: "bed",
    60: "dining table", 61: "toilet", 62: "tv", 63: "laptop", 64: "mouse",
    65: "remote", 66: "keyboard", 67: "cell phone", 68: "microwave", 69: "oven",
    70: "toaster", 71: "sink", 72: "refrigerator", 73: "book", 74: "clock",
    75: "vase", 76: "scissors", 77: "teddy bear", 78: "hair drier", 79: "toothbrush"
}

def flatten_rows(value):
    if isinstance(value, list):
        if len(value) >= 6 and all(isinstance(x, (int, float)) for x in value[:6]):
            return [value]
        rows = []
        for item in value:
            rows.extend(flatten_rows(item))
        return rows
    return []

try:
    data = json.loads(raw_path.read_text())
except Exception as e:
    print(f"Could not parse raw response JSON: {e}")
    sys.exit(0)

candidates = []
if isinstance(data, dict):
    if "predictions" in data:
        candidates.append(data["predictions"])
    if "outputs" in data:
        candidates.append(data["outputs"])
elif isinstance(data, list):
    candidates.append(data)

data_list = None

if isinstance(data, dict) and "outputs" in data and isinstance(data["outputs"], list) and data["outputs"]:
    out0 = data["outputs"][0]
    if isinstance(out0, dict):
        shape = out0.get("shape")
        flat = None
        if "data" in out0:
            flat = out0["data"]
        elif "contents" in out0 and isinstance(out0["contents"], dict) and "fp32_contents" in out0["contents"]:
            flat = out0["contents"]["fp32_contents"]
        if flat is not None and shape:
            arr = __import__("numpy").array(flat, dtype=float).reshape(shape)
            if arr.ndim == 3:
                if arr.shape[1] == 84 and arr.shape[2] != 84:
                    arr = arr.transpose(0, 2, 1)
                arr = arr[0]
            data_list = arr

rows = []
output_shape = None
if data_list is None:
    for c in candidates:
        if isinstance(c, list):
            # Handle v2 outputs list of dicts
            if c and isinstance(c[0], dict) and ("data" in c[0] or "contents" in c[0]):
                out0 = c[0]
                shape = out0.get("shape")
                flat = None
                if "data" in out0:
                    flat = out0["data"]
                elif "contents" in out0 and isinstance(out0["contents"], dict) and "fp32_contents" in out0["contents"]:
                    flat = out0["contents"]["fp32_contents"]
                if flat is not None and shape:
                    arr = __import__("numpy").array(flat, dtype=float).reshape(shape)
                    if arr.ndim == 3:
                        if arr.shape[1] == 84 and arr.shape[2] != 84:
                            arr = arr.transpose(0, 2, 1)
                        arr = arr[0]
                    data_list = arr
                    break
            rows.extend(flatten_rows(c))
        elif isinstance(c, dict):
            if "data" in c:
                rows.append(c["data"])
                if "shape" in c:
                    output_shape = c["shape"]
            elif "contents" in c:
                contents = c["contents"]
                if isinstance(contents, dict) and "fp32_contents" in contents:
                    rows.append(contents["fp32_contents"])
                    if "shape" in c:
                        output_shape = c["shape"]

def to_array(rows):
    arr = None
    try:
        arr = __import__("numpy").array(rows, dtype=float)
    except Exception:
        return None
    return arr

if data_list is None:
    arr = to_array(rows)
    if arr is None:
        arr = __import__("numpy").array(candidates, dtype=float)

    if arr is None:
        data_list = []
    else:
        # Normalize shape to [N, K]
        if arr.ndim == 3:
            # Either [1, 84, 8400] or [1, 8400, 84]
            if arr.shape[1] == 84 and arr.shape[2] != 84:
                arr = arr.transpose(0, 2, 1)
            arr = arr[0]
        elif arr.ndim == 4:
            arr = arr.reshape(arr.shape[0], -1, arr.shape[-1])[0]
        elif arr.ndim == 1 and output_shape:
            try:
                arr = arr.reshape(output_shape)
                if arr.ndim == 3:
                    if arr.shape[1] == 84 and arr.shape[2] != 84:
                        arr = arr.transpose(0, 2, 1)
                    arr = arr[0]
            except Exception:
                pass
        data_list = arr

detections = []

import numpy as np

def nms(boxes, scores, iou_threshold):
    if len(boxes) == 0:
        return []
    x1 = boxes[:, 0]
    y1 = boxes[:, 1]
    x2 = boxes[:, 2]
    y2 = boxes[:, 3]
    areas = (x2 - x1) * (y2 - y1)
    order = scores.argsort()[::-1]
    keep = []
    while order.size > 0:
        i = order[0]
        keep.append(i)
        xx1 = np.maximum(x1[i], x1[order[1:]])
        yy1 = np.maximum(y1[i], y1[order[1:]])
        xx2 = np.minimum(x2[i], x2[order[1:]])
        yy2 = np.minimum(y2[i], y2[order[1:]])
        w = np.maximum(0.0, xx2 - xx1)
        h = np.maximum(0.0, yy2 - yy1)
        inter = w * h
        iou = inter / (areas[i] + areas[order[1:]] - inter + 1e-9)
        inds = np.where(iou <= iou_threshold)[0]
        order = order[inds + 1]
    return keep

if isinstance(data_list, np.ndarray) and data_list.ndim == 2 and data_list.shape[1] >= 6:
    # Two common cases:
    # 1) NMS baked in: [N,6] => x1,y1,x2,y2,conf,cls
    # 2) Raw head: [N,84] => cx,cy,w,h + class scores
    if data_list.shape[1] == 6:
        for row in data_list:
            xmin, ymin, xmax, ymax, conf, cls = row[:6]
            conf = float(conf)
            if conf < conf_threshold:
                continue
            class_id = int(cls)
            detections.append({
                "class_id": class_id,
                "class_name": class_names.get(class_id, str(class_id)),
                "confidence": round(conf, 6),
                "bbox_xyxy": [float(xmin), float(ymin), float(xmax), float(ymax)],
            })
    else:
        boxes = data_list[:, :4]
        scores = data_list[:, 4:]
        max_scores = scores.max(axis=1)
        class_ids = scores.argmax(axis=1)
        keep = np.where(max_scores > conf_threshold)[0]
        if keep.size > 0:
            boxes = boxes[keep]
            scores = max_scores[keep]
            class_ids = class_ids[keep]
            # cx,cy,w,h -> xyxy
            x1 = boxes[:, 0] - boxes[:, 2] / 2
            y1 = boxes[:, 1] - boxes[:, 3] / 2
            x2 = boxes[:, 0] + boxes[:, 2] / 2
            y2 = boxes[:, 1] + boxes[:, 3] / 2
            boxes_xyxy = np.stack([x1, y1, x2, y2], axis=1)
            nms_keep = nms(boxes_xyxy, scores, nms_iou)
            if max_detections > 0:
                nms_keep = nms_keep[:max_detections]
            for i in nms_keep:
                class_id = int(class_ids[i])
                detections.append({
                    "class_id": class_id,
                    "class_name": class_names.get(class_id, str(class_id)),
                    "confidence": round(float(scores[i]), 6),
                    "bbox_xyxy": [float(x) for x in boxes_xyxy[i]],
                })

detections.sort(key=lambda d: d["confidence"], reverse=True)
if max_detections > 0:
    detections = detections[:max_detections]

result = {
    "confidence_threshold": conf_threshold,
    "nms_iou": nms_iou,
    "max_detections": max_detections,
    "num_detections": len(detections),
    "detections": detections,
}
filtered_path.write_text(json.dumps(result, indent=2))

lines = [
    f"Detections: {len(detections)} (conf >= {conf_threshold}, nms_iou={nms_iou}, max={max_detections})",
]
for i, d in enumerate(detections, 1):
    b = d["bbox_xyxy"]
    lines.append(
        f"{i:02d}. class={d['class_id']} ({d['class_name']}) conf={d['confidence']:.4f} "
        f"bbox=[{b[0]:.1f},{b[1]:.1f},{b[2]:.1f},{b[3]:.1f}]"
    )
summary_path.write_text("\n".join(lines) + "\n")

print(f"Saved filtered response to: {filtered_path}")
print(f"Saved summary to: {summary_path}")

# Draw annotated image
try:
    img = Image.open("${INPUT_FILE_PATH}").convert("RGB")
    w, h = img.size
    scale_x = w / float("${MODEL_INPUT_W}")
    scale_y = h / float("${MODEL_INPUT_H}")

    draw = ImageDraw.Draw(img)
    try:
        font = ImageFont.truetype("arial.ttf", 20)
    except Exception:
        font = ImageFont.load_default()

    for d in detections:
        x1, y1, x2, y2 = d["bbox_xyxy"]
        x1 *= scale_x
        x2 *= scale_x
        y1 *= scale_y
        y2 *= scale_y
        x1, x2 = sorted([x1, x2])
        y1, y2 = sorted([y1, y2])
        label = f"{d['class_name']} {d['confidence']:.2f}"
        draw.rectangle([x1, y1, x2, y2], outline="red", width=2)
        draw.text((x1, max(0, y1 - 20)), label, fill="red", font=font)

    img.save(annotated_path, quality=90)
    print(f"Saved annotated image to: {annotated_path}")
except Exception as e:
    print(f"Failed to create annotated image: {e}")
EOF

echo "Tip: set CONF_THRESHOLD (e.g. 0.40), NMS_IOU (e.g. 0.45), MAX_DETECTIONS (e.g. 20)."
echo "Done."
