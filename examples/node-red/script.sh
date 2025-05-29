#!/bin/sh

sleep 15
mkdir -p $NODE_RED_DIRECTORY
cp -r /root/.node-red/* "$NODE_RED_DIRECTORY"/

node-red --port 1880 --userDir $NODE_RED_DIRECTORY -D uiHost="::" -D httpRoot=$NODE_RED_BASE_URL &

while true; do
  sleep 1
done
