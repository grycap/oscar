#!/bin/bash
set -e

VOLUME_DIR="/data"
STATE_FILE="$VOLUME_DIR/counter.txt"

mkdir -p "$VOLUME_DIR"

COUNTER=0
if [ -f "$STATE_FILE" ]; then
  COUNTER=$(cat "$STATE_FILE")
fi

COUNTER=$((COUNTER + 1))
echo "$COUNTER" > "$STATE_FILE"

echo "volume_path=$VOLUME_DIR"
echo "counter=$COUNTER"
echo "state_file=$STATE_FILE"
