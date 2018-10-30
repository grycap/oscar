FROM python:3-alpine

RUN mkdir -p /usr/src/app
WORKDIR /usr/src/app

COPY requirements.txt /usr/src/app/

RUN apk add --no-cache git && pip3 install --no-cache-dir -r requirements.txt

COPY . /usr/src/app

RUN ln -s /usr/src/app/bin/docker-linux-amd64 /usr/bin/docker

EXPOSE 8080

ENTRYPOINT ["python3"]

CMD ["-m", "swagger_server"]