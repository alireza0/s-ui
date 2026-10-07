FROM --platform=$BUILDPLATFORM node:26-alpine AS front-builder
WORKDIR /app
COPY frontend/ ./
RUN npm ci && npm run build

FROM golang:1.27-alpine AS backend-builder
WORKDIR /app
ARG TARGETARCH
ARG TARGETVARIANT
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
ENV GOARCH=$TARGETARCH

RUN apk upgrade --no-cache --scripts=no apk-tools && \
    apk add --no-cache \
    gcc \
    musl-dev \
    libc-dev \
    make \
    git \
    wget \
    unzip \
    bash \
    curl

ENV CC=gcc

RUN CRONET_ARCH="$TARGETARCH" && \
    CRONET_URL="https://github.com/SagerNet/cronet-go/releases/latest/download/libcronet-linux-${CRONET_ARCH}.so"; \
    echo "Downloading $CRONET_URL" && \
    wget -q -O ./libcronet.so "$CRONET_URL" && \
    chmod 755 ./libcronet.so

COPY . .
COPY --from=front-builder /app/dist/ /app/web/html/

RUN if [ "$TARGETARCH" = "arm" ]; then export GOARM=7; [ "$TARGETVARIANT" = "v6" ] && export GOARM=6; fi; \
    . ./build-tags.sh && \
    TAGS=$(tags_for docker) && \
    LDFLAGS=$(ldflags_for docker) && \
    go build -ldflags="$LDFLAGS" -tags "$TAGS" -o sui main.go

FROM alpine:3
LABEL org.opencontainers.image.authors="alireza7@gmail.com"
ENV TZ=Asia/Tehran
WORKDIR /app
RUN set -ex && apk upgrade --no-cache --scripts=no apk-tools && \
    apk add --no-cache --upgrade bash ca-certificates nftables su-exec && \
    addgroup -S -g 10001 sui && \
    adduser -S -u 10001 -G sui -h /app -s /sbin/nologin sui
COPY --from=backend-builder /app/sui /app/libcronet.so /app/
COPY entrypoint.sh /app/

# Asks the binary, which reads the port the operator actually configured. A
# check with the port written in here goes red the moment they change it in the
# panel, and an orchestrator then kills a container that was working.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD ["./sui", "healthcheck"]

# The container still starts as root by default: a TUN inbound needs
# CAP_NET_ADMIN in the process's permitted set, and a panel port below 1024
# needs CAP_NET_BIND_SERVICE. Flipping the default would break both, silently,
# on every existing deployment. Set SUI_UID (and optionally SUI_GID) to have
# entrypoint.sh hand over to an unprivileged user instead -- see entrypoint.sh.
ENTRYPOINT [ "./entrypoint.sh" ]
