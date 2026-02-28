FROM --platform=$BUILDPLATFORM node:alpine AS front-builder
WORKDIR /app
COPY frontend/ ./
RUN npm install && npm run build

FROM golang:1.25-alpine AS backend-builder
WORKDIR /app
ARG TARGETARCH
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
ENV GOARCH=$TARGETARCH

RUN apk update && apk add --no-cache \
    gcc \
    musl-dev \
    libc-dev \
    make \
    git \
    wget \
    unzip \
    bash

ENV CC=gcc

COPY . .
COPY --from=front-builder /app/dist/ /app/web/html/

RUN go build -ldflags="-w -s" \
    -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_musl" \
    -o sui main.go

FROM --platform=$TARGETPLATFORM alpine
LABEL org.opencontainers.image.authors="alireza7@gmail.com"
ENV TZ=Asia/Tehran
WORKDIR /app
RUN set -ex && apk add --no-cache --upgrade bash tzdata ca-certificates nftables
COPY --from=backend-builder /app/sui /app/
COPY entrypoint.sh /app/
ENTRYPOINT [ "./entrypoint.sh" ]