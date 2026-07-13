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
WITH_CONSENT=${WITH_CONSENT:-true}
ADMIN_USERNAME_PROVIDED=false
ADMIN_PASSWORD_PROVIDED=false
if [[ -n "${ADMIN_USERNAME:-}" ]]; then
    ADMIN_USERNAME_PROVIDED=true
fi
if [[ -n "${ADMIN_PASSWORD:-}" ]]; then
    ADMIN_PASSWORD_PROVIDED=true
fi
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}"
# Direct Auth Secret gates the Direct API endpoints (secure by default). When not supplied, one is
# generated during setup and written to deployment.yaml.
DIRECT_AUTH_SECRET="${DIRECT_AUTH_SECRET:-}"
DIRECT_AUTH_SECRET_GENERATED=false

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
    echo "  --without-consent        Disable the bundled consent server"
    echo "  --admin-username VALUE   Username for the default admin user (default: admin)"
    echo "                           Falls back to ADMIN_USERNAME env var if flag not set"
    echo "  --admin-password VALUE   Password for the default admin user (default: admin)"
    echo "                           Falls back to ADMIN_PASSWORD env var if flag not set"
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
        --without-consent)
            WITH_CONSENT=false
            shift
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
        read -r -s -p "  Admin password [admin]: " _input_password
        echo ""
        ADMIN_PASSWORD="${_input_password:-admin}"
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

# write_direct_auth_secret writes the Direct Auth Secret into server.security.direct_auth_secret of the
# given deployment.yaml, updating an existing key, adding it under an existing security block, or
# creating a security block under server. Uses awk (portable across macOS/Linux) instead of sed -i.
write_direct_auth_secret() {
    local file="$1" val="$2" tmp esc
    # Escape for a YAML double-quoted scalar: backslash first, then double-quote. Pass via the
    # environment (ENVIRON) so awk does not re-process the backslash escapes.
    esc=${val//\\/\\\\}
    esc=${esc//\"/\\\"}
    tmp="$(mktemp)"
    if grep -qE '^[[:space:]]*direct_auth_secret:' "$file"; then
        esc="$esc" awk '
            /^[[:space:]]*direct_auth_secret:/ && !done {
                match($0, /^[[:space:]]*/); print substr($0, 1, RLENGTH) "direct_auth_secret: \"" ENVIRON["esc"] "\""
                done=1; next
            }
            { print }
        ' "$file" >"$tmp"
    elif grep -qE '^[[:space:]]*security:[[:space:]]*$' "$file"; then
        esc="$esc" awk '
            { print }
            /^[[:space:]]*security:[[:space:]]*$/ && !done { print "    direct_auth_secret: \"" ENVIRON["esc"] "\""; done=1 }
        ' "$file" >"$tmp"
    else
        esc="$esc" awk '
            { print }
            /^server:[[:space:]]*$/ && !done { print "  security:"; print "    direct_auth_secret: \"" ENVIRON["esc"] "\""; done=1 }
        ' "$file" >"$tmp"
    fi
    mv "$tmp" "$file"
}

# configure_direct_auth_secret ensures the Direct API is usable out of the box while staying secure by
# default: it respects an existing non-empty secret, otherwise uses the provided value or generates a
# random one, and writes it into deployment.yaml.
configure_direct_auth_secret() {
    local config_file existing
    config_file="$(resolve_config_file)"
    if [ -z "$config_file" ]; then
        log_warning "deployment.yaml not found; skipping Direct Auth Secret configuration"
        return 0
    fi

    # Respect an existing non-empty value (e.g. from a prior setup run or manual config).
    existing=$(grep -E '^[[:space:]]*direct_auth_secret:' "$config_file" | sed 's/#.*//' \
        | sed -E 's/^[[:space:]]*direct_auth_secret:[[:space:]]*//' | tr -d '"'\''[:space:]' | head -1)
    if [ -n "$existing" ]; then
        DIRECT_AUTH_SECRET="$existing"
        return 0
    fi

    # Use the provided value, or generate a random one. Guard openssl with '|| true' so a failure
    # does not abort the script under 'set -e' before the /dev/urandom fallback runs.
    if [ -z "$DIRECT_AUTH_SECRET" ]; then
        DIRECT_AUTH_SECRET="$(openssl rand -hex 32 2>/dev/null || true)"
        if [ -z "$DIRECT_AUTH_SECRET" ]; then
            DIRECT_AUTH_SECRET="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
        fi
        DIRECT_AUTH_SECRET_GENERATED=true
    fi

    write_direct_auth_secret "$config_file" "$DIRECT_AUTH_SECRET"
}

# print_direct_auth_secret_notice shows the generated Direct Auth Secret once, so the operator can capture
# it. Only shown when the secret was generated during this run (not for operator-supplied values).
print_direct_auth_secret_notice() {
    if [ "$DIRECT_AUTH_SECRET_GENERATED" != "true" ]; then
        return 0
    fi
    echo "Direct Auth Secret (Direct API): ${DIRECT_AUTH_SECRET}"
    echo "  Send it in the 'Direct-Auth-Secret' header when calling the Direct API endpoints."
    echo "  It has been written to deployment.yaml (server.security.direct_auth_secret)."
    echo ""
}

# Read configuration
read_config

# Configure the Direct Auth Secret before bootstrap so the Direct API is ready to use.
configure_direct_auth_secret

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
# Start Consent Server (if enabled)
# ============================================================================

CONSENT_PID=""

# Cleanup function — stops the consent server started below.
cleanup() {
    if [ -n "$CONSENT_PID" ]; then
        if [ "$VERBOSE_MODE" = "true" ]; then
            echo ""
            echo -e "${CYAN}🛑 Stopping consent server...${NC}"
        fi
        pkill -P $CONSENT_PID 2>/dev/null || true
        kill $CONSENT_PID 2>/dev/null || true
        wait $CONSENT_PID 2>/dev/null || true
    fi
}
trap cleanup EXIT INT TERM

CONSENT_SERVER_PORT="${CONSENT_SERVER_PORT:-9090}"
if [ "$WITH_CONSENT" = "true" ]; then
    CONSENT_SCRIPT="$(dirname "$0")/consent/start.sh"
    if [ ! -x "$CONSENT_SCRIPT" ]; then
        log_error "Consent server is enabled but consent/start.sh is missing or not executable"
        exit 1
    fi
    if [ "$VERBOSE_MODE" = "true" ]; then
        echo -e "${CYAN}Starting Consent Server...${NC}"
        (cd "$(dirname "$0")/consent" && ./start.sh) &
    else
        (cd "$(dirname "$0")/consent" && ./start.sh >/dev/null 2>&1) &
    fi
    CONSENT_PID=$!
    CONSENT_TIMEOUT=30
    CONSENT_ELAPSED=0
    while [ $CONSENT_ELAPSED -lt $CONSENT_TIMEOUT ]; do
        if ! kill -0 "$CONSENT_PID" 2>/dev/null; then
            log_error "Consent server process exited unexpectedly"
            exit 1
        fi
        if curl -s -f "http://localhost:${CONSENT_SERVER_PORT}/health/readiness" > /dev/null 2>&1; then
            if [ "$VERBOSE_MODE" = "true" ]; then
                echo -e "${GREEN}✓ Consent server is ready${NC}"
            fi
            break
        fi
        sleep 1
        CONSENT_ELAPSED=$((CONSENT_ELAPSED + 1))
    done
    if [ $CONSENT_ELAPSED -ge $CONSENT_TIMEOUT ]; then
        log_error "Consent server failed to become ready within ${CONSENT_TIMEOUT}s"
        exit 1
    fi
fi

# ============================================================================
# Create Default Resources (in-process bootstrap)
# ============================================================================
#
# Delegates to start.sh --bootstrap, which runs the binary's in-process bootstrap
# one-shot (create the default resources through the service layer, then exit).
# The consent server, if enabled, was already started above, so --without-consent
# is passed to avoid starting a second one. Admin credentials and the public URL
# are exported so the bootstrap subcommand picks them up.

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
if "$START_SCRIPT" --bootstrap --without-consent >"$BOOTSTRAP_LOG" 2>&1; then
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
    print_direct_auth_secret_notice
    echo "Run ./start.sh to start ${PRODUCT_NAME}."
    echo ""
else
    echo "========================================="
    echo -e "${GREEN}✅ Setup completed successfully!${NC}"
    echo "========================================="
    echo ""
    print_direct_auth_secret_notice
fi

# Cleanup will be called automatically via trap
exit 0
