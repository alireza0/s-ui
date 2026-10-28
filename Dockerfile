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

# libcronet is pinned to an explicit cronet-go release and verified against
# a SHA-256 recorded here. It used to be fetched from releases/latest with
# no verification at all: whatever that URL served on build day was copied
# into the published image and loaded by a root daemon, and its version
# could drift ahead of the cronet-go module in go.mod. CRONET_VERSION
# matches the Chromium version of the cronet-go module go.mod currently
# builds against (150.x) -- bump the two together.
ARG CRONET_VERSION=v150.0.7871.63-2
ARG CRONET_SHA256_AMD64=c3949c6ad64e1d8fcd1e3b1fae4e302b2e553d769665a4bd7576483564c3f026
ARG CRONET_SHA256_386=46becdd1b89a2d73a003907f581278f4b29ae16f35161013dcbf2188bc22aaf4
ARG CRONET_SHA256_ARM64=8f13a6186aca498d37ee5e1f410282f587d663995aca60d6bf29a2d4f5536f2b
ARG CRONET_SHA256_ARM=566c9ec0a18a83e3f08d99b77e3437d6cf21e58a4ec3fed60b11975b3ee8ea97

RUN CRONET_ARCH="$TARGETARCH" && \
    case "$TARGETARCH" in \
      amd64) CRONET_SHA256="$CRONET_SHA256_AMD64" ;; \
      386) CRONET_SHA256="$CRONET_SHA256_386" ;; \
      arm64) CRONET_SHA256="$CRONET_SHA256_ARM64" ;; \
      arm) CRONET_SHA256="$CRONET_SHA256_ARM" ;; \
      *) echo "no pinned libcronet checksum for TARGETARCH=$TARGETARCH" >&2; exit 1 ;; \
    esac && \
    CRONET_URL="https://github.com/SagerNet/cronet-go/releases/download/${CRONET_VERSION}/libcronet-linux-${CRONET_ARCH}.so" && \
    echo "Downloading $CRONET_URL" && \
    wget -q -O ./libcronet.so "$CRONET_URL" && \
    echo "$CRONET_SHA256  ./libcronet.so" | sha256sum -c - && \
    chmod 755 ./libcronet.so

# Modules in their own layer, so a source-only change keeps them cached.
COPY go.mod go.sum ./
RUN go mod download

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
