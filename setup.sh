#!/bin/bash
# ----------------------------------------------------------------------------
# Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

# Product Setup Script
# Orchestrates the complete setup lifecycle:
# 1. Starts the server with security disabled
# 2. Executes bootstrap scripts (built-in + custom)
# 3. Stops the server
# 4. Exits cleanly

# Ensure the script runs with Bash even if invoked via `sh`
if [ -z "${BASH_VERSION:-}" ]; then
    exec /usr/bin/env bash "$0" "$@"
fi

# Guard against Bash POSIX mode (e.g., Bash started with --posix or POSIXLY_CORRECT);
# use a guard to avoid re-exec loops if POSIX stays enabled.
if [ -z "${SETUP_SH_POSIX_REEXEC:-}" ] && set -o | grep -q 'posix.*on'; then
    SETUP_SH_POSIX_REEXEC=1 exec /usr/bin/env bash "$0" "$@"
fi

set -e

# Default settings
PRODUCT_NAME="ThunderID"
PRODUCT_NAME_LOWERCASE="$(echo "$PRODUCT_NAME" | tr '[:upper:]' '[:lower:]')"
DEBUG_PORT=${DEBUG_PORT:-2345}
DEBUG_MODE=${DEBUG_MODE:-false}
VERBOSE_MODE=${VERBOSE_MODE:-false}
SILENT_MODE=true
ADMIN_USERNAME_PROVIDED=false
ADMIN_PASSWORD_PROVIDED=false
if [[ -n "${ADMIN_USERNAME:-}" ]]; then
    ADMIN_USERNAME_PROVIDED=true
fi
if [[ -n "${ADMIN_PASSWORD:-}" ]]; then
    ADMIN_PASSWORD_PROVIDED=true
fi
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
# Left empty when not supplied: configure_admin_password (below) generates a random
# password in that case, rather than falling back to a fixed, predictable value.
ADMIN_PASSWORD="${ADMIN_PASSWORD:-}"
ADMIN_PASSWORD_GENERATED=false
# Direct Auth Secret gates the Direct API endpoints (secure by default). When not supplied, one is
# generated during setup and written to the secret file referenced by deployment.yaml.
DIRECT_AUTH_SECRET="${DIRECT_AUTH_SECRET:-}"
DIRECT_AUTH_SECRET_GENERATED=false
DIRECT_AUTH_SECRET_FILE=""
# Set when key material is generated this run (controls the one-time notice).
CERTS_GENERATED=false

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# ============================================================================
# Logging Functions
# ============================================================================

log_info() {
    if [ "$VERBOSE_MODE" = "true" ]; then
        echo -e "${BLUE}[INFO]${NC} $1"
    fi
}

log_success() {
    if [ "$VERBOSE_MODE" = "true" ]; then
        echo -e "${GREEN}[SUCCESS]${NC} ✓ $1"
    fi
}

log_warning() {
    if [ "$VERBOSE_MODE" = "true" ]; then
        echo -e "${YELLOW}[WARNING]${NC} ⚠ $1"
    fi
}

log_error() {
    echo -e "${RED}[ERROR]${NC} ✗ $1"
}

log_debug() {
    if [ "${DEBUG:-false}" = "true" ] && [ "$VERBOSE_MODE" = "true" ]; then
        echo -e "${CYAN}[DEBUG]${NC} $1"
    fi
}

# ============================================================================
# Help Function
# ============================================================================

print_help() {
    echo ""
    echo "${PRODUCT_NAME} Setup Script"
    echo ""
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  --verbose                Enable detailed setup output"
    echo "  --debug                  Enable debug mode with remote debugging"
    echo "  --debug-port PORT        Set debug port (default: 2345)"
    echo "  --admin-username VALUE   Username for the default admin user (default: admin)"
    echo "                           Falls back to ADMIN_USERNAME env var if flag not set"
    echo "  --admin-password VALUE   Password for the default admin user"
    echo "                           Falls back to ADMIN_PASSWORD env var; generated if unset"
    echo "  --direct-auth-secret VALUE Secret gating the Direct API endpoints"
    echo "                           Falls back to DIRECT_AUTH_SECRET env var; generated if unset"
    echo "  --help                   Show this help message"
    echo ""
    echo "Description:"
    echo "  This script performs initial setup by:"
    echo "  1. Starting ${PRODUCT_NAME} server temporarily with security disabled"
    echo "  2. Running bootstrap scripts to create default resources"
    echo "  3. Stopping the server cleanly"
    echo ""
    echo "  After setup completes, use './start.sh' to start ${PRODUCT_NAME} normally."
    echo ""
}

# ============================================================================
# Parse Command Line Arguments
# ============================================================================

while [[ $# -gt 0 ]]; do
    case $1 in
        --debug)
            DEBUG_MODE=true
            shift
            ;;
        --verbose)
            VERBOSE_MODE=true
            SILENT_MODE=false
            shift
            ;;
        --debug-port)
            DEBUG_PORT="$2"
            shift 2
            ;;
        --admin-username)
            if [[ -z "${2:-}" || "${2:-}" == --* ]]; then
                echo -e "${RED}--admin-username requires a non-empty value${NC}"
                exit 1
            fi
            ADMIN_USERNAME="$2"
            ADMIN_USERNAME_PROVIDED=true
            shift 2
            ;;
        --admin-password)
            if [[ -z "${2:-}" || "${2:-}" == --* ]]; then
                echo -e "${RED}--admin-password requires a non-empty value${NC}"
                exit 1
            fi
            ADMIN_PASSWORD="$2"
            ADMIN_PASSWORD_PROVIDED=true
            shift 2
            ;;
        --direct-auth-secret)
            if [[ -z "${2:-}" || "${2:-}" == --* ]]; then
                echo -e "${RED}--direct-auth-secret requires a non-empty value${NC}"
                exit 1
            fi
            DIRECT_AUTH_SECRET="$2"
            shift 2
            ;;
        --help)
            print_help
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# configure_admin_password ensures the default admin account is usable out of the box while staying
# secure by default: it uses the provided value, or generates a random one. Unlike the Direct Auth
# Secret, this intentionally regenerates every run where no value is supplied (no persisted value to
# check), so re-running setup.sh with nothing explicit set is also how an operator resets the password.
configure_admin_password() {
    if [ -n "$ADMIN_PASSWORD" ]; then
        return 0
    fi

    # Generate a 12-character password mixing letters, digits, and special characters.
    # The special set is limited to shell- and YAML-safe punctuation, because the value
    # flows through environment variables, script arguments, and the bundle's YAML
    # template before it is stored. Regenerate until the result contains at least one
    # digit and one special character so it reliably looks like a password.
    charset='A-Za-z0-9@#%+=_.?-'
    while true; do
        ADMIN_PASSWORD="$(LC_ALL=C tr -dc "$charset" < /dev/urandom 2>/dev/null | head -c 12 || true)"
        [ "${#ADMIN_PASSWORD}" -eq 12 ] || continue
        case "$ADMIN_PASSWORD" in *[0-9]*) ;; *) continue ;; esac
        case "$ADMIN_PASSWORD" in *[@#%+=_.?-]*) break ;; esac
    done
    ADMIN_PASSWORD_GENERATED=true
}

# ============================================================================
# Prompt for Admin Credentials (interactive mode only)
# ============================================================================

# Prompt for any credential not supplied via CLI flags or environment
# variables, but only when stdin is a terminal.
if [ -t 0 ] && [[ "$ADMIN_USERNAME_PROVIDED" == "false" || "$ADMIN_PASSWORD_PROVIDED" == "false" ]]; then
    echo ""
    echo "Configure the default admin user (press Enter to accept defaults):"
    echo ""
    if [[ "$ADMIN_USERNAME_PROVIDED" == "false" ]]; then
        read -r -p "  Admin username [admin]: " _input_username
        ADMIN_USERNAME="${_input_username:-admin}"
    fi
    if [[ "$ADMIN_PASSWORD_PROVIDED" == "false" ]]; then
        # Generate the password up front so it can be shown as the prompt default (the
        # value used if the operator presses Enter). A typed value overrides it.
        configure_admin_password
        read -r -s -p "  Admin password [$ADMIN_PASSWORD]: " _input_password
        echo ""
        if [ -n "$_input_password" ]; then
            ADMIN_PASSWORD="$_input_password"
            ADMIN_PASSWORD_GENERATED=false
        fi
    fi
    echo ""
fi

# ============================================================================
# Read Configuration from deployment.yaml
# ============================================================================

CONFIG_FILE="./deployment.yaml"

# Function to read config with fallback
read_config() {
    local config_file="$CONFIG_FILE"

    if [ ! -f "$config_file" ]; then
        # Try alternative path (for packaged distribution)
        config_file="./backend/cmd/server/deployment.yaml"
    fi

    if [ ! -f "$config_file" ]; then
        log_warning "Configuration file not found, using defaults"
        return 1
    fi

    log_debug "Reading configuration from: $config_file"

    # Try yq first (YAML parser)
    if command -v yq >/dev/null 2>&1; then
        HOSTNAME=$(yq eval '.server.hostname // "localhost"' "$config_file" 2>/dev/null)
        PORT=$(yq eval '.server.port // 8090' "$config_file" 2>/dev/null)
        HTTP_ONLY=$(yq eval '.server.http_only // false' "$config_file" 2>/dev/null)
        PUBLIC_URL=$(yq eval '.server.public_url // ""' "$config_file" 2>/dev/null)
    else
        # Fallback: basic parsing with grep/awk
        HOSTNAME=$(grep -E '^\s*hostname:' "$config_file" | sed 's/#.*//' | awk -F':' '{gsub(/[[:space:]"'\'']/,"",$2); print $2}' | head -1)
        PORT=$(grep -E '^\s*port:' "$config_file" | sed 's/#.*//' | awk -F':' '{gsub(/[[:space:]]/,"",$2); print $2}' | head -1)
        # Parse public_url (quoted or unquoted)
        PUBLIC_URL=$(grep -E '^\s*public_url:' "$config_file" | sed 's/#.*//' | grep -o '"[^"]*"' | tr -d '"' | head -1)
        if [ -z "$PUBLIC_URL" ]; then
            PUBLIC_URL=$(grep -E '^\s*public_url:' "$config_file" | sed 's/#.*//' | sed 's/^[[:space:]]*public_url:[[:space:]]*//' | sed 's/[[:space:]]*$//' | head -1)
        fi

        # Check for http_only
        if grep -q 'http_only.*true' "$config_file" 2>/dev/null; then
            HTTP_ONLY="true"
        else
            HTTP_ONLY="false"
        fi

        # Use defaults if not found
        HOSTNAME=${HOSTNAME:-localhost}
        PORT=${PORT:-8090}

    fi

    # Determine protocol
    if [ "$HTTP_ONLY" = "true" ]; then
        PROTOCOL="http"
    else
        PROTOCOL="https"
    fi
    return 0
}

# resolveConfigFile prints the path to the deployment.yaml in use, or nothing if not found.
resolve_config_file() {
    if [ -f "$CONFIG_FILE" ]; then
        echo "$CONFIG_FILE"
    elif [ -f "./backend/cmd/server/deployment.yaml" ]; then
        echo "./backend/cmd/server/deployment.yaml"
    fi
}

# print_admin_credentials_notice shows the generated admin password once, so the operator can capture
# it. Only shown when the password was generated during this run (not for operator-supplied values).
print_admin_credentials_notice() {
    if [ "$ADMIN_PASSWORD_GENERATED" != "true" ]; then
        return 0
    fi
    echo "Admin credentials:"
    echo "  Username: ${ADMIN_USERNAME}"
    echo "  Password: ${ADMIN_PASSWORD}"
    echo "  Sign in to the Console with these credentials."
    echo ""
}

# configure_direct_auth_secret ensures the Direct API is usable out of the box while staying secure by
# default. The secret is persisted to config/secrets/direct_auth_secret and the server reads it via the
# file:// reference in deployment.yaml. This keeps generation working when deployment.yaml is read-only
# (e.g. a mounted Kubernetes ConfigMap): only the secrets directory needs to be writable. An operator can
# still set an explicit inline secret in deployment.yaml, which is honored as-is.
configure_direct_auth_secret() {
    local config_file existing ref secret_file
    config_file="$(resolve_config_file)"
    if [ -z "$config_file" ]; then
        log_warning "deployment.yaml not found; skipping Direct Auth Secret configuration"
        return 0
    fi

    # Inspect the configured secret. A file:// reference points at the secret file this script
    # maintains; a plain value means the operator set an explicit secret, which is honored as-is.
    existing=$(grep -E '^[[:space:]]*direct_auth_secret:' "$config_file" | sed 's/#.*//' \
        | sed -E 's/^[[:space:]]*direct_auth_secret:[[:space:]]*//' | tr -d '"'\''[:space:]' | head -1)
    ref="${existing#file://}"
    if [ -n "$existing" ] && [ "$ref" = "$existing" ]; then
        DIRECT_AUTH_SECRET="$existing"
        return 0
    fi

    # Resolve the target secret file. Prefer the path from the file:// reference (relative to the
    # deployment.yaml directory); otherwise fall back to the default location.
    if [ -n "$ref" ]; then
        case "$ref" in
            /*) secret_file="$ref" ;;
            *)  secret_file="$(dirname "$config_file")/$ref" ;;
        esac
    else
        secret_file="$(dirname "$config_file")/config/secrets/direct_auth_secret"
    fi
    mkdir -p "$(dirname "$secret_file")"
    # Record the resolved path so the notice can report where the secret was written.
    DIRECT_AUTH_SECRET_FILE="$secret_file"

    # An explicit provided value is written to the secret file. Otherwise reuse an existing secret,
    # or generate a random one. Guard openssl with '|| true' so a failure does not abort the script
    # under 'set -e' before the /dev/urandom fallback runs.
    if [ -n "$DIRECT_AUTH_SECRET" ]; then
        printf '%s' "$DIRECT_AUTH_SECRET" >"$secret_file"
        chmod 600 "$secret_file"
        return 0
    fi
    if [ -s "$secret_file" ]; then
        chmod 600 "$secret_file"
        DIRECT_AUTH_SECRET="$(cat "$secret_file")"
        return 0
    fi
    DIRECT_AUTH_SECRET="$(openssl rand -hex 32 2>/dev/null || true)"
    if [ -z "$DIRECT_AUTH_SECRET" ]; then
        DIRECT_AUTH_SECRET="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
    fi
    printf '%s' "$DIRECT_AUTH_SECRET" >"$secret_file"
    chmod 600 "$secret_file"
    DIRECT_AUTH_SECRET_GENERATED=true
}

# print_direct_auth_secret_notice shows the generated Direct Auth Secret once, so the operator can capture
# it. Only shown when the secret was generated during this run (not for operator-supplied values).
print_direct_auth_secret_notice() {
    if [ "$DIRECT_AUTH_SECRET_GENERATED" != "true" ]; then
        return 0
    fi
    echo "Direct Auth Secret (Direct API): ${DIRECT_AUTH_SECRET}"
    echo "  Send it in the 'Direct-Auth-Secret' header when calling the Direct API endpoints."
    echo "  It has been written to ${DIRECT_AUTH_SECRET_FILE}, which deployment.yaml"
    echo "  references via server.security.direct_auth_secret."
    echo ""
}

# Generate a self-signed cert/key pair if it does not already exist.
generate_x509_cert() {
    local cert_file="$1" key_file="$2" algo="$3"
    if [ -f "$cert_file" ] && [ -f "$key_file" ]; then
        return 0
    fi
    if [ "$algo" = "ecdsa" ]; then
        openssl ecparam -name prime256v1 -genkey -noout -param_enc named_curve -out "$key_file" >/dev/null 2>&1 || true
        openssl req -new -x509 -nodes -days 3650 -key "$key_file" -out "$cert_file" \
            -subj "/O=WSO2/OU=${PRODUCT_NAME}/CN=localhost" \
            -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" >/dev/null 2>&1 || true
    else
        openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout "$key_file" -out "$cert_file" \
            -subj "/O=WSO2/OU=${PRODUCT_NAME}/CN=localhost" \
            -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" >/dev/null 2>&1 || true
    fi
    if [ ! -f "$cert_file" ] || [ ! -f "$key_file" ]; then
        log_error "Failed to generate certificate: $cert_file (is openssl installed?)"
        exit 1
    fi
    CERTS_GENERATED=true
}

# Generate the server TLS, JWT signing, and AES key material if absent (reused on later runs).
configure_certificates() {
    local config_file cert_dir
    config_file="$(resolve_config_file)"
    if [ -n "$config_file" ]; then
        cert_dir="$(dirname "$config_file")/config/certs"
    else
        cert_dir="./config/certs"
    fi
    mkdir -p "$cert_dir"

    generate_x509_cert "$cert_dir/server.cert" "$cert_dir/server.key" rsa
    generate_x509_cert "$cert_dir/signing.cert" "$cert_dir/signing.key" rsa
    generate_x509_cert "$cert_dir/ecdsa-signing.cert" "$cert_dir/ecdsa-signing.key" ecdsa

    local crypto_key="$cert_dir/crypto.key"
    if [ ! -f "$crypto_key" ]; then
        local key
        key="$(openssl rand -hex 32 2>/dev/null || true)"
        if [ -z "$key" ]; then
            key="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
        fi
        printf '%s' "$key" > "$crypto_key"
        CERTS_GENERATED=true
    fi
}

# Print a one-time notice when key material was generated this run.
print_certificates_notice() {
    if [ "$CERTS_GENERATED" != "true" ]; then
        return 0
    fi
    echo "Generated missing security material in config/certs."
    echo "  Preserve this directory; if these keys are lost or changed, previously issued tokens and encrypted data can no longer be validated or decrypted."
    echo ""
}

# Read configuration
read_config

# Configure the admin password and Direct Auth Secret before bootstrap so both are ready to use.
configure_admin_password
configure_direct_auth_secret
configure_certificates

# Construct base URL (internal API endpoint)
BASE_URL="${PROTOCOL}://${HOSTNAME}:${PORT}"

# Construct public URL (external/redirect URLs), strip trailing slash to avoid double slashes in paths
PUBLIC_URL="${PUBLIC_URL:-$BASE_URL}"
PUBLIC_URL="${PUBLIC_URL%/}"

echo ""
echo "========================================="
echo "   ${PRODUCT_NAME} Setup"
echo "========================================="
echo ""
if [ "$VERBOSE_MODE" = "true" ]; then
    echo -e "${BLUE}Server URL:${NC} $BASE_URL"
    echo -e "${BLUE}Public URL:${NC} $PUBLIC_URL"
    if [ "$DEBUG_MODE" = "true" ]; then
        echo -e "${BLUE}Debug:${NC} Enabled (port $DEBUG_PORT)"
    fi
    echo ""
fi

# ============================================================================
# Check for Port Conflicts
# ============================================================================

check_port() {
    local port=$1
    local port_name=$2
    if lsof -ti tcp:$port >/dev/null 2>&1; then
        echo ""
        echo -e "${RED}❌ Port $port is already in use${NC}"
        echo -e "${RED}   $port_name cannot start because another process is using port $port${NC}"
        echo ""
        echo -e "${YELLOW}💡 To find the process using this port:${NC}"
        echo "   lsof -i tcp:$port"
        echo ""
        echo -e "${YELLOW}💡 To stop the process:${NC}"
        echo "   kill -9 \$(lsof -ti tcp:$port)"
        echo ""
        exit 1
    fi
}

# Check if ports are available
check_port $PORT "${PRODUCT_NAME} server"
if [ "$DEBUG_MODE" = "true" ]; then
    check_port $DEBUG_PORT "Debug server"
fi

# Check for Delve if debug mode is enabled
if [ "$DEBUG_MODE" = "true" ] && ! command -v dlv &> /dev/null; then
    echo -e "${RED}❌ Debug mode requires Delve debugger${NC}"
    echo ""
    echo "💡 Install Delve using:"
    echo "   go install github.com/go-delve/delve/cmd/dlv@latest"
    exit 1
fi

# ============================================================================
# Create Default Resources (in-process bootstrap)
# ============================================================================
#
# Delegates to start.sh --bootstrap, which runs the binary's in-process bootstrap
# one-shot (create the default resources through the service layer, then exit).
# Admin credentials and the public URL are exported so the bootstrap subcommand
# picks them up.

export PUBLIC_URL="${PUBLIC_URL}"
export ADMIN_USERNAME
export ADMIN_PASSWORD

START_SCRIPT="$(dirname "$0")/start.sh"
if [ ! -x "$START_SCRIPT" ]; then
    log_error "start.sh is missing or not executable"
    exit 1
fi

if [ "$VERBOSE_MODE" = "true" ]; then
    echo -e "${BLUE}⏳ Creating default resources...${NC}"
fi

BOOTSTRAP_LOG="$(mktemp)"
if "$START_SCRIPT" --bootstrap >"$BOOTSTRAP_LOG" 2>&1; then
    [ "$VERBOSE_MODE" = "true" ] && cat "$BOOTSTRAP_LOG"
    rm -f "$BOOTSTRAP_LOG"
    log_success "Default resources created"
else
    cat "$BOOTSTRAP_LOG"
    rm -f "$BOOTSTRAP_LOG"
    log_error "Failed to create default resources"
    exit 1
fi

# ============================================================================
# Setup Completed
# ============================================================================

echo ""
echo ""
if [ "$SILENT_MODE" = "true" ]; then
    echo "========================================="
    echo "✅ Setup completed successfully!"
    echo "========================================="
    echo ""
    echo "Console URL: ${PUBLIC_URL}/console"
    echo ""
    print_admin_credentials_notice
    print_direct_auth_secret_notice
    print_certificates_notice
    echo "Run ./start.sh to start ${PRODUCT_NAME}."
    echo ""
else
    echo "========================================="
    echo -e "${GREEN}✅ Setup completed successfully!${NC}"
    echo "========================================="
    echo ""
    print_admin_credentials_notice
    print_direct_auth_secret_notice
    print_certificates_notice
fi

# Cleanup will be called automatically via trap
exit 0
