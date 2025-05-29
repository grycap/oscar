FROM alpine:latest

RUN apk add nodejs npm

RUN npm install -g --unsafe-perm node-red
RUN mkdir -p ~/.node-red
RUN mkdir -p ~/.node-red/context

# Copy and install dependencies
COPY dependencies.txt /dependencies.txt
RUN cd ~/.node-red && xargs npm install < /dependencies.txt