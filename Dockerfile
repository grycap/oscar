FROM python:3-alpine

RUN mkdir -p /usr/src/app
WORKDIR /usr/src/app

COPY . .
RUN pip3 install --no-cache-dir -r requirements.txt

RUN mkdir bin && \
    wget https://github.com/openfaas/faas/releases/download/0.9.14/fwatchdog -O bin/fwatchdog && \
    wget https://github.com/grycap/faas-supervisor/releases/download/v0.8.5-beta/supervisor -O bin/supervisor && \
    chmod +x bin/*   

EXPOSE 8080

ENTRYPOINT ["python3"]
CMD ["-m", "swagger_server"]