#!/bin/sh
set -eu
. ./build-tags.sh

output=$1
mkdir -p "$output"
tags=$(tags_for agent)
ldflags=$(ldflags_for agent)
version=$(go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' github.com/sagernet/sing-box)
case "$version" in
    v[0-9]*) ;;
    *) echo "sing-box must be pinned to a version" >&2; exit 1 ;;
esac

for arch in amd64 arm64; do
    file="s-ui-node-linux-$arch"
    GOOS=linux GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -tags "$tags" -ldflags "$ldflags" -o "$output/$file" ./cmd/s-ui-node
    (cd "$output" && sha256sum "$file" > "$file.sha256")
done
read -r amd64 _ < "$output/s-ui-node-linux-amd64.sha256"
read -r arm64 _ < "$output/s-ui-node-linux-arm64.sha256"
printf '{"sing_box_version":"%s","agents":{"amd64":"%s","arm64":"%s"}}\n' "$version" "$amd64" "$arm64" > "$output/manifest.json"
