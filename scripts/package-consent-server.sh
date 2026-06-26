#!/bin/bash

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

# package-consent-server.sh
# Download, configure, and stage the default consent server into distribution.
#
# Usage: ./scripts/package-consent-server.sh <GO_OS> <GO_ARCH> <DIST_OUTPUT_PATH>
#
# Arguments:
#   GO_OS            - Target OS in Go env format (linux, darwin, windows)
#   GO_ARCH          - Target architecture in Go env format (amd64, arm64)
#   DIST_OUTPUT_PATH - Absolute path to the distribution product folder
#                      (a 'consent' subdirectory will be created inside this)
#
# Exit codes:
#   0 - Success
#   1 - Failure

set -e

# Consent server release coordinates
CONSENT_SERVER_VERSION="0.3.0"
CONSENT_SERVER_DOWNLOAD_URL="https://github.com/wso2/openfgc/releases/download"
CONSENT_SERVER_PORT=9090

GO_OS="${1}"
GO_ARCH="${2}"
DIST_OUTPUT_PATH="${3}"

if [ -z "$GO_OS" ] || [ -z "$GO_ARCH" ] || [ -z "$DIST_OUTPUT_PATH" ]; then
    echo "Error: Missing required arguments."
    echo "Usage: $0 <GO_OS> <GO_ARCH> <DIST_OUTPUT_PATH>"
    exit 1
fi

# Map Go env OS/ARCH names to release artifact naming
PACKAGE_OS="$GO_OS"
PACKAGE_ARCH="$GO_ARCH"

if [ "$GO_OS" = "darwin" ]; then
    PACKAGE_OS="macos"
elif [ "$GO_OS" = "windows" ]; then
    PACKAGE_OS="win"
fi

if [ "$GO_ARCH" = "amd64" ]; then
    PACKAGE_ARCH="x64"
fi

ARCHIVE_NAME="consent-server-${CONSENT_SERVER_VERSION}-${PACKAGE_OS}-${PACKAGE_ARCH}.zip"
ARCHIVE_URL="${CONSENT_SERVER_DOWNLOAD_URL}/v${CONSENT_SERVER_VERSION}/${ARCHIVE_NAME}"
EXTRACTED_FOLDER="consent-server-${CONSENT_SERVER_VERSION}-${PACKAGE_OS}-${PACKAGE_ARCH}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "================================================================"
echo "Packaging consent server ${CONSENT_SERVER_VERSION} for ${PACKAGE_OS}/${PACKAGE_ARCH}..."
echo "Downloading from: $ARCHIVE_URL"
echo "================================================================"

if ! curl -fsSL "$ARCHIVE_URL" -o "$TMP_DIR/$ARCHIVE_NAME"; then
    echo "Error: Failed to download consent server from $ARCHIVE_URL"
    exit 1
fi

# Verify archive integrity using SHA256 checksum if available
CHECKSUM_URL="${ARCHIVE_URL}.sha256"
if curl -fsSL "$CHECKSUM_URL" -o "$TMP_DIR/${ARCHIVE_NAME}.sha256" 2>/dev/null; then
    echo "Verifying archive checksum..."
    EXPECTED_HASH=$(awk '{print $1}' "$TMP_DIR/${ARCHIVE_NAME}.sha256")
    ACTUAL_HASH=$(sha256sum "$TMP_DIR/$ARCHIVE_NAME" | awk '{print $1}')
    if [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
        echo "Error: Checksum verification failed for $ARCHIVE_NAME"
        echo "  Expected: $EXPECTED_HASH"
        echo "  Actual:   $ACTUAL_HASH"
        exit 1
    fi
    echo "Checksum verification passed."
else
    echo "Warning: No .sha256 checksum file found for $ARCHIVE_NAME, skipping verification."
fi

echo "Extracting consent server archive..."
(cd "$TMP_DIR" && unzip -q "$ARCHIVE_NAME")

WORK_DIR="$TMP_DIR/$EXTRACTED_FOLDER"
if [ ! -d "$WORK_DIR" ]; then
    echo "Error: Expected extracted directory '$EXTRACTED_FOLDER' not found in archive."
    exit 1
fi

echo "Initializing SQLite database..."
mkdir -p "$WORK_DIR/repository/database"
sqlite3 "$WORK_DIR/repository/database/consentdb.db" < "$WORK_DIR/dbscripts/db_schema_sqlite.sql"
sqlite3 "$WORK_DIR/repository/database/consentdb.db" "PRAGMA journal_mode=WAL;"

echo "Writing SQLite deployment configuration..."
cat > "$WORK_DIR/repository/conf/deployment.yaml" << EOF
server:
  hostname: localhost
  port: ${CONSENT_SERVER_PORT}
  readTimeout: 30s
  writeTimeout: 30s
  idleTimeout: 120s

database:
  consent:
    type: sqlite
    path: repository/database/consentdb.db
    options: "_pragma=journal_mode(WAL)&_pragma=cache_size(-16000)"

logging:
  level: info

consent:
  status_mappings:
    active_status: ACTIVE
    expired_status: EXPIRED
    revoked_status: REVOKED
    created_status: CREATED
    rejected_status: REJECTED
  auth_status_mappings:
    approved_state: APPROVED
    rejected_state: REJECTED
    created_state: CREATED
    system_expired_state: SYS_EXPIRED
    system_revoked_state: SYS_REVOKED
EOF

echo "Staging consent server into distribution..."
mkdir -p "$DIST_OUTPUT_PATH/consent"
cp -r "$WORK_DIR/." "$DIST_OUTPUT_PATH/consent/"

if [ "$GO_OS" != "windows" ]; then
    chmod +x "$DIST_OUTPUT_PATH/consent/consent-server" 2>/dev/null || true
    chmod +x "$DIST_OUTPUT_PATH/consent/start.sh" 2>/dev/null || true
fi

echo "================================================================"
echo "Consent server packaged successfully at: $DIST_OUTPUT_PATH/consent"
echo "================================================================"
