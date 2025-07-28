#!/bin/sh

if [ "$INPUT_TYPE" = "json" ]
then
    jq '.message' "$INPUT_FILE_PATH" -r | /usr/games/cowsay && echo "$COWSAY_SECRET" | /usr/games/cowsay
else
    cat "$INPUT_FILE_PATH" | /usr/games/cowsay && echo "$COWSAY_SECRET" | /usr/games/cowsay
fi
