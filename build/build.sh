#!/usr/bin/env bash
# Cross-compile the SmartEYE Windows executable (GUI subsystem, no console).
# Usage: build/build.sh [version]
set -euo pipefail

cd "$(dirname "$0")/.."
VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo dev)}"
OUT="dist"
mkdir -p "$OUT"

LDFLAGS="-s -w -H=windowsgui -X github.com/ozodmeofficial/smarteye/internal/meta.Version=${VERSION}"

echo "Building SmartEYE ${VERSION} for windows/amd64..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "${LDFLAGS}" -o "${OUT}/smarteye.exe" ./cmd/smarteye

echo "Built ${OUT}/smarteye.exe"
ls -la "${OUT}/smarteye.exe"
