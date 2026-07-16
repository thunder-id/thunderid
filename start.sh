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

# Default settings
PRODUCT_NAME="ThunderID"
PRODUCT_NAME_LOWERCASE="$(echo "$PRODUCT_NAME" | tr '[:upper:]' '[:lower:]')"
BINARY_NAME="${PRODUCT_NAME_LOWERCASE}"
BACKEND_PORT=${BACKEND_PORT:-8090}
DEBUG_PORT=${DEBUG_PORT:-2345}
DEBUG_MODE=${DEBUG_MODE:-false}
WITH_CONSENT=${WITH_CONSENT:-true}
RESOURCES_FILE=""
ENV_FILE=""
BOOTSTRAP_MODE=false
BOOTSTRAP_AND_SERVE=false
BOOTSTRAP_EXTRA_ARGS=()
ADMIN_USERNAME_FLAG_SET=false
ADMIN_PASSWORD_FLAG_SET=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --debug)
            DEBUG_MODE=true
            shift
            ;;
        --debug-port)
            DEBUG_PORT="$2"
            shift 2
            ;;
        --port)
            BACKEND_PORT="$2"
            shift 2
            ;;
        --without-consent)
            WITH_CONSENT=false
            shift
            ;;
        --bootstrap)
            # Run the in-process bootstrap one-shot (create default resources) instead
            # of starting the long-running server, then exit.
            BOOTSTRAP_MODE=true
            shift
            ;;
        --bootstrap-and-serve)
            # Run the in-process bootstrap one-shot first, then start the long-running
            # server (convenient for local/dev: seed once, then serve).
            BOOTSTRAP_AND_SERVE=true
            shift
            ;;
        --console-redirect-uris)
            if [ $# -lt 2 ] || [ -z "$2" ]; then
                echo "Error: --console-redirect-uris requires a value"
                exit 1
            fi
            BOOTSTRAP_EXTRA_ARGS+=(--console-redirect-uris "$2")
            shift 2
            ;;
        --admin-username)
            if [ $# -lt 2 ] || [ -z "$2" ]; then
                echo "Error: --admin-username requires a value"
                exit 1
            fi
            BOOTSTRAP_EXTRA_ARGS+=(--admin-username "$2")
            ADMIN_USERNAME_FLAG_SET=true
            shift 2
            ;;
        --admin-password)
            if [ $# -lt 2 ] || [ -z "$2" ]; then
                echo "Error: --admin-password requires a value"
                exit 1
            fi
            BOOTSTRAP_EXTRA_ARGS+=(--admin-password "$2")
            ADMIN_PASSWORD_FLAG_SET=true
            shift 2
            ;;
        --env)
            ENV_FILE="$2"
            shift 2
            ;;
        --help)
            echo "${PRODUCT_NAME} Server Startup Script"
            echo ""
            echo "Usage: $0 [resources_file] [options]"
            echo ""
            echo "Arguments:"
            echo "  resources_file       Path to exported resources YAML file (optional)"
            echo ""
            echo "Options:"
            echo "  --env FILE           Path to env file with KEY=VALUE variables"
            echo "  --debug              Enable debug mode with remote debugging"
            echo "  --port PORT          Set application port (default: 8090)"
            echo "  --debug-port PORT    Set debug port (default: 2345)"
            echo "  --without-consent    Disable the bundled consent server"
            echo "  --bootstrap          Create default resources in-process, then exit (used by setup.sh)"
            echo "  --bootstrap-and-serve Create default resources in-process, then start the server"
            echo "  --admin-username VALUE  Username for the default admin user (--bootstrap-and-serve only)"
            echo "                          Optional; falls back to ADMIN_USERNAME env var, then defaults to 'admin'"
            echo "  --admin-password VALUE  Password for the default admin user (--bootstrap-and-serve only)"
            echo "                          Falls back to ADMIN_PASSWORD env var; bootstrap fails if neither is set"
            echo "  --help               Show this help message"
            echo ""
            echo "First-Time Setup:"
            echo "  For initial setup, use the setup script:"
            echo "    ./setup.sh"
            echo ""
            echo "  Then start the server normally:"
            echo "    ./start.sh"
            echo ""
            echo "Examples:"
            echo "  $0                                    Start server normally"
            echo "  $0 --debug                            Start in debug mode"
            echo "  $0 --port 9090                        Start on custom port"
            echo "  $0 cloud.yml --env my.env             Start with exported resources and env"
            exit 0
            ;;
        -*)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
        *)
            if [[ -z "$RESOURCES_FILE" ]]; then
                RESOURCES_FILE="$1"
            else
                echo "Unknown argument: $1"
                echo "Use --help for usage information"
                exit 1
            fi
            shift
            ;;
    esac
done

# --admin-username/--admin-password only make sense when bootstrapping happens in this
# invocation, and specifically only for --bootstrap-and-serve — --bootstrap is the
# seed-only mode setup.sh drives internally via environment variables, not flags.
if [[ "$ADMIN_USERNAME_FLAG_SET" == "true" || "$ADMIN_PASSWORD_FLAG_SET" == "true" ]] && [ "$BOOTSTRAP_AND_SERVE" != "true" ]; then
    echo "Error: --admin-username/--admin-password are only valid together with --bootstrap-and-serve"
    exit 1
fi

# Resolve relative paths to absolute before the working directory potentially changes.
if [[ -n "$RESOURCES_FILE" && "$RESOURCES_FILE" != /* ]]; then
    RESOURCES_FILE="$(pwd)/$RESOURCES_FILE"
fi
if [[ -n "$ENV_FILE" && "$ENV_FILE" != /* ]]; then
    ENV_FILE="$(pwd)/$ENV_FILE"
fi

set -e  # Exit immediately if a command exits with a non-zero status

# Check for port conflicts
check_port() {
    local port=$1
    local port_name=$2
    if lsof -ti tcp:$port >/dev/null 2>&1; then
        echo ""
        echo "❌ Port $port is already in use"
        echo "   $port_name cannot start because another process is using port $port"
        echo ""
        echo "💡 To find the process using this port:"
        echo "   lsof -i tcp:$port"
        echo ""
        echo "💡 To stop the process:"
        echo "   kill -9 \$(lsof -ti tcp:$port)"
        echo ""
        exit 1
    fi
}

# Load and export env vars from the env file.
load_env_file() {
    if [[ -z "$ENV_FILE" ]]; then
        return
    fi
    if [[ ! -f "$ENV_FILE" ]]; then
        echo "Error: env file not found: $ENV_FILE"
        exit 1
    fi
    echo "Loading environment from $ENV_FILE ..."
    while IFS= read -r line || [[ -n "$line" ]]; do
        [[ -z "$line" || "$line" == \#* ]] && continue
        line="${line%$'\r'}"
        if [[ "$line" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
            key="${BASH_REMATCH[1]}"
            value="${BASH_REMATCH[2]}"
            if [[ "$value" == \[* ]]; then
                # JSON array — expand into KEY_0, KEY_1, ...
                idx=0
                _json_tmp=$(mktemp)
                if ! python3 -c "import json,sys; [print(x) for x in json.loads(sys.argv[1])]" "$value" > "$_json_tmp" 2>&1; then
                    echo "Error: failed to parse JSON array for key '$key': $(cat "$_json_tmp")" >&2
                    rm -f "$_json_tmp"
                    exit 1
                fi
                while IFS= read -r elem; do
                    export "${key}_${idx}=${elem}"
                    ((idx++))
                done < "$_json_tmp"
                rm -f "$_json_tmp"
            else
                export "${key}=${value}"
            fi
        fi
    done < "$ENV_FILE"
}

# Check if ports are available. The bootstrap one-shot does not bind the server
# port, so the check is skipped in that mode.
if [ "$BOOTSTRAP_MODE" != "true" ]; then
    check_port "$BACKEND_PORT" "${PRODUCT_NAME} server"
    if [ "$DEBUG_MODE" = "true" ]; then
        check_port "$DEBUG_PORT" "Debug server"
    fi
fi

# Check if Delve is available for debug mode
if [ "$DEBUG_MODE" = "true" ]; then
    # Check for dlv in PATH
    if ! command -v dlv &> /dev/null; then
        echo "❌ Debug mode requires Delve debugger"
        echo ""
        echo "💡 Install Delve using:"
        echo "   go install github.com/go-delve/delve/cmd/dlv@latest"
        echo ""
        echo "🔧 Add Delve to PATH"
        echo ""
        echo "🔧 After installation, run: $0 --debug"
        exit 1
    fi
fi

# Load env vars before starting the binary so substitution works in resource files.
if [[ -n "$ENV_FILE" ]]; then
    load_env_file
fi

# Cleanup function
CONSENT_PID=""
SERVER_PID=""
cleanup() {
    echo -e "\n🛑 Stopping server..."
    if [ -n "$SERVER_PID" ]; then
        kill $SERVER_PID 2>/dev/null || true
    fi
    if [ -n "$CONSENT_PID" ]; then
        pkill -P $CONSENT_PID 2>/dev/null || true
        kill $CONSENT_PID 2>/dev/null || true
    fi
}
trap cleanup SIGINT SIGTERM EXIT

# Start consent server if enabled
CONSENT_SERVER_PORT="${CONSENT_SERVER_PORT:-9090}"
if [ "$WITH_CONSENT" = "true" ]; then
    CONSENT_SCRIPT="$(dirname "$0")/consent/start.sh"
    if [ ! -x "$CONSENT_SCRIPT" ]; then
        echo "Error: Consent server is enabled but consent/start.sh is missing or not executable"
        exit 1
    fi
    echo "Starting Consent Server..."
    (cd "$(dirname "$0")/consent" && ./start.sh) &
    CONSENT_PID=$!
    CONSENT_TIMEOUT=30
    CONSENT_ELAPSED=0
    while [ $CONSENT_ELAPSED -lt $CONSENT_TIMEOUT ]; do
        if ! kill -0 "$CONSENT_PID" 2>/dev/null; then
            echo "Error: Consent server process exited unexpectedly"
            exit 1
        fi
        if curl -s -f "http://localhost:${CONSENT_SERVER_PORT}/health/readiness" > /dev/null 2>&1; then
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
fi

# Bootstrap: create the default resources in-process. In --bootstrap mode this is the
# only action (exit afterwards). In --bootstrap-and-serve mode it runs first and then
# the long-running server is started below. The consent server started above stays up
# for the bootstrap (entity-type creation depends on it) — and, in bootstrap-and-serve
# mode, for the server too.
if [ "$BOOTSTRAP_MODE" = "true" ] || [ "$BOOTSTRAP_AND_SERVE" = "true" ]; then
    echo "⚡ Running ${PRODUCT_NAME} bootstrap ..."
    if BACKEND_PORT=$BACKEND_PORT "./${BINARY_NAME}" bootstrap "${BOOTSTRAP_EXTRA_ARGS[@]}"; then
        echo "✅ Bootstrap completed."
    else
        echo "❌ Bootstrap failed."
        exit 1
    fi
    if [ "$BOOTSTRAP_MODE" = "true" ]; then
        exit 0
    fi
fi

# Run the Server
if [ "$DEBUG_MODE" = "true" ]; then
    echo "⚡ Starting ${PRODUCT_NAME} Server in DEBUG mode..."
    echo "📝 Application will run on: https://localhost:$BACKEND_PORT"
    echo "🐛 Remote debugger will listen on: localhost:$DEBUG_PORT"
    echo ""
    echo "💡 Connect using remote debugging configuration:"
    echo "   Host: 127.0.0.1, Port: $DEBUG_PORT"
    echo ""

    # Run debugger
    RESOURCES_ARGS=()
    [[ -n "$RESOURCES_FILE" ]] && RESOURCES_ARGS=(-resources "$RESOURCES_FILE")
    BACKEND_PORT=$BACKEND_PORT dlv exec "--listen=:${DEBUG_PORT}" --headless=true --api-version=2 --accept-multiclient --continue \
        "./${BINARY_NAME}" -- "${RESOURCES_ARGS[@]}" &
    SERVER_PID=$!
else
    echo "⚡ Starting ${PRODUCT_NAME} Server ..."

    RESOURCES_ARGS=()
    [[ -n "$RESOURCES_FILE" ]] && RESOURCES_ARGS=(-resources "$RESOURCES_FILE")
    BACKEND_PORT=$BACKEND_PORT "./${BINARY_NAME}" "${RESOURCES_ARGS[@]}" &
    SERVER_PID=$!
fi

# Status
echo ""
echo "🚀 Server running"
echo "Press Ctrl+C to stop the server."

# Wait for background processes
wait $SERVER_PID
