#!/usr/bin/env bash
# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0
#
# run-e2e.sh - Local E2E test runner for ThunderID.
#
# Extracts the built distribution to tests/e2e/distribution/, runs setup.sh
# once to bootstrap default resources, then starts the server, imports sample-app
# resources via the import API (authenticated with OAuth2), and runs the Playwright test suite.
#
# Runs in two phases against two independently provisioned servers:
#   1. Everything tagged @wayfinder excluded, against a server with the sample apps imported.
#   2. Only @wayfinder specs, against a fresh server with just the E2E admin app imported. The
#      Wayfinder bundle those specs import replaces the server-wide CORS allowlist and default
#      resource server (full-replace import semantics, see server_config_import.go), so it must
#      not share a server with anything that depends on either setting; a fresh server also avoids
#      a `Customer` user-type name collision with the sample apps' own `Customer` type. This phase
#      also starts the standalone Wayfinder sample app (port 5173) that the tryout specs
#      (tests/wayfinder/**) drive a real browser against.
# Any extra arguments are passed to BOTH phases, with each phase's own --grep/--grep-invert applied
# last so it always wins - phase 1 never runs a @wayfinder spec and phase 2 never runs anything else.
#
# Usage:
#   ./run-e2e.sh [playwright-args...]
#
# Examples:
#   ./run-e2e.sh
#   ./run-e2e.sh --project=chromium
#   ./run-e2e.sh --grep @accessibility
#
# Requirements: curl, jq, python3, pnpm, lsof, unzip

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SAMPLE_APP_DIR="$PROJECT_ROOT/samples/apps/react-sdk-sample"
WAYFINDER_APP_DIR="$PROJECT_ROOT/samples/apps/wayfinder-sample/frontend"
SERVER_URL="${BASE_URL:-https://localhost:8090}"
SAMPLE_URL="${SAMPLE_APP_URL:-https://localhost:3000}"
WAYFINDER_URL="${WAYFINDER_APP_URL:-http://localhost:5173}"
_p="${SERVER_URL##*:}"; SERVER_PORT="${_p%%/*}"
_p="${SAMPLE_URL##*:}"; SAMPLE_PORT="${_p%%/*}"
_p="${WAYFINDER_URL##*:}"; WAYFINDER_PORT="${_p%%/*}"
unset _p

# Resolve the distribution zip for the current platform.
GO_OS=$(go env GOOS)
GO_ARCH=$(go env GOARCH)
[ "$GO_OS" = "darwin" ] && PKG_OS="macos" || PKG_OS="$GO_OS"
VERSION=$(sed 's/^v//' "$PROJECT_ROOT/version.txt")
DIST_FOLDER="thunderid-${VERSION}-${PKG_OS}-${GO_ARCH}"
DIST_HOME="$SCRIPT_DIR/distribution"
DIST_ZIP="$PROJECT_ROOT/target/dist/${DIST_FOLDER}.zip"

ADMIN_USER="${ADMIN_USERNAME:-admin}"
ADMIN_PASS="${ADMIN_PASSWORD:-admin}"
ADMIN_TOKEN=""

kill_port() {
    lsof -ti tcp:"$1" | xargs kill -9 2>/dev/null || true
}

wait_for_url() {
    local url="$1" label="$2" i=0
    echo "Waiting for $label at $url..."
    while [ $i -lt 60 ]; do
        if curl -skf "$url" > /dev/null 2>&1; then
            echo "$label is ready."
            return 0
        fi
        i=$((i + 1))
        sleep 2
    done
    echo "ERROR: $label did not become ready after 120s."
    return 1
}

cleanup() {
    echo "Cleaning up..."
    kill_port "$SAMPLE_PORT"
    kill_port "$WAYFINDER_PORT"
    kill_port "$SERVER_PORT"
    rm -rf "$DIST_HOME"
}
trap cleanup EXIT

# Extracts a fresh distribution into tests/e2e/distribution/, bootstraps it, and starts the
# server. Always starts from a clean unzip, so a stale writable-layer config (CORS, a user type)
# from a previous phase can never leak into the next.
start_fresh_server() {
    rm -rf "$DIST_HOME"
    if [ ! -f "$DIST_ZIP" ]; then
        echo "ERROR: Distribution zip not found at $DIST_ZIP. Run 'make build' first."
        exit 1
    fi
    echo "Extracting distribution to $DIST_HOME..."
    mkdir -p "$DIST_HOME"
    unzip -q "$DIST_ZIP" -d "$SCRIPT_DIR/distribution-tmp"
    mv "$SCRIPT_DIR/distribution-tmp/$DIST_FOLDER/"* "$DIST_HOME/"
    rm -rf "$SCRIPT_DIR/distribution-tmp"

    echo "Running setup..."
    (cd "$DIST_HOME" && ./setup.sh --admin-username "$ADMIN_USER" --admin-password "$ADMIN_PASS")

    echo "Starting ThunderID server..."
    (cd "$DIST_HOME" && ./start.sh) &
    wait_for_url "$SERVER_URL/health/liveness" "ThunderID server"
}

# Stops the server started by start_fresh_server and removes its distribution, so the next
# start_fresh_server call is not racing an orphaned process still holding the sqlite files.
stop_server() {
    echo "Stopping ThunderID server..."
    kill_port "$SERVER_PORT"
    local i=0
    while curl -skf "$SERVER_URL/health/liveness" > /dev/null 2>&1 && [ $i -lt 30 ]; do
        sleep 1
        i=$((i + 1))
    done
    if curl -skf "$SERVER_URL/health/liveness" > /dev/null 2>&1; then
        echo "ERROR: ThunderID server at $SERVER_URL did not stop within 30s."
        return 1
    fi
    rm -rf "$DIST_HOME"
}

# Obtains an admin token via OAuth2 auth code + PKCE (CONSOLE app, admin credentials) and sets the
# global ADMIN_TOKEN. Must be called plainly (never `ADMIN_TOKEN=$(mint_admin_token)` and never
# inside an `if`/`||`) - its `exit 1` calls only abort the right way when the function's own exit
# status is what `set -e` sees.
mint_admin_token() {
    echo "Obtaining admin token..."
    local CONSOLE_REDIRECT_URI="https://localhost:8090/console"
    local CODE_VERIFIER CODE_CHALLENGE
    CODE_VERIFIER=$(openssl rand -hex 32 | cut -c1-43)
    CODE_CHALLENGE=$(printf '%s' "$CODE_VERIFIER" | openssl dgst -sha256 -binary | openssl base64 -A | tr '+/' '-_' | tr -d '=')

    local headers_file
    headers_file=$(mktemp)
    curl -sk -o /dev/null -D "$headers_file" \
        -G "$SERVER_URL/oauth2/authorize" \
        --data-urlencode "client_id=CONSOLE" \
        --data-urlencode "redirect_uri=$CONSOLE_REDIRECT_URI" \
        --data-urlencode "scope=system" \
        --data-urlencode "resource=$SERVER_URL/mcp" \
        --data-urlencode "response_type=code" \
        --data-urlencode "code_challenge=$CODE_CHALLENGE" \
        --data-urlencode "code_challenge_method=S256"

    local LOCATION AUTH_ID EXEC_ID
    LOCATION=$(grep -i "^location:" "$headers_file" | tr -d '\r' | sed 's/^[Ll]ocation: //')
    rm -f "$headers_file"
    AUTH_ID=$(echo "$LOCATION" | sed -n 's/.*[?&]authId=\([^&]*\).*/\1/p')
    EXEC_ID=$(echo "$LOCATION" | sed -n 's/.*[?&]executionId=\([^&]*\).*/\1/p')

    if [ -z "$AUTH_ID" ] || [ -z "$EXEC_ID" ]; then
        echo "ERROR: Failed to parse authId/executionId from authorize redirect."
        echo "Location header: $LOCATION"
        exit 1
    fi

    # The console login flow runs an SSO check before the credentials prompt. This bootstrap is a
    # fresh, cookie-less login (no SSO session), so the first execute advances the flow past the
    # SSO check to the credentials prompt and mints a challenge token; the second submits the
    # admin credentials with that token.
    local PROMPT_RESP CHALLENGE_TOKEN
    PROMPT_RESP=$(curl -sk -X POST "$SERVER_URL/flow/execute" \
        -H "Content-Type: application/json" \
        -d "{\"executionId\": \"$EXEC_ID\"}")
    CHALLENGE_TOKEN=$(echo "$PROMPT_RESP" | jq -r '.challengeToken // empty' || echo "")

    if [ -z "$CHALLENGE_TOKEN" ]; then
        echo "ERROR: Flow execution did not return a challenge token."
        echo "Response: $PROMPT_RESP"
        exit 1
    fi

    local FLOW_RESP ASSERTION
    FLOW_RESP=$(curl -sk -X POST "$SERVER_URL/flow/execute" \
        -H "Content-Type: application/json" \
        -d "$(jq -n \
            --arg executionId "$EXEC_ID" \
            --arg challengeToken "$CHALLENGE_TOKEN" \
            --arg username "$ADMIN_USER" \
            --arg password "$ADMIN_PASS" \
            --arg action "action_001" \
            '{executionId: $executionId, challengeToken: $challengeToken, inputs: {username: $username, password: $password}, action: $action}')")
    ASSERTION=$(echo "$FLOW_RESP" | jq -r '.assertion // empty' || echo "")

    if [ -z "$ASSERTION" ]; then
        echo "ERROR: Flow execution did not return an assertion."
        echo "Response: $FLOW_RESP"
        exit 1
    fi

    local CALLBACK_RESP AUTH_CODE
    CALLBACK_RESP=$(curl -sk -X POST "$SERVER_URL/oauth2/auth/callback" \
        -H "Content-Type: application/json" \
        -d "{\"authId\": \"$AUTH_ID\", \"assertion\": \"$ASSERTION\"}")
    AUTH_CODE=$(echo "$CALLBACK_RESP" | jq -r '.redirect_uri // empty' | sed 's/.*[?&]code=\([^&]*\).*/\1/' || echo "")

    if [ -z "$AUTH_CODE" ]; then
        echo "ERROR: OAuth2 callback did not return an authorization code."
        echo "Response: $CALLBACK_RESP"
        exit 1
    fi

    local TOKEN_RESP
    TOKEN_RESP=$(curl -sk -X POST "$SERVER_URL/oauth2/token" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        --data-urlencode "grant_type=authorization_code" \
        --data-urlencode "code=$AUTH_CODE" \
        --data-urlencode "redirect_uri=$CONSOLE_REDIRECT_URI" \
        --data-urlencode "client_id=CONSOLE" \
        --data-urlencode "resource=$SERVER_URL/mcp" \
        --data-urlencode "code_verifier=$CODE_VERIFIER")
    ADMIN_TOKEN=$(echo "$TOKEN_RESP" | jq -r '.access_token // empty' || echo "")

    if [ -z "$ADMIN_TOKEN" ]; then
        echo "ERROR: Failed to obtain admin access token."
        echo "Response: $TOKEN_RESP"
        exit 1
    fi
}

# Converts a KEY=VALUE .env-style file into a JSON object on stdout, or "{}" if the file is absent.
env_to_json() {
    local vars_file="$1"
    if [ ! -f "$vars_file" ]; then
        echo "{}"
        return
    fi
    python3 - "$vars_file" <<'PYEOF'
import sys, json
pairs = {}
for line in open(sys.argv[1]):
    line = line.rstrip()
    if '=' in line and not line.startswith('#'):
        k, _, v = line.partition('=')
        try:
            pairs[k.strip()] = json.loads(v.strip())
        except (ValueError, json.JSONDecodeError):
            pairs[k.strip()] = v.strip()
print(json.dumps(pairs))
PYEOF
}

# Imports one declarative config file via POST /import, upsert enabled. `vars_file`, if given, is
# converted to the request's `variables` map. Must be called plainly, same reasoning as
# mint_admin_token: its `exit 1` calls need to reach `set -e` directly.
import_config() {
    local config="$1" vars_file="${2:-}" label="${3:-$1}"
    local vars_json="{}"
    [ -n "$vars_file" ] && vars_json=$(env_to_json "$vars_file")

    local content
    content=$(jq -Rs . < "$config")
    local response_file
    response_file=$(mktemp)
    local http_status
    http_status=$(curl -sk -o "$response_file" -w "%{http_code}" \
        -X POST "$SERVER_URL/import" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "{\"content\": $content, \"variables\": $vars_json, \"options\": {\"upsert\": true}}")

    if [ "$http_status" != "200" ]; then
        echo "ERROR: import returned HTTP $http_status for $label:"
        cat "$response_file"; echo ""; rm -f "$response_file"; exit 1
    fi
    local failed_count
    failed_count=$(jq -r '.summary.failed // 0' "$response_file" || echo "0")
    if [ "$failed_count" != "0" ]; then
        echo "ERROR: import of $label had $failed_count failed resource(s):"
        cat "$response_file"; echo ""; rm -f "$response_file"; exit 1
    fi
    rm -f "$response_file"
    echo "  Imported $label."
}

# Builds (if not already built) and starts the sample app used by the non-Wayfinder specs.
start_sample_app() {
    echo "Setting up sample app..."
    cd "$SAMPLE_APP_DIR"
    if [ ! -d "dist" ]; then
        echo "Building sample app..."
        pnpm install --frozen-lockfile
        pnpm run build
    fi
    pnpm start &
    wait_for_url "$SAMPLE_URL" "Sample app"
    cd "$SCRIPT_DIR"
}

# Starts the standalone Wayfinder sample app (its own npm workspace, not pnpm) for the
# tests/wayfinder/** tryout specs. A plain Vite dev server, no build step needed.
start_wayfinder_app() {
    echo "Setting up Wayfinder sample app..."
    cd "$WAYFINDER_APP_DIR"
    [ -f .env ] || cp .env.example .env
    [ -d node_modules ] || npm install
    npm run dev &
    wait_for_url "$WAYFINDER_URL" "Wayfinder sample app"
    cd "$SCRIPT_DIR"
}

# Creates a default .env if missing and exports the values resolved for this run, which always
# take precedence over .env: dotenv.config() in playwright.config.ts does not override
# already-set process.env values.
setup_env() {
    if [ ! -f "$SCRIPT_DIR/.env" ]; then
        echo "Creating default .env for E2E tests..."
        cp "$SCRIPT_DIR/defaults.env" "$SCRIPT_DIR/.env"
    fi
    export BASE_URL="$SERVER_URL"
    export SERVER_URL="$SERVER_URL"
    export ADMIN_USERNAME="$ADMIN_USER"
    export ADMIN_PASSWORD="$ADMIN_PASS"
    export SAMPLE_APP_URL="$SAMPLE_URL"
    export WAYFINDER_APP_URL="$WAYFINDER_URL"
}

# Runs "$@" without tripping `set -e` on a non-zero exit, returning its exit code instead. A
# failure in one phase must not abort the script before the other phase's server has even been
# provisioned - call as `run_phase ... || rc=$?`, never bare.
run_phase() {
    set +e
    "$@"
    local rc=$?
    set -e
    return $rc
}

# Abort if a server is already running to avoid silently disrupting it.
if curl -sk "$SERVER_URL/health/liveness" > /dev/null 2>&1; then
    echo "A ThunderID server is already running at $SERVER_URL."
    echo "Stop it before running this script, which needs to manage the server lifecycle."
    echo "To run tests against an already-running server: cd tests/e2e && npx playwright test"
    exit 1
fi

# Phase 1: everything except @wayfinder, against a server with the sample apps imported.
start_fresh_server
mint_admin_token

echo "Importing declarative resources..."
for sample in vanilla-sample react-sdk-sample; do
    config="$PROJECT_ROOT/samples/apps/$sample/thunderid-config/thunderid-config.yaml"
    vars_file="$PROJECT_ROOT/samples/apps/$sample/thunderid-config/thunderid.env"

    # vanilla-sample keeps its default config under a 'basic/' subdirectory.
    if [ ! -f "$config" ]; then
        config="$PROJECT_ROOT/samples/apps/$sample/thunderid-config/basic/thunderid-config.yaml"
        vars_file="$PROJECT_ROOT/samples/apps/$sample/thunderid-config/basic/thunderid.env"
    fi

    [ -f "$config" ] || { echo "  No config for $sample, skipping."; continue; }
    import_config "$config" "$vars_file" "$sample"
done
import_config "$SCRIPT_DIR/thunderid-config.yaml" "" "E2E admin app"
import_config "$SCRIPT_DIR/thunderid-config-sample-apps.yaml" "" "E2E sample-app infrastructure"

start_sample_app
setup_env
cd "$SCRIPT_DIR"
pnpm install --frozen-lockfile

echo "Running Playwright E2E tests (core)..."
rc1=0
run_phase npx playwright test "$@" --grep-invert @wayfinder --pass-with-no-tests || rc1=$?

# Phase 2: only @wayfinder, against a fresh server with just the E2E admin app imported. Run
# regardless of phase 1's result - this is a different server, so phase 1 tells us nothing about
# it, and re-running phase 1 to see phase 2 would cost the whole run again.
kill_port "$SAMPLE_PORT"
stop_server
start_fresh_server
mint_admin_token
import_config "$SCRIPT_DIR/thunderid-config.yaml" "" "E2E admin app"
start_wayfinder_app

# Own report paths, or this phase's run would delete phase 1's reports and traces outright.
echo "Running Playwright E2E tests (wayfinder)..."
rc2=0
PLAYWRIGHT_BLOB_OUTPUT_DIR=blob-report-wayfinder \
PLAYWRIGHT_HTML_OUTPUT_DIR=playwright-report-wayfinder \
PLAYWRIGHT_JSON_OUTPUT_FILE=test-results-wayfinder/test-results.json \
PLAYWRIGHT_JUNIT_OUTPUT_FILE=test-results-wayfinder/junit.xml \
run_phase npx playwright test "$@" --output=test-results-wayfinder --grep @wayfinder --pass-with-no-tests || rc2=$?

if [ $rc1 -ne 0 ] || [ $rc2 -ne 0 ]; then
    [ $rc1 -ne 0 ] && echo "ERROR: core phase failed (exit $rc1)."
    [ $rc2 -ne 0 ] && echo "ERROR: wayfinder phase failed (exit $rc2)."
    exit 1
fi
