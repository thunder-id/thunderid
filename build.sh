#!/bin/bash
# ----------------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Check if --without-consent is passed and remove it from args
WITHOUT_CONSENT=${WITHOUT_CONSENT:-false}
NEW_ARGS=()
for arg in "$@"; do
    if [ "$arg" = "--without-consent" ]; then
        WITHOUT_CONSENT="true"
    else
        NEW_ARGS+=("$arg")
    fi
done
set -- "${NEW_ARGS[@]}"

# --- Set Default OS and the architecture --- 
# Auto-detect GO OS
DEFAULT_OS=$(go env GOOS 2>/dev/null)
if [ -z "$DEFAULT_OS" ]; then
  UNAME_OS="$(uname -s)"
  case "$UNAME_OS" in
    Darwin) DEFAULT_OS="darwin" ;;
    Linux) DEFAULT_OS="linux" ;;
    MINGW*|MSYS*|CYGWIN*) DEFAULT_OS="windows" ;;
    *) echo "Unsupported OS: $UNAME_OS"; exit 1 ;;
  esac
fi
# Auto-detect GO ARCH
DEFAULT_ARCH=$(go env GOARCH 2>/dev/null)
if [ -z "$DEFAULT_ARCH" ]; then
  UNAME_ARCH="$(uname -m)"
  case "$UNAME_ARCH" in
    x86_64|amd64) DEFAULT_ARCH="amd64" ;;
    arm64|aarch64) DEFAULT_ARCH="arm64" ;;
    *) echo "Unsupported architecture: $UNAME_ARCH"; exit 1 ;;
  esac
fi

GO_OS=${2:-$DEFAULT_OS}
GO_ARCH=${3:-$DEFAULT_ARCH}

echo "Using GO OS: $GO_OS and ARCH: $GO_ARCH"

# --- Package Distribution details ---
GO_PACKAGE_OS=$GO_OS
GO_PACKAGE_ARCH=$GO_ARCH

# Normalize OS name for distribution packaging
if [ "$GO_OS" = "darwin" ]; then
    GO_PACKAGE_OS=macos
elif [ "$GO_OS" = "windows" ]; then
    GO_PACKAGE_OS="win"
fi

if [ "$GO_ARCH" = "amd64" ]; then
    GO_PACKAGE_ARCH=x64
fi

VERSION_FILE=version.txt
VERSION=$(cat "$VERSION_FILE")
PRODUCT_VERSION=${VERSION}
if [[ $PRODUCT_VERSION == v* ]]; then
  PRODUCT_VERSION="${PRODUCT_VERSION#v}"
fi
PRODUCT_NAME="ThunderID"
PRODUCT_NAME_LOWERCASE="$(echo "$PRODUCT_NAME" | tr '[:upper:]' '[:lower:]')"
BINARY_NAME="${PRODUCT_NAME_LOWERCASE}"
PRODUCT_FOLDER=${BINARY_NAME}-${PRODUCT_VERSION}-${GO_PACKAGE_OS}-${GO_PACKAGE_ARCH}

# --- Sample App Distribution details ---
# React Vanilla Sample
VANILLA_SAMPLE_APP_VERSION=$(grep -o '"version": *"[^"]*"' samples/apps/react-vanilla-sample/package.json | sed 's/"version": *"\(.*\)"/\1/')
VANILLA_SAMPLE_APP_FOLDER="sample-app-react-vanilla-${VANILLA_SAMPLE_APP_VERSION}"

# React SDK Sample
REACT_SDK_SAMPLE_APP_VERSION=$(grep -o '"version": *"[^"]*"' samples/apps/react-sdk-sample/package.json | sed 's/"version": *"\(.*\)"/\1/')
REACT_SDK_SAMPLE_APP_FOLDER="sample-app-react-sdk-${REACT_SDK_SAMPLE_APP_VERSION}"

# React API-based Sample
REACT_API_SAMPLE_APP_VERSION=$(grep -o '"version": *"[^"]*"' samples/apps/react-api-based-sample/package.json | sed 's/"version": *"\(.*\)"/\1/')
REACT_API_SAMPLE_APP_FOLDER="sample-app-react-api-based-${REACT_API_SAMPLE_APP_VERSION}"

# Wayfinder Sample
WAYFINDER_SAMPLE_APP_VERSION=$(grep -o '"version": *"[^"]*"' samples/apps/wayfinder-sample/package.json | sed 's/"version": *"\(.*\)"/\1/')
WAYFINDER_SAMPLE_APP_FOLDER="sample-app-wayfinder-${WAYFINDER_SAMPLE_APP_VERSION}"



# Directories
TARGET_DIR=target
OUTPUT_DIR=$TARGET_DIR/out
DIST_DIR=$TARGET_DIR/dist
BUILD_DIR=$OUTPUT_DIR/.build
LOCAL_CERT_DIR=$OUTPUT_DIR/.cert
BACKEND_BASE_DIR=backend
BACKEND_DIR=$BACKEND_BASE_DIR/cmd/server
REPOSITORY_DIR=$BACKEND_BASE_DIR/cmd/server/repository
REPOSITORY_DB_DIR=$REPOSITORY_DIR/database
SERVER_SCRIPTS_DIR=$BACKEND_BASE_DIR/scripts
SERVER_DB_SCRIPTS_DIR=$BACKEND_BASE_DIR/dbscripts
SECURITY_DIR=repository/resources/security
FRONTEND_BASE_DIR=frontend
GATE_APP_DIST_DIR=apps/gate
CONSOLE_APP_DIST_DIR=apps/console
FRONTEND_GATE_APP_SOURCE_DIR=$FRONTEND_BASE_DIR/apps/gate
FRONTEND_CONSOLE_APP_SOURCE_DIR=$FRONTEND_BASE_DIR/apps/console
SAMPLE_BASE_DIR=samples
VANILLA_SAMPLE_APP_DIR=$SAMPLE_BASE_DIR/apps/react-vanilla-sample
VANILLA_SAMPLE_APP_SERVER_DIR=$VANILLA_SAMPLE_APP_DIR/server
REACT_SDK_SAMPLE_APP_DIR=$SAMPLE_BASE_DIR/apps/react-sdk-sample
REACT_API_SAMPLE_APP_DIR=$SAMPLE_BASE_DIR/apps/react-api-based-sample
WAYFINDER_SAMPLE_APP_DIR=$SAMPLE_BASE_DIR/apps/wayfinder-sample


# Quick start declarative bundles staged into the console's welcome feature so they're inlined
# into the console JS bundle at build time.
# Add a new bundle as a single line: "<dest-name>:<source-dir>".
# <dest-name> may include "/" for grouping (e.g. "wayfinder/redirect-based").
QUICKSTART_SAMPLE_BUNDLES=(
    "wayfinder:$WAYFINDER_SAMPLE_APP_DIR/thunderid-config"
)
QUICKSTART_BUNDLE_STAGE_DIR="$FRONTEND_CONSOLE_APP_SOURCE_DIR/src/features/welcome/data/sample-bundles"

# Default ports
GATE_APP_DEFAULT_PORT=5190
CONSOLE_APP_DEFAULT_PORT=5191
DOCS_DEFAULT_PORT=3000

# Integration test filters (optional)
TEST_RUN="${4:-}"
TEST_PACKAGE="${5:-}"

# PNPM version to use for frontend builds and docs build
PNPM_VERSION="11.0.9"

# ============================================================================
# Read Configuration from deployment.yaml
# ============================================================================

CONFIG_FILE="./backend/cmd/server/repository/conf/deployment.yaml"

# Function to read config with fallback
read_config() {
    local config_file="$CONFIG_FILE"

    if [ ! -f "$config_file" ]; then
        # Use defaults if config file not found
        HOSTNAME="localhost"
        PORT=8090
        HTTP_ONLY="false"
        PUBLIC_HOSTNAME=""
        CONSENT_ENABLED="true"
    else
        # Try yq first (YAML parser)
        if command -v yq >/dev/null 2>&1; then
            HOSTNAME=$(yq eval '.server.hostname // "localhost"' "$config_file" 2>/dev/null)
            PORT=$(yq eval '.server.port // 8090' "$config_file" 2>/dev/null)
            HTTP_ONLY=$(yq eval '.server.http_only // false' "$config_file" 2>/dev/null)
            PUBLIC_HOSTNAME=$(yq eval '.server.public_hostname // ""' "$config_file" 2>/dev/null)
            CONSENT_ENABLED=$(yq eval '.consent.enabled // true' "$config_file" 2>/dev/null)
        else
            # Fallback: basic parsing with grep/awk
            HOSTNAME=$(grep -E '^\s*hostname:' "$config_file" | awk -F':' '{gsub(/[[:space:]"'\'']/,"",$2); print $2}' | head -1)
            PORT=$(grep -E '^\s*port:' "$config_file" | awk -F':' '{gsub(/[[:space:]]/,"",$2); print $2}' | head -1)
            PUBLIC_HOSTNAME=$(grep -E '^\s*public_hostname:' "$config_file" | grep -o '"[^"]*"' | tr -d '"' | head -1)

            # Check for http_only
            if grep -q 'http_only.*true' "$config_file" 2>/dev/null; then
                HTTP_ONLY="true"
            else
                HTTP_ONLY="false"
            fi

            # Check for consent.enabled (default: true)
            if grep -A1 '^consent:' "$config_file" 2>/dev/null | grep -q 'enabled.*false'; then
                CONSENT_ENABLED="false"
            else
                CONSENT_ENABLED="true"
            fi

            # Use defaults if not found
            HOSTNAME=${HOSTNAME:-localhost}
            PORT=${PORT:-8090}
        fi

    fi

    # Determine protocol
    if [ "$HTTP_ONLY" = "true" ]; then
        PROTOCOL="http"
    else
        PROTOCOL="https"
    fi
    return 0
}

# Read configuration
read_config

# Construct base URL (internal API endpoint)
BASE_URL="${PROTOCOL}://${HOSTNAME}:${PORT}"

# Construct public URL (external/redirect URLs)
if [ -n "$PUBLIC_HOSTNAME" ]; then
    PUBLIC_URL="$PUBLIC_HOSTNAME"
else
    PUBLIC_URL="$BASE_URL"
fi

function get_coverage_exclusion_pattern() {
    # Read exclusion patterns (full package paths) from .excludecoverage file
    local coverage_exclude_file
    
    # Check if we're already in the backend directory or need to use relative path
    if [ -f ".excludecoverage" ]; then
        coverage_exclude_file=".excludecoverage"
    elif [ -f "$SCRIPT_DIR/$BACKEND_BASE_DIR/.excludecoverage" ]; then
        coverage_exclude_file="$SCRIPT_DIR/$BACKEND_BASE_DIR/.excludecoverage"
    else
        echo "" >&2
        return
    fi
    
    # Read non-comment, non-empty lines and join with '|' for grep (exact package path matching)
    local pattern=$(awk '!/^#/ && NF {if(count++)printf "|"; printf "%s", $0}' "$coverage_exclude_file")
    echo "$pattern"
}

function clean() {
    echo "================================================================"
    echo "Cleaning build artifacts..."
    rm -rf "$TARGET_DIR"

    echo "Removing certificates in the $BACKEND_DIR/$SECURITY_DIR"
    rm -rf "$BACKEND_DIR/$SECURITY_DIR"

    echo "Removing certificates in the $VANILLA_SAMPLE_APP_DIR"
    rm -f "$VANILLA_SAMPLE_APP_DIR/server.cert"
    rm -f "$VANILLA_SAMPLE_APP_DIR/server.key"

    echo "Removing certificates in the $VANILLA_SAMPLE_APP_SERVER_DIR"
    rm -f "$VANILLA_SAMPLE_APP_SERVER_DIR/server.cert"
    rm -f "$VANILLA_SAMPLE_APP_SERVER_DIR/server.key"

    echo "Removing certificates in the $REACT_SDK_SAMPLE_APP_DIR"
    rm -f "$REACT_SDK_SAMPLE_APP_DIR/server.cert"
    rm -f "$REACT_SDK_SAMPLE_APP_DIR/server.key"

    echo "Removing certificates in the $REACT_API_SAMPLE_APP_DIR"
    rm -f "$REACT_API_SAMPLE_APP_DIR/server.cert"
    rm -f "$REACT_API_SAMPLE_APP_DIR/server.key"
    echo "================================================================"
}

function build_backend() {
    echo "================================================================"
    echo "Building Go backend..."
    mkdir -p "$BUILD_DIR"

    # Set binary name with .exe extension for Windows
    local output_binary="$BINARY_NAME"
    if [ "$GO_OS" = "windows" ]; then
        output_binary="${BINARY_NAME}.exe"
    fi

    # Check if coverage build is requested via ENABLE_COVERAGE environment variable
    local build_flags="-x"
    if [ "$ENABLE_COVERAGE" = "true" ]; then
        echo "Building with coverage instrumentation enabled..."
        # Build coverage package list
        cd "$BACKEND_BASE_DIR" || exit 1
        local exclude_pattern=$(get_coverage_exclusion_pattern)
        local coverpkg
        if [ -n "$exclude_pattern" ]; then
            echo "Excluding coverage for patterns: $exclude_pattern"
            coverpkg=$(go list ./... | grep -v -E "$exclude_pattern" | tr '\n' ',' | sed 's/,$//')
        else
            coverpkg=$(go list ./... | tr '\n' ',' | sed 's/,$//')
        fi
        cd "$SCRIPT_DIR" || exit 1
        build_flags="$build_flags -cover -coverpkg=$coverpkg"
    fi

    GOOS=$GO_OS GOARCH=$GO_ARCH CGO_ENABLED=0 go build -C "$BACKEND_BASE_DIR" \
    $build_flags -ldflags "-X \"main.version=$VERSION\" \
    -X \"main.buildDate=$$(date -u '+%Y-%m-%d %H:%M:%S UTC')\"" \
    -o "../$BUILD_DIR/$output_binary" ./cmd/server

    echo "Initializing databases..."
    initialize_databases true
    echo "================================================================"
}

function initialize_databases() {
    echo "================================================================"
    local override=$1
    if [[ -z "$override" ]]; then
        override=false
    fi

    echo "Initializing SQLite databases..."

    mkdir -p "$REPOSITORY_DB_DIR"

    db_files=("configdb.db" "runtimedb.db" "userdb.db")
    script_paths=("configdb/sqlite.sql" "runtimedb/sqlite.sql" "userdb/sqlite.sql")

    for ((i = 0; i < ${#db_files[@]}; i++)); do
        db_file="${db_files[$i]}"
        script_rel_path="${script_paths[$i]}"
        db_path="$REPOSITORY_DB_DIR/$db_file"
        script_path="$SERVER_DB_SCRIPTS_DIR/$script_rel_path"

        if [[ -f "$script_path" ]]; then
            if [[ -f "$db_path" ]]; then
                if $override; then
                    echo " - Removing existing $db_file as override is true"
                    rm "$db_path"
                else
                    echo " ! Skipping $db_file: DB already exists. Delete the existing and re-run to recreate."
                    continue
                fi
            fi

            echo " - Creating $db_file using $script_path"
            sqlite3 "$db_path" < "$script_path"
            sqlite3 "$db_path" "PRAGMA journal_mode=WAL;"
            if [ $? -ne 0 ]; then
                echo "Failed to enable WAL mode for $db_file"
                exit 1
            fi
        else
            echo " ! Skipping $db_file: SQL script not found at $script_path"
        fi
    done

    echo "SQLite database initialization complete."
    echo "================================================================"
}

function ensure_pnpm() {
    if ! command -v pnpm >/dev/null 2>&1; then
        echo "pnpm not found, installing..."
        npm install -g pnpm@$PNPM_VERSION
    fi
}

function build_frontend() {
    echo "================================================================"
    echo "Building frontend apps..."
    ensure_pnpm

    sync_quickstart_bundles

    # Install dependencies
    echo "Installing frontend dependencies..."
    pnpm install --frozen-lockfile

    echo "Building frontend applications & packages..."
    pnpm build:frontend
    
    # Return to script directory
    cd "$SCRIPT_DIR" || exit 1
    echo "================================================================"
}

function build_sdks_js() {
    ensure_pnpm
    
    echo "Installing SDK dependencies..."
    pnpm install --frozen-lockfile
    
    echo "Building JavaScript ecosystem SDK packages..."
    pnpm --filter './sdks/**' build
    cd "$SCRIPT_DIR" || exit 1
}

function test_sdks_js() {
    ensure_pnpm
    
    echo "Installing SDK dependencies..."
    pnpm install --frozen-lockfile
    
    echo "Running JavaScript ecosystem SDK tests..."
    pnpm --filter './sdks/**' test
    cd "$SCRIPT_DIR" || exit 1
}

function lint_sdks_js() {
    ensure_pnpm
    
    echo "Installing SDK dependencies..."
    pnpm install --frozen-lockfile
    
    echo "Linting JavaScript ecosystem SDK packages..."
    pnpm --filter './sdks/**' lint
    cd "$SCRIPT_DIR" || exit 1
}

function build_sdks() {
    echo "================================================================"
    echo "Building SDKs..."
    build_sdks_js
    echo "================================================================"
}

function test_sdks() {
    echo "================================================================"
    echo "Running SDK tests..."
    test_sdks_js
    echo "================================================================"
}

function lint_sdks() {
    echo "================================================================"
    echo "Linting SDKs..."
    lint_sdks_js
    echo "================================================================"
}

function build_docs() {
    echo "================================================================"
    echo "Building documentation..."
    ensure_pnpm
    
    echo "Installing frontend dependencies (required for docs build)..."
    pnpm install --frozen-lockfile
    
    echo "Building documentation..."
    pnpm run build:docs
    
    # Return to script directory
    cd "$SCRIPT_DIR" || exit 1
    echo "================================================================"
}

function prepare_backend_for_packaging() {
    echo "================================================================"
    echo "Copying backend artifacts..."

    # Use appropriate binary name based on OS
    local binary_name="$BINARY_NAME"
    if [ "$GO_OS" = "windows" ]; then
        binary_name="${BINARY_NAME}.exe"
    fi

    cp "$BUILD_DIR/$binary_name" "$DIST_DIR/$PRODUCT_FOLDER/"
    cp -r "$REPOSITORY_DIR" "$DIST_DIR/$PRODUCT_FOLDER/"
    cp "$VERSION_FILE" "$DIST_DIR/$PRODUCT_FOLDER/"
    cp -r "$SERVER_SCRIPTS_DIR" "$DIST_DIR/$PRODUCT_FOLDER/"
    cp -r "$SERVER_DB_SCRIPTS_DIR" "$DIST_DIR/$PRODUCT_FOLDER/"
    mkdir -p "$DIST_DIR/$PRODUCT_FOLDER/$SECURITY_DIR"

    # Copy bootstrap directory
    echo "Copying bootstrap scripts..."
    cp -r "$BACKEND_DIR/bootstrap" "$DIST_DIR/$PRODUCT_FOLDER/"
    # Ensure execute permissions on bootstrap scripts
    chmod +x "$DIST_DIR/$PRODUCT_FOLDER/bootstrap/"*.sh 2>/dev/null || true

    echo "=== Ensuring server certificates exist in the distribution ==="
    ensure_certificates "$DIST_DIR/$PRODUCT_FOLDER/$SECURITY_DIR" "server"
    ensure_certificates "$DIST_DIR/$PRODUCT_FOLDER/$SECURITY_DIR" "signing"
    echo "================================================================"

    echo "=== Ensuring crypto file exists in the distribution ==="
    ensure_crypto_file "$DIST_DIR/$PRODUCT_FOLDER/$SECURITY_DIR"
    echo "================================================================"
}

function prepare_frontend_for_packaging() {
    echo "================================================================"
    echo "Copying frontend artifacts..."

    mkdir -p "$DIST_DIR/$PRODUCT_FOLDER/$GATE_APP_DIST_DIR"
    mkdir -p "$DIST_DIR/$PRODUCT_FOLDER/$CONSOLE_APP_DIST_DIR"

    # Copy gate app build output
    if [ -d "$FRONTEND_GATE_APP_SOURCE_DIR/dist" ]; then
        echo "Copying Gate app build output..."
        shopt -s dotglob
        cp -r "$FRONTEND_GATE_APP_SOURCE_DIR/dist/"* "$DIST_DIR/$PRODUCT_FOLDER/$GATE_APP_DIST_DIR"
        shopt -u dotglob
    else
        echo "Warning: Gate app build output not found at $FRONTEND_GATE_APP_SOURCE_DIR/dist"
    fi
    
    # Copy console app build output
    if [ -d "$FRONTEND_CONSOLE_APP_SOURCE_DIR/dist" ]; then
        echo "Copying Console app build output..."
        shopt -s dotglob
        cp -r "$FRONTEND_CONSOLE_APP_SOURCE_DIR/dist/"* "$DIST_DIR/$PRODUCT_FOLDER/$CONSOLE_APP_DIST_DIR"
        shopt -u dotglob
    else
        echo "Warning: Console app build output not found at $FRONTEND_CONSOLE_APP_SOURCE_DIR/dist"
    fi

    echo "================================================================"
}

function sync_quickstart_bundles() {
    # Stage Quick start declarative bundles into the console's public dir.
    echo "Syncing quick start sample bundles to console welcome data dir..."
    rm -rf "$QUICKSTART_BUNDLE_STAGE_DIR"
    for entry in "${QUICKSTART_SAMPLE_BUNDLES[@]}"; do
        local dest_name="${entry%%:*}"
        local src_dir="${entry#*:}"
        local dest_dir="$QUICKSTART_BUNDLE_STAGE_DIR/$dest_name"
        if [ -d "$src_dir" ]; then
            echo "  Staging '$dest_name' from $src_dir"
            mkdir -p "$dest_dir"
            shopt -s dotglob
            cp -r "$src_dir/"* "$dest_dir"
            shopt -u dotglob
        else
            echo "  Warning: quick start bundle source not found at $src_dir (dest '$dest_name')"
        fi
    done
}

function package() {
    echo "================================================================"
    echo "Packaging backend & frontend artifacts..."

    mkdir -p "$DIST_DIR/$PRODUCT_FOLDER"

    prepare_frontend_for_packaging
    prepare_backend_for_packaging

    # Copy the appropriate startup and setup scripts based on the target OS
    if [ "$GO_OS" = "windows" ]; then
        echo "Including Windows scripts (start.ps1, setup.ps1)..."
        cp -r "start.ps1" "$DIST_DIR/$PRODUCT_FOLDER"
        cp -r "setup.ps1" "$DIST_DIR/$PRODUCT_FOLDER"
    else
        echo "Including Unix scripts (start.sh, setup.sh)..."
        cp -r "start.sh" "$DIST_DIR/$PRODUCT_FOLDER"
        cp -r "setup.sh" "$DIST_DIR/$PRODUCT_FOLDER"
        # Ensure execute permissions on Unix scripts
        chmod +x "$DIST_DIR/$PRODUCT_FOLDER/start.sh"
        chmod +x "$DIST_DIR/$PRODUCT_FOLDER/setup.sh"
    fi

    if [ "$WITHOUT_CONSENT" != "true" ]; then
        echo "Packaging consent server..."
        bash "$SCRIPT_DIR/scripts/package-consent-server.sh" \
                "$GO_OS" "$GO_ARCH" "$(cd "$DIST_DIR/$PRODUCT_FOLDER" && pwd)"
    else
        echo "Skipping consent server packaging (--without-consent)..."
        local target_yaml="$DIST_DIR/$PRODUCT_FOLDER/repository/conf/deployment.yaml"
        if command -v yq >/dev/null 2>&1; then
            yq eval '.consent.enabled = false' -i "$target_yaml" 2>/dev/null || sed -i.bak '/^consent:/ { n; s/enabled: true/enabled: false/; }' "$target_yaml" || true
        else
            sed -i.bak '/^consent:/ { n; s/enabled: true/enabled: false/; }' "$target_yaml" || true
        fi
        rm -f "${target_yaml}.bak" 2>/dev/null || true
    fi

    echo "Creating zip file..."
    (cd "$DIST_DIR" && find "$PRODUCT_FOLDER" | sort | zip "$PRODUCT_FOLDER.zip" -@)
    rm -rf "${DIST_DIR:?}/$PRODUCT_FOLDER" "$BUILD_DIR"
    echo "================================================================"
}

function build_sample_app() {
    echo "================================================================"
    echo "Building sample apps..."

    # Build React Vanilla sample
    echo "=== Building React Vanilla sample app ==="
    echo "=== Ensuring React Vanilla sample app certificates exist ==="
    ensure_certificates "$VANILLA_SAMPLE_APP_DIR" "server"

    cd "$VANILLA_SAMPLE_APP_DIR" || exit 1
    echo "Installing React Vanilla sample dependencies..."
    pnpm install --frozen-lockfile

    echo "Building React Vanilla sample app..."
    pnpm run build

    cd - || exit 1
    echo "✅ React Vanilla sample app built successfully."

    # Build React SDK sample
    echo "=== Building React SDK sample app ==="

    # Ensure certificates exist for React SDK sample
    echo "=== Ensuring React SDK sample app certificates exist ==="
    ensure_certificates "$REACT_SDK_SAMPLE_APP_DIR" "server"

    cd "$REACT_SDK_SAMPLE_APP_DIR" || exit 1
    echo "Installing React SDK sample dependencies..."
    pnpm install

    echo "Building React SDK sample app..."
    pnpm run build

    cd - || exit 1
    echo "✅ React SDK sample app built successfully."

    # Build React API-based sample
    echo "=== Building React API-based sample app ==="

    # Ensure certificates exist for React API-based sample
    echo "=== Ensuring React API-based sample app certificates exist ==="
    ensure_certificates "$REACT_API_SAMPLE_APP_DIR" "server"

    cd "$REACT_API_SAMPLE_APP_DIR" || exit 1
    echo "Installing React API-based sample dependencies..."
    pnpm install --frozen-lockfile

    echo "Building React API-based sample app..."
    pnpm run build

    cd - || exit 1
    echo "✅ React API-based sample app built successfully."

    # Build Wayfinder sample (Wayfinder)
    echo "=== Building Wayfinder sample app ==="

    cd "$WAYFINDER_SAMPLE_APP_DIR" || exit 1
    echo "Installing Wayfinder sample dependencies..."
    npm install

    echo "Building Wayfinder sample frontend..."
    (cd frontend && npm run build)

    cd "$SCRIPT_DIR" || exit 1

    echo "✅ Wayfinder sample app built successfully."

    echo "================================================================"
}

function package_sample_app() {
    echo "================================================================"
    echo "Packaging sample apps..."

    # Package React Vanilla sample
    echo "=== Packaging React Vanilla sample app ==="
    package_vanilla_sample

    # Package React SDK sample
    echo "=== Packaging React SDK sample app ==="
    package_react_sdk_sample

    # Package React API-based sample
    echo "=== Packaging React API-based sample app ==="
    package_react_api_based_sample

    # Package Wayfinder sample
    echo "=== Packaging Wayfinder sample app ==="
    package_wayfinder_sample

    echo "================================================================"
}

function package_vanilla_sample() {
    local tgz

    cd "$VANILLA_SAMPLE_APP_DIR" || exit 1
    pnpm pack --pack-destination "$SCRIPT_DIR/$DIST_DIR"
    cd "$SCRIPT_DIR" || exit 1

    tgz=$(ls "$DIST_DIR"/thunderid-react-vanilla-sample-*.tgz 2>/dev/null | head -1)
    if [ -z "$tgz" ]; then
        echo "Error: pnpm pack did not produce a tgz for react-vanilla-sample"
        exit 1
    fi

    tar xzf "$tgz" -C "$DIST_DIR"
    mv "$DIST_DIR/package" "$DIST_DIR/$VANILLA_SAMPLE_APP_FOLDER"
    (cd "$DIST_DIR" && find "$VANILLA_SAMPLE_APP_FOLDER" | sort | zip "$VANILLA_SAMPLE_APP_FOLDER.zip" -@)
    rm -rf "${DIST_DIR:?}/$VANILLA_SAMPLE_APP_FOLDER" "$tgz"

    echo "✅ React Vanilla sample app packaged successfully as $DIST_DIR/$VANILLA_SAMPLE_APP_FOLDER.zip"
}

function package_react_sdk_sample() {
    local tgz

    cd "$REACT_SDK_SAMPLE_APP_DIR" || exit 1
    pnpm pack --pack-destination "$SCRIPT_DIR/$DIST_DIR"
    cd "$SCRIPT_DIR" || exit 1

    tgz=$(ls "$DIST_DIR"/thunderid-react-sdk-sample-*.tgz 2>/dev/null | head -1)
    if [ -z "$tgz" ]; then
        echo "Error: pnpm pack did not produce a tgz for react-sdk-sample"
        exit 1
    fi

    tar xzf "$tgz" -C "$DIST_DIR"
    mv "$DIST_DIR/package" "$DIST_DIR/$REACT_SDK_SAMPLE_APP_FOLDER"
    (cd "$DIST_DIR" && find "$REACT_SDK_SAMPLE_APP_FOLDER" | sort | zip "$REACT_SDK_SAMPLE_APP_FOLDER.zip" -@)
    rm -rf "${DIST_DIR:?}/$REACT_SDK_SAMPLE_APP_FOLDER" "$tgz"

    echo "✅ React SDK sample app packaged successfully as $DIST_DIR/$REACT_SDK_SAMPLE_APP_FOLDER.zip"
}

function package_react_api_based_sample() {
    local tgz

    cd "$REACT_API_SAMPLE_APP_DIR" || exit 1
    pnpm pack --pack-destination "$SCRIPT_DIR/$DIST_DIR"
    cd "$SCRIPT_DIR" || exit 1

    tgz=$(ls "$DIST_DIR"/thunderid-react-api-based-sample-*.tgz 2>/dev/null | head -1)
    if [ -z "$tgz" ]; then
        echo "Error: pnpm pack did not produce a tgz for react-api-based-sample"
        exit 1
    fi

    tar xzf "$tgz" -C "$DIST_DIR"
    mv "$DIST_DIR/package" "$DIST_DIR/$REACT_API_SAMPLE_APP_FOLDER"
    (cd "$DIST_DIR" && find "$REACT_API_SAMPLE_APP_FOLDER" | sort | zip "$REACT_API_SAMPLE_APP_FOLDER.zip" -@)
    rm -rf "${DIST_DIR:?}/$REACT_API_SAMPLE_APP_FOLDER" "$tgz"

    echo "✅ React API-based sample app packaged successfully as $DIST_DIR/$REACT_API_SAMPLE_APP_FOLDER.zip"
}

function package_wayfinder_sample() {
    local tgz

    cd "$WAYFINDER_SAMPLE_APP_DIR" || exit 1
    pnpm pack --pack-destination "$SCRIPT_DIR/$DIST_DIR"
    cd "$SCRIPT_DIR" || exit 1

    tgz=$(ls "$DIST_DIR"/thunderid-wayfinder-sample-*.tgz 2>/dev/null | head -1)
    if [ -z "$tgz" ]; then
        echo "Error: pnpm pack did not produce a tgz for wayfinder-sample"
        exit 1
    fi

    tar xzf "$tgz" -C "$DIST_DIR"
    mv "$DIST_DIR/package" "$DIST_DIR/$WAYFINDER_SAMPLE_APP_FOLDER"

    for dir in frontend backend smtp-server ai-agent; do
        if [ -f "$DIST_DIR/$WAYFINDER_SAMPLE_APP_FOLDER/$dir/.env.example" ]; then
            cp "$DIST_DIR/$WAYFINDER_SAMPLE_APP_FOLDER/$dir/.env.example" "$DIST_DIR/$WAYFINDER_SAMPLE_APP_FOLDER/$dir/.env"
        fi
    done

    (cd "$DIST_DIR" && find "$WAYFINDER_SAMPLE_APP_FOLDER" | sort | zip "$WAYFINDER_SAMPLE_APP_FOLDER.zip" -@)
    rm -rf "${DIST_DIR:?}/$WAYFINDER_SAMPLE_APP_FOLDER" "$tgz"

    echo "✅ Wayfinder sample app packaged successfully as $DIST_DIR/$WAYFINDER_SAMPLE_APP_FOLDER.zip"
}

function test_unit() {
    echo "================================================================"
    echo "Running unit tests with coverage..."
    cd "$BACKEND_BASE_DIR" || exit 1
    
    # Build coverage package list
    local exclude_pattern=$(get_coverage_exclusion_pattern)
    local coverpkg
    if [ -n "$exclude_pattern" ]; then
        echo "Excluding coverage for patterns: $exclude_pattern"
        coverpkg=$(go list ./... | grep -v -E "$exclude_pattern" | tr '\n' ',' | sed 's/,$//')
    else
        echo "No exclusion patterns found, including all packages"
        coverpkg=$(go list ./... | tr '\n' ',' | sed 's/,$//')
    fi
    
    # Run gotestsum if available, otherwise fallback to go test
    if command -v gotestsum &> /dev/null; then
        echo "Running unit tests with coverage using gotestsum..."
        gotestsum -- -v -count=1 -coverprofile=coverage_unit.out -covermode=atomic -coverpkg="$coverpkg" ./... || { echo "There are unit test failures."; exit 1; }
    else
        echo "Running unit tests with coverage using go test..."
        go test -v -count=1 -coverprofile=coverage_unit.out -covermode=atomic -coverpkg="$coverpkg" ./... || { echo "There are unit test failures."; exit 1; }
    fi
    
    echo "Unit test coverage profile generated in: backend/coverage_unit.out"
    
    # Generate HTML coverage report for unit tests
    go tool cover -html=coverage_unit.out -o=coverage_unit.html
    echo "Unit test coverage HTML report generated in: backend/coverage_unit.html"
    
    # Display unit test coverage summary
    echo ""
    echo "================================================================"
    echo "Unit Test Coverage Summary:"
    go tool cover -func=coverage_unit.out | tail -n 1
    echo "================================================================"
    echo ""
    
    cd "$SCRIPT_DIR" || exit 1
    echo "================================================================"
}

function test_integration() {
    echo "================================================================"
    echo "Running integration tests..."
    cd "$SCRIPT_DIR" || exit 1
    
    # Build extra args from test filters
    local extra_args=""
    
    if [ -n "$TEST_RUN" ]; then
        extra_args="$extra_args -run $TEST_RUN"
        echo "Test filter: -run $TEST_RUN"
    fi
    
    if [ -n "$TEST_PACKAGE" ]; then
        extra_args="$extra_args -package $TEST_PACKAGE"
        echo "Test package: $TEST_PACKAGE"
    fi
    
    # Set up coverage directory for integration tests
    local coverage_dir="$(pwd)/$OUTPUT_DIR/.test/integration"
    mkdir -p "$coverage_dir"
    
    # Export coverage directory for the server binary to use
    export GOCOVERDIR="$coverage_dir"
    
    echo "Coverage data will be collected in: $coverage_dir"
    go run -C ./tests/integration ./main.go $extra_args
    test_exit_code=$?
    
    # Process coverage data if tests passed or failed
    if [ -d "$coverage_dir" ] && [ "$(ls -A $coverage_dir 2>/dev/null)" ]; then
        echo "================================================================"
        echo "Processing integration test coverage..."
        
        # Convert binary coverage data to text format
        cd "$BACKEND_BASE_DIR" || exit 1
        go tool covdata textfmt -i="$coverage_dir" -o="../$TARGET_DIR/coverage_integration.out"
        echo "Integration test coverage report generated in: $TARGET_DIR/coverage_integration.out"
        
        # Generate HTML coverage report
        go tool cover -html="../$TARGET_DIR/coverage_integration.out" -o="../$TARGET_DIR/coverage_integration.html"
        echo "Integration test coverage HTML report generated in: $TARGET_DIR/coverage_integration.html"
        
        # Display coverage summary
        echo ""
        echo "================================================================"
        echo "Coverage Summary:"
        go tool cover -func="../$TARGET_DIR/coverage_integration.out" | tail -n 1
        echo "================================================================"
        echo ""
        
        cd "$SCRIPT_DIR" || exit 1
    else
        echo "================================================================"
        echo "No coverage data collected"
    fi
    
    # Exit with the test exit code
    if [ $test_exit_code -ne 0 ]; then
        echo "================================================================"
        echo "Integration tests failed with exit code: $test_exit_code"
        exit $test_exit_code
    fi
    
    echo "================================================================"
}

function merge_coverage() {
    echo "================================================================"
    echo "Merging coverage reports..."
    cd "$SCRIPT_DIR" || exit 1
    
    local unit_coverage="$BACKEND_BASE_DIR/coverage_unit.out"
    local integration_coverage="$TARGET_DIR/coverage_integration.out"
    local combined_coverage="$TARGET_DIR/coverage_combined.out"
    
    # Check if both coverage files exist
    if [ ! -f "$unit_coverage" ]; then
        echo "Warning: Unit test coverage file not found at $unit_coverage"
        echo "Skipping coverage merge."
        return 0
    fi
    
    if [ ! -f "$integration_coverage" ]; then
        echo "Warning: Integration test coverage file not found at $integration_coverage"
        echo "Skipping coverage merge."
        return 0
    fi
    
    echo "Merging unit and integration test coverage..."
    
    # Get the mode from the first file and write to combined coverage
    head -n 1 "$unit_coverage" > "$combined_coverage"
    
    # Combine both files (skip mode lines) and merge overlapping coverage
    { tail -n +2 "$unit_coverage"; tail -n +2 "$integration_coverage"; } | \
        awk '
        {
            key = $1 " " $2
            if (!(key in lines)) {
                lines[key] = $0
                count[key] = $3
            } else {
                # For duplicate entries, take the maximum count
                if ($3 > count[key]) {
                    count[key] = $3
                    lines[key] = $1 " " $2 " " $3
                }
            }
        }
        END {
            for (key in lines) {
                print lines[key]
            }
        }
        ' | sort >> "$combined_coverage"
    
    echo "Combined coverage report generated in: $combined_coverage"
    
    # Generate HTML coverage report for combined coverage
    cd "$BACKEND_BASE_DIR" || exit 1
    go tool cover -html="../$combined_coverage" -o="../$TARGET_DIR/coverage_combined.html"
    echo "Combined coverage HTML report generated in: $TARGET_DIR/coverage_combined.html"
    
    # Display combined coverage summary
    echo ""
    echo "================================================================"
    echo "Combined Test Coverage Summary:"
    go tool cover -func="../$combined_coverage" | tail -n 1
    echo "================================================================"
    echo ""
    
    cd "$SCRIPT_DIR" || exit 1
    echo "================================================================"
}

function ensure_certificates() {
    local cert_dir=$1
    local cert_name_prefix=${2:-"server"}  # Default to "server" if not specified
    local cert_file_name="${cert_name_prefix}.cert"
    local key_file_name="${cert_name_prefix}.key"

    # Generate certificate and key file if they don't exist in the cert directory
    local local_cert_file="${LOCAL_CERT_DIR}/${cert_file_name}"
    local local_key_file="${LOCAL_CERT_DIR}/${key_file_name}"
    if [[ ! -f "$local_cert_file" || ! -f "$local_key_file" ]]; then
        mkdir -p "$LOCAL_CERT_DIR"
        echo "Generating certificates (${cert_name_prefix}) in $LOCAL_CERT_DIR..."
        OPENSSL_ERR=$(
            openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
                -keyout "$local_key_file" \
                -out "$local_cert_file" \
                -subj "/O=WSO2/OU=${PRODUCT_NAME}/CN=localhost" \
                > /dev/null 2>&1
        )
        if [[ $? -ne 0 ]]; then
            echo "Error generating certificates: $OPENSSL_ERR"
            exit 1
        fi
        echo "Certificates generated successfully in $LOCAL_CERT_DIR."
    else
        echo "Certificates (${cert_name_prefix}) already exist in $LOCAL_CERT_DIR."
    fi

    # Copy the generated certificates to the specified directory
    local cert_file="$cert_dir/${cert_file_name}"
    local key_file="$cert_dir/${key_file_name}"

    if [[ ! -f "$cert_file" || ! -f "$key_file" ]]; then
        mkdir -p "$cert_dir"
        echo "Copying certificates (${cert_name_prefix}) to $cert_dir..."
        cp "$local_cert_file" "$cert_file"
        cp "$local_key_file" "$key_file"
        echo "Certificates copied successfully to $cert_dir."
    else
        echo "Certificates (${cert_name_prefix}) already exist in $cert_dir."
    fi
}

function ensure_crypto_file() {
    local KEY_DIR="$1"
    
    # Define the default path for the key file
    local KEY_FILE="$KEY_DIR/crypto.key"

    echo "=== Ensuring crypto key file exists in the distribution ==="

    # Check Whether the key file exists
    if [ -f "$KEY_FILE" ]; then
        echo "Default crypto key file already present in $KEY_FILE. Skipping generation."
    else
        echo "Default crypto key file not found. Generating new key at $KEY_FILE..."
        
        # Generate 32-byte key (64 hex characters) using openssl
        local NEW_KEY
        if ! NEW_KEY=$(openssl rand -hex 32); then
            echo "ERROR: Failed to generate crypto key using openssl."
            exit 1
        fi

        # Ensure the target directory exists
        mkdir -p "$KEY_DIR"

        # Write the key to the new file.
        echo -n "$NEW_KEY" > "$KEY_FILE"
        
        echo "Successfully generated and added new crypto key to $KEY_FILE."
    fi
    
    echo "================================================================"
}

function run() {
    cleanup_servers() {
        echo -e "\n🛑 Shutting down servers..."
        if [ ! -z "$FRONTEND_PID" ]; then 
            kill $FRONTEND_PID 2>/dev/null
        fi
        pkill -f "pnpm.*dev" 2>/dev/null
        pkill -f "vite" 2>/dev/null
        if [ ! -z "$BACKEND_PID" ]; then 
            kill $BACKEND_PID 2>/dev/null
        fi
        if [ ! -z "$CONSENT_PID" ]; then 
            kill $CONSENT_PID 2>/dev/null
        fi
        sleep 1
        echo "✅ All servers stopped successfully."
    }
    trap cleanup_servers EXIT INT TERM

    echo "Running frontend apps..."
    run_frontend

    if [ "$CONSENT_ENABLED" = "true" ] && [ "$WITHOUT_CONSENT" != "true" ]; then
        echo "Running consent server..."
        run_consent
    fi

    # Save original skip security value and temporarily set to true
    ORIGINAL_SKIP_SECURITY="${SKIP_SECURITY:-}"
    export SKIP_SECURITY=true
    run_backend false

    # Run initial data setup
    echo "⚙️  Running initial data setup..."
    echo ""
    
    # Wait for server to be ready
    MAX_RETRIES=30
    RETRY_INTERVAL=2
    retries=0
    
    echo "[INFO] Waiting for ${PRODUCT_NAME} server to be ready..."
    while [ $retries -lt $MAX_RETRIES ]; do
        if curl -k -s -f "$BASE_URL/health/readiness" > /dev/null 2>&1; then
            echo "✓ Server is ready!"
            break
        fi
        
        retries=$((retries + 1))
        if [ $retries -ge $MAX_RETRIES ]; then
            echo "❌ Server did not become ready after $MAX_RETRIES attempts"
            echo "💡 Please ensure the ${PRODUCT_NAME} server is running at $BASE_URL"
            exit 1
        fi
        
        echo "[WAITING] Attempt $retries/$MAX_RETRIES - Server not ready yet, retrying in ${RETRY_INTERVAL}s..."
        sleep $RETRY_INTERVAL
    done
    
    echo ""
    
    # Run the bootstrap script directly with environment variable and arguments
    API_BASE="$BASE_URL" \
        ADMIN_USERNAME="${ADMIN_USERNAME:-}" \
        ADMIN_PASSWORD="${ADMIN_PASSWORD:-}" \
        "$BACKEND_BASE_DIR/cmd/server/bootstrap/01-default-resources.sh" \
        --console-redirect-uris "https://localhost:$CONSOLE_APP_DEFAULT_PORT/console"

    if [ $? -ne 0 ]; then
        echo "❌ Initial data setup failed"
        echo "💡 Check the logs above for more details"
        exit 1
    fi

    echo "🔒 Restoring security setting and restarting backend..."
    # Restore original SKIP_SECURITY value
    if [ -n "$ORIGINAL_SKIP_SECURITY" ]; then
        export SKIP_SECURITY="$ORIGINAL_SKIP_SECURITY"
    else
        unset SKIP_SECURITY
    fi
    # Start backend with initial output but without final output/wait
    start_backend false

    echo ""
    echo "🚀 Servers running:"
    echo "  👉 Backend : $BASE_URL"
    echo "  📱 Frontend :"
    echo "      🚪 Gate (Login/Register): https://localhost:$GATE_APP_DEFAULT_PORT/gate"
    echo "      🛠️  Console (System Management): https://localhost:$CONSOLE_APP_DEFAULT_PORT/console"
    echo ""

    echo "Press Ctrl+C to stop."

    wait $BACKEND_PID 2>/dev/null
}

function debug_backend() {
    run_backend true true
}

function run_backend() {
    local show_final_output=${1:-true}
    local debug=${2:-false}

    echo "=== Ensuring server certificates exist ==="
    ensure_certificates "$BACKEND_DIR/$SECURITY_DIR" "server"
    ensure_certificates "$BACKEND_DIR/$SECURITY_DIR" "signing"

    echo "=== Ensuring sample app certificates exist ==="
    ensure_certificates "$VANILLA_SAMPLE_APP_DIR" "server"
    ensure_certificates "$REACT_API_SAMPLE_APP_DIR" "server"

    ensure_crypto_file "$BACKEND_DIR/$SECURITY_DIR"

    echo "Initializing databases..."
    initialize_databases

    if [ "$CONSENT_ENABLED" = "true" ] && [ "$WITHOUT_CONSENT" != "true" ] && [ -z "$CONSENT_PID" ]; then
        echo "Running consent server..."
        run_consent
    fi

    start_backend "$show_final_output" "$debug"
}

function start_backend() {
    local show_final_output=${1:-true}
    local debug=${2:-false}

    # Kill known ports
    function kill_port() {
        local port=$1
        lsof -ti tcp:$port | xargs kill -9 2>/dev/null || true
    }

    kill_port $PORT

    if [ "$debug" = "true" ]; then
        echo "=== Starting backend on $BASE_URL in debug mode ==="
        (
            cd "$BACKEND_DIR" || exit 1
            dlv debug --headless --listen=127.0.0.1:2345 --api-version=2 --accept-multiclient --continue -- .
        ) &
        BACKEND_PID=$!
    else
        echo "=== Starting backend on $BASE_URL ==="
        go run -C "$BACKEND_DIR" . &
        BACKEND_PID=$!
    fi

    if [ "$show_final_output" = "true" ]; then
        echo ""
        echo "🚀 Servers running:"
        echo "👉 Backend : $BASE_URL"
        echo "Press Ctrl+C to stop."

        trap 'echo -e "\n🛑 Shutting down servers..."; kill $BACKEND_PID 2>/dev/null; [ -n "$CONSENT_PID" ] && kill $CONSENT_PID 2>/dev/null; echo "✅ Servers stopped successfully."; exit 0' SIGINT

        wait $BACKEND_PID 2>/dev/null
    fi
}

function run_frontend() {
    echo "================================================================"
    echo "Running frontend apps..."
    ensure_pnpm

    sync_quickstart_bundles

    # Install dependencies
    echo "Installing frontend dependencies..."
    pnpm install --frozen-lockfile
    
    echo "Building frontend applications & packages..."
    pnpm build:frontend
    
    echo "Starting frontend applications in the background..."
    # Start frontend processes in background
    pnpm -r --parallel --filter "@thunderid/console" --filter "@thunderid/gate" dev &
    FRONTEND_PID=$!
    
    # Return to script directory
    cd "$SCRIPT_DIR" || exit 1
    echo "================================================================"
}

function run_docs() {
    echo "================================================================"
    echo "Starting documentation development server..."
    ensure_pnpm
    
    # Install dependencies
    echo "Installing frontend dependencies (required for docs)..."
    pnpm install --frozen-lockfile
    
    # Navigate to docs directory
    cd "$SCRIPT_DIR/docs" || exit 1
    
    echo "Starting documentation server with live reload..."
    echo "📚 Documentation will be available at http://localhost:$DOCS_DEFAULT_PORT"
    echo "Press Ctrl+C to stop."
    pnpm dev
    
    # Return to script directory
    cd "$SCRIPT_DIR" || exit 1
    echo "================================================================"
}

function run_consent() {
    local consent_dir="$TARGET_DIR/consent"
    local consent_port="${CONSENT_SERVER_PORT:-9090}"

    if [ ! -f "$consent_dir/consent-server" ]; then
        echo "=== Downloading consent server ==="
        ./scripts/package-consent-server.sh "$GO_OS" "$GO_ARCH" "$TARGET_DIR"
    fi

    if [ ! -f "$consent_dir/consent-server" ]; then
        echo "Error: Consent server binary not found at $consent_dir/consent-server"
        exit 1
    fi

    echo "=== Starting consent server ==="
    (cd "$consent_dir" && ./consent-server) &
    CONSENT_PID=$!
    CONSENT_TIMEOUT=30
    CONSENT_ELAPSED=0
    while [ $CONSENT_ELAPSED -lt $CONSENT_TIMEOUT ]; do
        if ! kill -0 "$CONSENT_PID" 2>/dev/null; then
            echo "Error: Consent server process exited unexpectedly"
            exit 1
        fi
        if curl -s -f "http://localhost:${consent_port}/health/readiness" > /dev/null 2>&1; then
            echo "Consent server is ready"
            break
        fi
        sleep 1
        CONSENT_ELAPSED=$((CONSENT_ELAPSED + 1))
    done
    if [ $CONSENT_ELAPSED -ge $CONSENT_TIMEOUT ]; then
        echo "Error: Consent server failed to become ready within ${CONSENT_TIMEOUT}s"
        exit 1
    fi
    
    echo "Consent server started (PID: $CONSENT_PID)"
}

case "$1" in
    clean)
        clean
        ;;
    build_backend)
        build_backend
        package
        ;;
    build_frontend)
        build_frontend
        ;;
    build_docs)
        build_docs
        ;;
    build_sdks)
        build_sdks
        ;;
    test_sdks)
        test_sdks
        ;;
    lint_sdks)
        lint_sdks
        ;;
    build_samples)
        build_sdks_js
        build_sample_app
        package_sample_app
        ;;
    package_samples)
        package_sample_app
        ;;
    build)
        build_backend
        build_frontend
        build_sdks
        package
        build_sample_app
        package_sample_app
        ;;
    test_unit)
        test_unit
        ;;
    test_integration)
        test_integration
        ;;
    merge_coverage)
        merge_coverage
        ;;
    test)
        test_unit
        test_integration
        ;;
    run)
        run
        ;;
    run_backend)
        run_backend
        ;;
    debug_backend)
        debug_backend
        ;;
    run_frontend)
        run_frontend
        ;;
    run_docs)
        run_docs
        ;;
    *)
        echo "Usage: ./build.sh {clean|build|build_backend|build_frontend|build_docs|test|run} [OS] [ARCH]"
        echo ""
        echo "  clean                    - Clean build artifacts"
        echo "  build                    - Build the complete ${PRODUCT_NAME} application (backend + frontend + samples)"
        echo "  build_backend            - Build only the ${PRODUCT_NAME} backend server"
        echo "  build_frontend           - Build only the Next.js frontend applications"
        echo "  build_docs               - Build only the documentation"
        echo "  build_sdks               - Build all SDK packages"
        echo "  test_sdks                - Run tests for all SDK packages"
        echo "  lint_sdks                - Run linting for all SDK packages"
        echo "  build_samples            - Build the sample applications"
        echo "  test_unit                - Run unit tests with coverage"
        echo "  test_integration         - Run integration tests. Use -run and -package for filtering"
        echo "  merge_coverage           - Merge unit and integration test coverage reports"
        echo "  test                     - Run all tests (unit and integration)"
        echo "  run                      - Run the ${PRODUCT_NAME} server for development (with automatic initial data setup)"
        echo "  run_backend              - Run the ${PRODUCT_NAME} backend for development"
        echo "  debug_backend            - Run the ${PRODUCT_NAME} backend for development in debug mode"
        echo "  run_frontend             - Run the ${PRODUCT_NAME} frontend for development"
        echo "  run_docs                 - Run the documentation development server with live reload"
        echo ""
        echo "  --without-consent        - Skip packaging/running the consent server"
        exit 1
        ;;
esac
