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
BINARY_NAME="${PRODUCT_NAME_LOWERCASE}"
DEBUG_PORT=${DEBUG_PORT:-2345}
DEBUG_MODE=${DEBUG_MODE:-false}
VERBOSE_MODE=${VERBOSE_MODE:-false}
SILENT_MODE=true
BOOTSTRAP_FAIL_FAST=${BOOTSTRAP_FAIL_FAST:-true}
BOOTSTRAP_SKIP_PATTERN="${BOOTSTRAP_SKIP_PATTERN:-}"
BOOTSTRAP_ONLY_PATTERN="${BOOTSTRAP_ONLY_PATTERN:-}"
BOOTSTRAP_DIR="${BOOTSTRAP_DIR:-./bootstrap}"
WITH_CONSENT=${WITH_CONSENT:-true}

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
# API Call Helper Function
# ============================================================================

api_call() {
    local method="$1"
    local endpoint="$2"
    local data="${3:-}"

    local url="${API_BASE}${endpoint}"

    log_debug "API Call: $method $url"

    if [ -z "$data" ]; then
        curl -k -s -w "\n%{http_code}" -X "$method" \
            "$url" \
            -H "Content-Type: application/json" 2>/dev/null || echo "000"
    else
        curl -k -s -w "\n%{http_code}" -X "$method" \
            "$url" \
            -H "Content-Type: application/json" \
            -d "$data" 2>/dev/null || echo "000"
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
# Read Configuration from deployment.yaml
# ============================================================================

CONFIG_FILE="./repository/conf/deployment.yaml"

# Function to read config with fallback
read_config() {
    local config_file="$CONFIG_FILE"

    if [ ! -f "$config_file" ]; then
        # Try alternative path (for packaged distribution)
        config_file="./backend/cmd/server/repository/conf/deployment.yaml"
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
        SYSTEM_RS_HANDLE=$(yq eval '.resource.system_resource_server.handle // ""' "$config_file" 2>/dev/null)
        SYSTEM_RS_IDENTIFIER=$(yq eval '.resource.system_resource_server.identifier // ""' "$config_file" 2>/dev/null)
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

        # Read system resource server config (nested under resource:)
        SYSTEM_RS_HANDLE=$(grep -A5 'system_resource_server:' "$config_file" 2>/dev/null | grep -E '^\s*handle:' | awk -F':' '{gsub(/[[:space:]"'\'']/,"",$2); print $2}' | head -1)
        SYSTEM_RS_IDENTIFIER=$(grep -A5 'system_resource_server:' "$config_file" 2>/dev/null | grep -E '^\s*identifier:' | grep -o '"[^"]*"' | tr -d '"' | head -1)
        if [ -z "$SYSTEM_RS_IDENTIFIER" ]; then
            SYSTEM_RS_IDENTIFIER=$(grep -A5 'system_resource_server:' "$config_file" 2>/dev/null | grep -E '^\s*identifier:' | awk -F':' '{gsub(/[[:space:]"'\'']/,""); s=""; for(i=2;i<=NF;i++) s=s (i>2?":":"") $i; print s}' | head -1)
        fi
    fi
    SYSTEM_RS_HANDLE=${SYSTEM_RS_HANDLE:-}
    SYSTEM_RS_IDENTIFIER=${SYSTEM_RS_IDENTIFIER:-}

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
SERVER_PID=""

# Cleanup function
cleanup() {
    if [ "$VERBOSE_MODE" = "true" ]; then
        echo ""
        echo -e "${CYAN}🛑 Stopping temporary server...${NC}"
    fi
    if [ -n "$SERVER_PID" ]; then
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi
    if [ -n "$CONSENT_PID" ]; then
        pkill -P $CONSENT_PID 2>/dev/null || true
        kill $CONSENT_PID 2>/dev/null || true
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
# Start the Server with Security Disabled
# ============================================================================

if [ "$VERBOSE_MODE" = "true" ]; then
    echo -e "${YELLOW}⚠️  Starting temporary server with security disabled...${NC}"
    echo ""
fi

# Export environment variable to skip security
export SKIP_SECURITY=true

if [ "$DEBUG_MODE" = "true" ]; then
    if [ "$VERBOSE_MODE" = "true" ]; then
        dlv exec --listen=:$DEBUG_PORT --headless=true --api-version=2 --accept-multiclient --continue ./${BINARY_NAME} &
    else
        dlv exec --listen=:$DEBUG_PORT --headless=true --api-version=2 --accept-multiclient --continue ./${BINARY_NAME} >/dev/null 2>&1 &
    fi
    SERVER_PID=$!
else
    if [ "$VERBOSE_MODE" = "true" ]; then
        ./${BINARY_NAME} &
    else
        ./${BINARY_NAME} >/dev/null 2>&1 &
    fi
    SERVER_PID=$!
fi

# ============================================================================
# Wait for Server to be Ready
# ============================================================================

if [ "$VERBOSE_MODE" = "true" ]; then
    echo -e "${BLUE}⏳ Waiting for server to be ready...${NC}"
fi
TIMEOUT=60
ELAPSED=0
RETRY_DELAY=2

while [ $ELAPSED -lt $TIMEOUT ]; do
    if curl -k -s "${BASE_URL}/health/readiness" > /dev/null 2>&1; then
        if [ "$VERBOSE_MODE" = "true" ]; then
            echo -e "${GREEN}✓ Server is ready${NC}"
            echo ""
        fi
        break
    fi
    sleep $RETRY_DELAY
    ELAPSED=$((ELAPSED + RETRY_DELAY))
    if [ "$VERBOSE_MODE" = "true" ]; then
        printf "."
    fi
done

if [ $ELAPSED -ge $TIMEOUT ]; then
    echo ""
    echo -e "${RED}❌ Server failed to start within ${TIMEOUT} seconds${NC}"
    echo -e "${RED}Expected server at: ${BASE_URL}${NC}"
    exit 1
fi

# ============================================================================
# Run Bootstrap Scripts
# ============================================================================

# Export variables to be used in scripts
export API_BASE="${BASE_URL}"
export PUBLIC_URL="${PUBLIC_URL}"
export SYSTEM_RS_HANDLE="${SYSTEM_RS_HANDLE}"
export SYSTEM_RS_IDENTIFIER="${SYSTEM_RS_IDENTIFIER}"
export SETUP_SILENT_MODE="${SILENT_MODE}"

# FD3 always points to the real terminal stdout.
# Quiet-mode result markers write to FD3 so they reach the terminal even
# when FD1 (normal stdout) is suppressed inside the bootstrap subshell.
exec 3>&1

# Check if bootstrap directory exists
if [ ! -d "$BOOTSTRAP_DIR" ]; then
    log_warning "Bootstrap directory not found: $BOOTSTRAP_DIR"
    log_info "Skipping bootstrap execution"
else
    log_info "========================================="
    log_info "${PRODUCT_NAME} Bootstrap Process"
    log_info "========================================="
    log_info "Bootstrap directory: $BOOTSTRAP_DIR"
    log_info "Fail fast: $BOOTSTRAP_FAIL_FAST"
    log_info "Started at: $(date)"
    echo ""

    # Collect all scripts from bootstrap directory
    SCRIPTS=()

    # Find scripts in bootstrap directory (exclude common.sh)
    if [ -d "$BOOTSTRAP_DIR" ]; then
        for script in "$BOOTSTRAP_DIR"/*.sh "$BOOTSTRAP_DIR"/*.bash; do
            [ ! -e "$script" ] && continue
            if [[ "$(basename "$script")" == "common.sh" ]]; then
                continue
            fi
            SCRIPTS+=("$script")
        done
    fi

    # Sort scripts by filename (numeric prefix determines order)
    IFS=$'\n' SORTED_SCRIPTS=($(printf '%s\n' "${SCRIPTS[@]}" | sort))
    unset IFS

    if [ ${#SORTED_SCRIPTS[@]} -eq 0 ]; then
        log_warning "No bootstrap scripts found"
    else
        log_info "Discovered ${#SORTED_SCRIPTS[@]} script(s)"
        echo ""

        # Execute scripts
        SCRIPT_COUNT=0
        SUCCESS_COUNT=0
        FAILED_COUNT=0
        SKIPPED_COUNT=0

        for script in "${SORTED_SCRIPTS[@]}"; do
            script_name=$(basename "$script")

            if [ "$SILENT_MODE" = "true" ]; then
                if [ "$script_name" = "01-default-resources.sh" ]; then
                    echo ""
                    echo "  Default resources"
                elif [ "$script_name" = "02-sample-resources.sh" ]; then
                    echo ""
                    echo "  Sample resources"
                fi
            fi

            # Skip if matches skip pattern
            if [ -n "$BOOTSTRAP_SKIP_PATTERN" ] && [[ "$script_name" =~ $BOOTSTRAP_SKIP_PATTERN ]]; then
                log_info "⊘ Skipping $script_name (matches skip pattern)"
                SKIPPED_COUNT=$((SKIPPED_COUNT + 1))
                continue
            fi

            # Skip if doesn't match only pattern
            if [ -n "$BOOTSTRAP_ONLY_PATTERN" ] && ! [[ "$script_name" =~ $BOOTSTRAP_ONLY_PATTERN ]]; then
                log_info "⊘ Skipping $script_name (doesn't match only pattern)"
                SKIPPED_COUNT=$((SKIPPED_COUNT + 1))
                continue
            fi

            # Check if executable
            if [ ! -x "$script" ]; then
                log_warning "$script_name is not executable, setting permissions..."
                chmod +x "$script" || {
                    log_error "Failed to make $script_name executable"
                    FAILED_COUNT=$((FAILED_COUNT + 1))
                    if [ "$BOOTSTRAP_FAIL_FAST" = "true" ]; then
                        exit 1
                    fi
                    continue
                }
            fi

            log_info "▶ Executing: $script_name"
            SCRIPT_COUNT=$((SCRIPT_COUNT + 1))

            # Execute script
            START_TIME=$(date +%s)

            set +e  # Temporarily disable exit on error to catch errors
            (
                set -e  # Re-enable in subshell to catch script errors
                # In quiet mode suppress all ordinary stdout so only explicit
                # FD3 writes (log_result_success/failure) reach the terminal.
                if [ "$SILENT_MODE" = "true" ]; then
                    exec 1>/dev/null
                fi
                source "$script"
            )
            EXIT_CODE=$?
            set -e  # Re-enable exit on error

            END_TIME=$(date +%s)
            DURATION=$((END_TIME - START_TIME))

            if [ $EXIT_CODE -eq 0 ]; then
                log_success "$script_name completed (${DURATION}s)"
                SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
            else
                if [ "$VERBOSE_MODE" = "true" ]; then
                    log_error "$script_name failed with exit code $EXIT_CODE (${DURATION}s)"
                fi
                FAILED_COUNT=$((FAILED_COUNT + 1))

                # Check if we should fail fast
                if [ "$BOOTSTRAP_FAIL_FAST" = "true" ]; then
                    if [ "$VERBOSE_MODE" = "true" ]; then
                        log_error "Stopping bootstrap (BOOTSTRAP_FAIL_FAST=true)"
                    fi
                    if [ "$SILENT_MODE" = "true" ]; then
                        echo ""
                        echo "========================================="
                        echo "❌ Setup failed."
                        echo "========================================="
                        echo ""
                    fi
                    exit 1
                fi
            fi
            echo ""
        done

        # Summary
        echo ""
        log_info "========================================="
        log_info "Bootstrap Summary"
        log_info "========================================="
        log_info "Total scripts discovered: ${#SORTED_SCRIPTS[@]}"
        log_info "Executed: $SCRIPT_COUNT"
        log_success "Successful: $SUCCESS_COUNT"

        if [ $FAILED_COUNT -gt 0 ] && [ "$VERBOSE_MODE" = "true" ]; then
            log_error "Failed: $FAILED_COUNT"
        fi

        if [ $SKIPPED_COUNT -gt 0 ]; then
            log_info "Skipped: $SKIPPED_COUNT"
        fi

        log_info "Completed at: $(date)"
        log_info "========================================="

        if [ $FAILED_COUNT -gt 0 ]; then
            exit 1
        fi

        log_success "Bootstrap completed successfully!"
    fi
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
    echo "Admin credentials:"
    echo "  URL:      ${PUBLIC_URL}/console"
    echo "  Username: admin"
    echo "  Password: admin"
    echo ""
    echo "Run ./start.sh to start ${PRODUCT_NAME}."
    echo ""
else
    echo "========================================="
    echo -e "${GREEN}✅ Setup completed successfully!${NC}"
    echo "========================================="
    echo ""
fi

# Cleanup will be called automatically via trap
exit 0
