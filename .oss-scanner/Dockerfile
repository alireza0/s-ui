# Builds s-ui (Go backend + Vue frontend submodule) for the OSS Scanner.
# Network is available here and removed afterwards, so every Go module and npm
# package the tests need is fetched in this file.
FROM golang:1.27
ENV DEBIAN_FRONTEND=noninteractive \
    CGO_ENABLED=1 \
    GOFLAGS=-buildvcs=false
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates git curl python3 sqlite3 \
    && rm -rf /var/lib/apt/lists/*

# Node for the frontend, taken from the official image rather than Debian's older package.
COPY --from=node:26-slim /usr/local/bin/node /usr/local/bin/node
COPY --from=node:26-slim /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -s ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
    && ln -s ../lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx

# scanner contract: the checkout lives inside the image, at /src
COPY . /src
WORKDIR /src

# The scanner clones without submodules, and .dockerignore keeps .git out of the
# build context, so the submodule cannot be initialised here: clone the frontend's main.
RUN rm -rf frontend \
    && git clone --depth 1 https://github.com/alireza0/s-ui-frontend frontend

# Frontend: install, build, and place the bundle where web/web.go embeds it.
RUN cd frontend && npm ci && npm run build \
    && mkdir -p ../web/html && rm -rf ../web/html/* && cp -R dist/* ../web/html/

# Backend: the "test" profile from build-tags.sh has every protocol but the
# cronet ones, which need the Chromium toolchain. Keep debug info (no -w -s).
RUN go mod download
RUN . ./build-tags.sh \
    && go build -tags "$(tags_for test)" -ldflags "-checklinkname=0" -o sui main.go

# Run the tests so the build cache is warm offline, without failing the build on a red test.
RUN . ./build-tags.sh \
    && (go test -tags "$(tags_for test)" -count=1 ./... || echo "WARNING: go test reported failures")
RUN cd frontend && (npm test || echo "WARNING: frontend tests reported failures")
