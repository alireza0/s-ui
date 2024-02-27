FROM node:alpine as front-builder
WORKDIR /app
COPY frontend/ ./
RUN npm install && npm run build

FROM golang:1.22-alpine AS backend-builder
WORKDIR /app
ARG TARGETARCH
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
ENV CGO_ENABLED=1
RUN apk --no-cache --update add build-base gcc wget unzip
COPY backend/ ./
COPY --from=front-builder  /app/dist/ /app/web/html/
RUN go build -o sui main.go

FROM alpine
LABEL org.opencontainers.image.authors="alireza7@gmail.com"
ENV TZ=Asia/Tehran
WORKDIR /app
RUN apk add  --no-cache --update ca-certificates tzdata
COPY --from=backend-builder  /app/sui /app/
VOLUME [ "s-ui" ]
CMD [ "./sui" ]