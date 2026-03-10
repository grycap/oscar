#!/bin/bash
set -e

WS_DIR="/data"
STATE_FILE="$WS_DIR/counter.txt"

mkdir -p "$WS_DIR"

COUNTER=0
if [ -f "$STATE_FILE" ]; then
  COUNTER=$(cat "$STATE_FILE")
fi

COUNTER=$((COUNTER + 1))
echo "$COUNTER" > "$STATE_FILE"

echo "workspace_path=$WS_DIR"
echo "counter=$COUNTER"
echo "state_file=$STATE_FILE"
