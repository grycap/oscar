FROM alpine:latest

RUN apk add nodejs npm

RUN npm install -g --unsafe-perm node-red
RUN mkdir -p ~/.node-red
RUN mkdir -p ~/.node-red/context
RUN cd ~/.node-red && npm install node-red-dashboard