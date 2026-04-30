# syntax=docker/dockerfile:1.7

FROM golang:1.25 AS build

ARG VERSION
ARG GIT_COMMIT
ARG GOOS=linux

WORKDIR /oscar

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY main.go ./
COPY pkg ./pkg

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${GOOS} CGO_ENABLED=0 go build --ldflags "-s -w \
-X \"github.com/grycap/oscar/v3/pkg/version.Version=${VERSION}\" \
-X \"github.com/grycap/oscar/v3/pkg/version.GitCommit=${GIT_COMMIT}\"" \
    -o oscar .


FROM node:20-alpine AS ui-build

WORKDIR /dashboard

COPY dashboard/package.json ./

RUN --mount=type=cache,target=/root/.npm \
    npm install

COPY dashboard /dashboard

RUN --mount=type=cache,target=/root/.npm \
    node scripts/deploy_container.cjs && npm run build


FROM alpine:3.14

LABEL org.label-schema.license="Apache 2.0" \
    org.label-schema.vcs-url="https://github.com/grycap/oscar" \
    org.label-schema.vcs-type="Git" \
    org.label-schema.name="grycap/oscar" \
    org.label-schema.vendor="grycap" \
    org.label-schema.docker.schema-version="1.0"

RUN addgroup -S app \
    && adduser -S -g app app \
    && apk add --no-cache ca-certificates

WORKDIR /home/app

EXPOSE 8080

COPY --from=build /oscar/oscar .
COPY --from=ui-build /dashboard/dist assets

RUN chown -R app:app ./

USER app

CMD ["./oscar"]
