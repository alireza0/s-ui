#!/usr/bin/env bash
# Test Docker multi-platform build (linux/amd64, 386, arm64, arm/v7, arm/v6)
# Requires: frontend_dist/ (run from repo root after building frontend)

set -e
cd "$(dirname "$0")/.."

echo "==> Preparing frontend_dist..."
if [ ! -d "frontend_dist" ] || [ -z "$(ls -A frontend_dist 2>/dev/null)" ]; then
  echo "Building frontend..."
  (cd frontend && npm install --prefer-offline --no-audit && npm run build)
  rm -rf frontend_dist
  mkdir -p frontend_dist
  cp -R frontend/dist/* frontend_dist/
  echo "frontend_dist ready."
else
  echo "frontend_dist exists, skipping frontend build."
fi

PLATFORMS="linux/amd64,linux/386,linux/arm64/v8,linux/arm/v7,linux/arm/v6"
echo "==> Testing Docker build for: $PLATFORMS"
docker buildx build \
  --platform "$PLATFORMS" \
  -f Dockerfile.frontend-artifact \
  --build-arg CRONET_RELEASE=latest \
  --progress=plain \
  . 2>&1 | tee docker-build-test.log

echo "==> Done. Check docker-build-test.log for full output."
