#!/bin/bash
docker build -t ghcr.io/grycap/wordcount-compss-python .
docker push ghcr.io/grycap/wordcount-compss-python