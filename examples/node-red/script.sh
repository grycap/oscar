#!/bin/sh

sleep 15
mkdir -p $NODE_RED_DIRECTORY
mkdir -p $NODE_RED_DIRECTORY/lib
mkdir -p $NODE_RED_DIRECTORY/lib/flows
mkdir -p $NODE_RED_DIRECTORY/lib/flows/oscar-subflows
cp /oscar-subflows/* $NODE_RED_DIRECTORY/lib/flows/oscar-subflows

node-red --port 1880 --userDir $NODE_RED_DIRECTORY -D uiHost="::" -D credentialSecret=false -D contextStorage.default.module=localfilesystem -D httpRoot=$NODE_RED_BASE_URL &

while true; do
  sleep 1
done

