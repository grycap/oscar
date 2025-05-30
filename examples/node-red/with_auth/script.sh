#!/bin/sh

sleep 15
mkdir -p $NODE_RED_DIRECTORY

NODE_RED_PWD_HASH=$(echo "$NODE_RED_PWD" | node-red admin hash-pw | cut -d' ' -f2)

node-red --port 1880 --userDir $NODE_RED_DIRECTORY \
  -D uiHost="::" \
  -D httpRoot=$NODE_RED_BASE_URL \
  -D credentialSecret=false \
  -D contextStorage.default.module=localfilesystem \
  -D adminAuth="{\"type\":\"credentials\",\"users\":[{\"username\":\"admin\",\"password\":\"$NODE_RED_PWD_HASH\",\"permissions\":\"*\"}]}" \
  &

while true; do
  sleep 1
done
