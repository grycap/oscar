FROM bitnami/minideb:bullseye

RUN apt update && \
    apt install -y --no-install-recommends cowsay jq libexpat1 && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*