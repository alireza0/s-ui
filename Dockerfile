FROM node:alpine as front-builder
WORKDIR /app
COPY frontend/ ./
RUN npm install && npm run build

FROM golang:1.21-alpine AS backend-builder
WORKDIR /app
ARG TARGETARCH
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
ENV CGO_ENABLED=1
RUN apk --no-cache --update add build-base gcc wget unzip
COPY backend/ ./
COPY --from=front-builder  /app/dist/ /app/web/html/
RUN go build -o sui main.go


FROM golang:1.22-alpine AS singbox-builder
WORKDIR /app
ARG SINGBOX_VER=v1.8.5
ARG SINGBOX_TAGS="with_grpc,with_quic,with_ech,with_reality_server,with_clash_api,with_v2ray_api"
ARG GOPROXY=""
ENV GOPROXY ${GOPROXY}
ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
RUN apk --no-cache --update add build-base gcc wget unzip git
RUN set -ex \
    && git clone --depth 1 --branch $SINGBOX_VER https://github.com/SagerNet/sing-box.git \
    && cd sing-box \
    && export COMMIT=$(git rev-parse --short HEAD) \
    && export VERSION=$(go run ./cmd/internal/read_tag) \
    && go build -v -trimpath -tags \
        $SINGBOX_TAGS \
        -ldflags "-X \"github.com/sagernet/sing-box/constant.Version=$VERSION\" -s -w -buildid=" \
        ./cmd/sing-box

FROM alpine
LABEL org.opencontainers.image.authors="alireza7@gmail.com"
ENV TZ=Asia/Tehran
WORKDIR /app
RUN apk add  --no-cache --update ca-certificates tzdata
COPY --from=backend-builder /app/sui /app/
COPY --from=singbox-builder /app/sing-box/sing-box /app/bin/
COPY runSingbox.sh /app/bin/
COPY docker-entrypoint.sh /app/
VOLUME [ "s-ui" ]
CMD [ "./docker-entrypoint.sh" ]
