#!/usr/bin/env bash
set -euo pipefail

# xparse-cli cross-compile build script
# Usage: ./build.sh <version>
# Example: ./build.sh v1.0.1

if [ $# -lt 1 ]; then
  echo "Usage: $0 <version>"
  echo "Example: $0 v1.0.1"
  exit 1
fi

VERSION="$1"
GO_BIN="${GO_BIN:-$(command -v go || true)}"
if [ -z "$GO_BIN" ]; then
  echo "go executable not found; set GO_BIN to its absolute path."
  exit 1
fi

PKG="gitlab.intsig.net/xparse/xparse-client/cmd/parse"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CLI_DIR="${SCRIPT_DIR}"
DIST_DIR="${SCRIPT_DIR}/dist"

COMMIT=$(git -C "$SCRIPT_DIR" rev-parse --short HEAD)
DATE=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS="-s -w -X ${PKG}.version=${VERSION} -X ${PKG}.commit=${COMMIT} -X ${PKG}.date=${DATE}"

PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
  "windows/arm64"
)

mkdir -p "$DIST_DIR"

for platform in "${PLATFORMS[@]}"; do
  GOOS="${platform%/*}"
  GOARCH="${platform#*/}"
  OUTPUT="xparse-cli-${GOOS}-${GOARCH}"
  if [ "$GOOS" = "windows" ]; then
    OUTPUT="${OUTPUT}.exe"
  fi

  echo "Building ${OUTPUT} ..."
  cd "$CLI_DIR"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
    "$GO_BIN" build -ldflags "$LDFLAGS" -o "${DIST_DIR}/${OUTPUT}" .
done

echo ""
echo "Build complete: ${VERSION} (${COMMIT})"
echo "Output: ${DIST_DIR}/"
ls -lh "$DIST_DIR"/xparse-cli-*

# Verify the binary built for the current host.
echo ""
HOST_GOOS="$("$GO_BIN" env GOOS)"
HOST_GOARCH="$("$GO_BIN" env GOARCH)"
HOST_BINARY="${DIST_DIR}/xparse-cli-${HOST_GOOS}-${HOST_GOARCH}"
if [ "$HOST_GOOS" = "windows" ]; then
  HOST_BINARY="${HOST_BINARY}.exe"
fi
if [ -x "$HOST_BINARY" ]; then
  "$HOST_BINARY" version
else
  echo "Skipping executable verification: no runnable ${HOST_GOOS}/${HOST_GOARCH} artifact."
fi
