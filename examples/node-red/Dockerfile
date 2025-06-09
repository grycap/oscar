FROM alpine:latest

# Install dependencies
RUN apk update && apk upgrade
RUN apk add nodejs npm python3

# Install Node-RED globally
RUN npm install -g --unsafe-perm node-red
# Install OSCAR nodes for Node-RED
COPY dependencies.txt /dependencies.txt
RUN xargs npm install -g --no-audit --no-update-notifier --no-fund --save --save-prefix=~ --omit=dev --engine-stric < /dependencies.txt
RUN rm dependencies.txt

# Install dependencies: git
RUN apk add git

# Prepare directories for subflows and examples
RUN mkdir -p oscar-subflows
RUN mkdir -p oscar-examples

# Download subflows and examples from GitHub
RUN git clone --depth=1 https://github.com/ai4os/ai4-compose.git temp_repo

# Copy subflows to the oscar-subflows directory
RUN find temp_repo/node-red/flows -type f -name "*.json" | while read file; do \
        filename=$(basename "$file"); \
        parentdir=$(basename "$(dirname "$file")"); \
        cp "$file" "oscar-subflows/$filename"; \
    done

# Copy examples to the oscar-examples directory
RUN find temp_repo/node-red/examples -type f -name "*.json" | while read file; do \
        filename=$(basename "$file"); \
        parentdir=$(basename "$(dirname "$file")"); \
        cp "$file" "oscar-examples/$filename"; \
    done 

# Clean up temporary repository
RUN rm -rf temp_repo

# Remove git to reduce image size
RUN apk del git

# Clean up unnecessary files
RUN apk cache clean --force
RUN npm cache clean --force

# Disable npm bin-links to avoid issues with symlinks using MinIO
RUN npm config set bin-links false --global