FROM seldonio/mlserver:1.7.1-huggingface

# Prepare huggingface cache directory for transformers v4.41.2
RUN mkdir -p /opt/mlserver/.cache/hub && \
    echo "1" > "/opt/mlserver/.cache/hub/version.txt"
ENV HF_HUB_CACHE="/opt/mlserver/.cache/hub"

# Download the model and its dependencies
RUN pip install --no-cache-dir --break-system-packages "huggingface_hub[cli]" transformers==4.41.2 && \
    huggingface-cli download microsoft/resnet-50 && \
    pip cache purge

USER root

# Update the system and install nginx or any other required packages
RUN microdnf update -y && \
    microdnf install -y nginx  && \
    microdnf clean all

# Create the model directory 
# (the /opt/mlserver directory is used by MLServer and it is mandatory if using seldonio/mlserver images) 
RUN mkdir -p /opt/mlserver/models
COPY model-settings.json /opt/mlserver/models

# Copy the nginx configuration file (it is universal for all models) 
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Disable gzip compression for save problems with the proxy 
ENV MLSERVER_GZIP_ENABLED="false"
ENV MLSERVER_METRICS_ENDPOINT=""

ENTRYPOINT [ "" ]