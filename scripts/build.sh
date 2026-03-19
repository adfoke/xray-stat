#!/usr/bin/env sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
OUTPUT_DIR=${OUTPUT_DIR:-"$ROOT_DIR/dist"}
BINARY_NAME=${BINARY_NAME:-xray-stat}
GOCACHE_DIR=${GOCACHE_DIR:-"$ROOT_DIR/.cache/go-build"}
GOMODCACHE_DIR=${GOMODCACHE_DIR:-"$ROOT_DIR/.cache/go-mod"}
GOOS_VALUE=darwin
GOARCH_VALUE=arm64

mkdir -p "$OUTPUT_DIR"
mkdir -p "$GOCACHE_DIR"
mkdir -p "$GOMODCACHE_DIR"

OUTPUT_PATH="$OUTPUT_DIR/$BINARY_NAME"

echo "Building $OUTPUT_PATH"

CGO_ENABLED=0 \
GOOS="$GOOS_VALUE" \
GOARCH="$GOARCH_VALUE" \
GOCACHE="$GOCACHE_DIR" \
GOMODCACHE="$GOMODCACHE_DIR" \
go build \
	-trimpath \
	-buildvcs=false \
	-ldflags="-s -w -buildid=" \
	-o "$OUTPUT_PATH" \
	"$ROOT_DIR"

if command -v strip >/dev/null 2>&1; then
	strip -x "$OUTPUT_PATH" 2>/dev/null || true
fi

ls -lh "$OUTPUT_PATH"
