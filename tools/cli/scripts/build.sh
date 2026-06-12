#!/bin/bash
set -euo pipefail
# ----------------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# ----------------------------------------------------------------------------

# =============================================================================
# CLI Build Script
#
# Cross-compiles the CLI for all supported platforms.
# Output binaries are written to dist/ inside the cli directory.
# =============================================================================

PRODUCT_NAME="ThunderID"
PRODUCT_NAME_LOWERCASE="$(echo "$PRODUCT_NAME" | tr '[:upper:]' '[:lower:]')"

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
  GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 go build -ldflags="-s -w" -o "$OUT" ./cmd/"${PRODUCT_NAME_LOWERCASE}"/
done

echo "Done. Binaries written to cli/dist/"
