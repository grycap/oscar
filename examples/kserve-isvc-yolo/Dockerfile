FROM python:3.11-slim-bookworm

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1

# Runtime dependencies for script.sh:
# - curl for calling KServe
# - pillow + numpy for image preprocessing/postprocessing
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       ca-certificates \
       curl \
       libjpeg62-turbo \
       zlib1g \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir numpy pillow

