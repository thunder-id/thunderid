#!/bin/bash
set -euo pipefail
# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

# =============================================================================
# CLI Build Script
#
# Cross-compiles the CLI for all supported platforms.
# Output binaries are written to dist/ inside the cli directory.
# =============================================================================

PRODUCT_NAME="ThunderID"
PRODUCT_NAME_LOWERCASE="$(echo "$PRODUCT_NAME" | tr '[:upper:]' '[:lower:]')"

# Version reported by `--version`. Supplied by the npx build wrapper from
# package.json; falls back to the unreleased placeholder for direct local builds.
VERSION="${VERSION:-0.0.0-semantically-released}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_DIR="$SCRIPT_DIR/.."
DIST_DIR="$CLI_DIR/dist"

mkdir -p "$DIST_DIR"

# Format: "GOOS GOARCH output-name"
TARGETS=(
  "darwin  amd64 ${PRODUCT_NAME_LOWERCASE}-darwin-x64"
  "darwin  arm64 ${PRODUCT_NAME_LOWERCASE}-darwin-arm64"
  "linux   amd64 ${PRODUCT_NAME_LOWERCASE}-linux-x64"
  "linux   arm64 ${PRODUCT_NAME_LOWERCASE}-linux-arm64"
  "windows amd64 ${PRODUCT_NAME_LOWERCASE}-win-x64.exe"
)

cd "$CLI_DIR"

for entry in "${TARGETS[@]}"; do
  GOOS=$(echo "$entry" | awk '{print $1}')
  GOARCH=$(echo "$entry" | awk '{print $2}')
  OUT_NAME=$(echo "$entry" | awk '{print $3}')
  OUT="$DIST_DIR/$OUT_NAME"
  echo "Building $GOOS/$GOARCH → $OUT_NAME"
  GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=$VERSION" -o "$OUT" ./cmd/"${PRODUCT_NAME_LOWERCASE}"/
done

echo "Done. Binaries written to cli/dist/"
