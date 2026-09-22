FROM alpine:3.24

RUN apk add --no-cache ca-certificates

COPY oscar-bin-downloader.sh .
RUN chmod +x oscar-bin-downloader.sh && mkdir /data

CMD ["./oscar-bin-downloader.sh"]