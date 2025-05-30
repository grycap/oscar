FROM alpine:latest

RUN apk add nodejs npm

RUN npm install -g --unsafe-perm node-red

COPY dependencies.txt /dependencies.txt
RUN xargs npm install -g --no-audit --no-update-notifier --no-fund --save --save-prefix=~ --omit=dev --engine-stric < /dependencies.txt
