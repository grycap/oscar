#!/bin/sh

sleep 15
mkdir -p $NODE_RED_DIRECTORY
mkdir -p $NODE_RED_DIRECTORY/lib
mkdir -p $NODE_RED_DIRECTORY/lib/flows
mkdir -p $NODE_RED_DIRECTORY/lib/flows/oscar-subflows
mkdir -p $NODE_RED_DIRECTORY/lib/flows/oscar-examples
cp /oscar-subflows/* $NODE_RED_DIRECTORY/lib/flows/oscar-subflows
cp /oscar-examples/* $NODE_RED_DIRECTORY/lib/flows/oscar-examples

NODE_RED_PWD_HASH=$(echo "$PASSWORD" | node-red admin hash-pw | cut -d' ' -f2)

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
