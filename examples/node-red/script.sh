#!/bin/sh

node-red --port 1880 -D uiHost="::" -D httpRoot=$NODE_RED_BASE_URL &

while true; do
  sleep 1
done
