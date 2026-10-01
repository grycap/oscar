#!/bin/sh

configured_message=${MESSAGE:-}
header_message=${Http_X_Message:-}
payload_message=""

if jq -e . "$INPUT_FILE_PATH" >/dev/null 2>&1; then
    payload_message=$(jq -r '.environment.MESSAGE // empty' "$INPUT_FILE_PATH")
fi

effective_message=$configured_message
source="service environment"

if [ -n "$payload_message" ]; then
    effective_message=$payload_message
    source="invocation payload"
fi

if [ -n "$header_message" ]; then
    effective_message=$header_message
    source="HTTP header"
fi

printf 'Configured MESSAGE: %s\n' "$configured_message"
printf 'Header Http_X_Message: %s\n' "${header_message:-<unset>}"
printf 'Payload environment.MESSAGE: %s\n' "${payload_message:-<unset>}"
printf 'Effective MESSAGE: %s\n' "$effective_message"
printf 'Selected source: %s\n' "$source"
