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
# Pass --phase=1 or --phase=2 to run only that phase; omit it to run both, as above.
#
# When both phases run, their blob reports are merged into one HTML report at the default
# playwright-report/ location afterward, so `playwright show-report` shows every test. A
# --phase=1|2 run leaves that phase's own report untouched (playwright-report/ or
# playwright-report-wayfinder/ respectively) and does not merge.
#
# Usage:
#   ./run-e2e.sh [--phase=1|2] [playwright-args...]
#
# Examples:
#   ./run-e2e.sh
#   ./run-e2e.sh --project=chromium
#   ./run-e2e.sh --grep @accessibility
#   ./run-e2e.sh --phase=2
#
# Requirements: curl, jq, python3, pnpm, lsof, unzip

set -euo pipefail
# Job control, so each "cmd &" below gets its own process group (see kill_job) instead of
# sharing this script's - without it, killing a group would kill run-e2e.sh itself. Each start
# is followed by `disown` so job control doesn't also report "Terminated: 15" for it at cleanup;
# the tracked PID is what kill_job uses, so nothing depends on the job table entry.
set -m

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SAMPLE_APP_DIR="$PROJECT_ROOT/samples/apps/react-sdk-sample"
WAYFINDER_APP_DIR="$PROJECT_ROOT/samples/apps/wayfinder-sample/frontend"
WAYFINDER_API_DIR="$PROJECT_ROOT/samples/apps/wayfinder-sample/backend"
WAYFINDER_AGENT_DIR="$PROJECT_ROOT/samples/apps/wayfinder-sample/ai-agent"
SMTP_SERVER_DIR="$PROJECT_ROOT/samples/apps/wayfinder-sample/smtp-server"
SERVER_URL="${BASE_URL:-https://localhost:8090}"
SAMPLE_URL="${SAMPLE_APP_URL:-https://localhost:3000}"
WAYFINDER_URL="${WAYFINDER_APP_URL:-http://localhost:5173}"
WAYFINDER_API_URL="${WAYFINDER_API_URL:-http://localhost:8787}"
WAYFINDER_AGENT_URL="${WAYFINDER_AGENT_URL:-http://localhost:8790}"
MOCK_SMTP_INBOX_URL="${MOCK_SMTP_INBOX_URL:-http://localhost:8788}"
# Extracts the port from a URL, falling back to the scheme's default port (443/80) when the URL
# has none (e.g. a portless override like https://myserver.example.com).
url_port() {
    local rest="${1#*://}"
    rest="${rest%%/*}"
    if [[ "$rest" == *:* ]]; then
        echo "${rest##*:}"
    elif [[ "$1" == https://* ]]; then
        echo 443
    else
        echo 80
    fi
}
SERVER_PORT=$(url_port "$SERVER_URL")
SAMPLE_PORT=$(url_port "$SAMPLE_URL")
WAYFINDER_PORT=$(url_port "$WAYFINDER_URL")
WAYFINDER_API_PORT=$(url_port "$WAYFINDER_API_URL")
WAYFINDER_AGENT_PORT=$(url_port "$WAYFINDER_AGENT_URL")
MOCK_SMTP_INBOX_PORT=$(url_port "$MOCK_SMTP_INBOX_URL")

# Pull --phase=1|2 out of the arguments, leaving the rest to pass through to Playwright unchanged.
PHASE=""
PLAYWRIGHT_ARGS=()
for arg in "$@"; do
    case "$arg" in
        --phase=1|--phase=2)
            PHASE="${arg#--phase=}"
            ;;
        --phase=*)
            echo "ERROR: --phase must be 1 or 2 (got '${arg#--phase=}')." >&2
            exit 1
            ;;
        *)
            PLAYWRIGHT_ARGS+=("$arg")
            ;;
    esac
done
if [ ${#PLAYWRIGHT_ARGS[@]} -gt 0 ]; then
    set -- "${PLAYWRIGHT_ARGS[@]}"
else
    set --
fi

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

# $! of each background server, set right after it's started (see the start_* functions below).
# Tracking these directly - rather than only discovering them later via lsof on their port -
# means kill_job can still reap a server that crashed or hung before ever binding its port.
SERVER_PID=""
SAMPLE_APP_PID=""
WAYFINDER_APP_PID=""
WAYFINDER_API_PID=""
WAYFINDER_AGENT_PID=""
MOCK_SMTP_PID=""

# Sends SIGTERM to the given PID's process group (reaping a supervisor like `node --watch`
# along with the worker it forked, not just the worker), waits up to 5s for it to exit (the Go
# server and the Wayfinder sample apps all handle SIGTERM), then SIGKILLs anything still standing.
kill_pgid_of() {
    local pid="$1" pgid
    [ -z "$pid" ] && return 0
    pgid=$(ps -o pgid= -p "$pid" 2>/dev/null | tr -d ' ') || true
    [ -z "$pgid" ] && pgid="$pid"
    kill -TERM -- -"$pgid" 2>/dev/null || true
    local i=0
    while [ $i -lt 10 ] && kill -0 -- -"$pgid" 2>/dev/null; do
        sleep 0.5
        i=$((i + 1))
    done
    kill -9 -- -"$pgid" 2>/dev/null || true
}

# Stops a server we started: its tracked PID (catches it even if it crashed or hung before ever
# binding its port), then whatever is still listening on the port (catches a stale process left
# over from an earlier, differently-torn-down run - pid may be empty if we never started it).
kill_job() {
    local pid="$1" port="$2" leftover
    kill_pgid_of "$pid"
    leftover=$(lsof -ti tcp:"$port" 2>/dev/null | head -1) || true
    kill_pgid_of "$leftover"
}

# Reads a single KEY=VALUE line from an .env-style file (ignoring comments), or empty if the file
# or key is absent. Unlike ADMIN_USERNAME/etc above, the LLM_* vars have no CI export step, so they
# must be readable straight out of .env - see setup_env below.
env_file_var() {
    local key="$1" file="$2"
    [ -f "$file" ] || return 0
    grep "^${key}=" "$file" | tail -1 | cut -d= -f2-
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
    kill_job "$SAMPLE_APP_PID" "$SAMPLE_PORT"
    kill_job "$WAYFINDER_APP_PID" "$WAYFINDER_PORT"
    kill_job "$WAYFINDER_API_PID" "$WAYFINDER_API_PORT"
    kill_job "$WAYFINDER_AGENT_PID" "$WAYFINDER_AGENT_PORT"
    kill_job "$MOCK_SMTP_PID" "$MOCK_SMTP_INBOX_PORT"
    kill_job "$SERVER_PID" "$SERVER_PORT"
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

    if [ "${1:-}" = "--with-mock-social" ]; then
        # Redirect the backend's Google and GitHub endpoints to the local mock servers used by the social
        # login E2E tests (see utils/mock-google-oidc-server.ts and utils/mock-github-oauth-server.ts),
        # without touching the checked-in deployment.yaml. Production leaves these unset, so the real
        # providers are used unchanged. Ports must match MOCK_GOOGLE_BASE_URL/MOCK_GITHUB_BASE_URL in
        # defaults.env.
        MOCK_GOOGLE_BASE_URL="${MOCK_GOOGLE_BASE_URL:-http://localhost:8093}"
        MOCK_GITHUB_BASE_URL="${MOCK_GITHUB_BASE_URL:-http://localhost:8092}"
        if grep -q "identity_provider:" "$DIST_HOME/deployment.yaml"; then
            echo "deployment.yaml already has an identity_provider: block; leaving it as-is (mock URLs not appended)."
        else
            cat >> "$DIST_HOME/deployment.yaml" <<EOF

identity_provider:
  google_base_url: "$MOCK_GOOGLE_BASE_URL"
  github_base_url: "$MOCK_GITHUB_BASE_URL"
EOF
        fi
    fi

    # Run setup.sh to bootstrap default resources (admin user, console app config, etc.).
    echo "Running setup..."
    (cd "$DIST_HOME" && ./setup.sh --admin-username "$ADMIN_USER" --admin-password "$ADMIN_PASS")

    echo "Starting ThunderID server..."
    (cd "$DIST_HOME" && ./start.sh) &
    SERVER_PID=$!
    disown
    wait_for_url "$SERVER_URL/health/liveness" "ThunderID server"
}

# Stops the server started by start_fresh_server and removes its distribution, so the next
# start_fresh_server call is not racing an orphaned process still holding the sqlite files.
stop_server() {
    echo "Stopping ThunderID server..."
    kill_job "$SERVER_PID" "$SERVER_PORT"
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
    local CONSOLE_REDIRECT_URI="$SERVER_URL/console"
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
    LOCATION=$(grep -i "^location:" "$headers_file" | tr -d '\r' | sed 's/^[Ll]ocation: //' || echo "")
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
    AUTH_CODE=$(echo "$CALLBACK_RESP" | jq -r '.redirect_uri // empty' | sed -n 's/.*[?&]code=\([^&]*\).*/\1/p' || echo "")

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
    pnpm --silent start &
    SAMPLE_APP_PID=$!
    disown
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
    npm --silent run dev &
    WAYFINDER_APP_PID=$!
    disown
    wait_for_url "$WAYFINDER_URL" "Wayfinder sample app"
    cd "$SCRIPT_DIR"
}

# Starts the Wayfinder sample's backend (booking API + MCP server) that the AI agent tryout specs
# need in addition to the frontend above. Only started when LLM_API_KEY is set - see phase 2 below.
start_wayfinder_backend() {
    echo "Setting up Wayfinder backend..."
    cd "$WAYFINDER_API_DIR"
    [ -f .env ] || cp .env.example .env
    [ -d node_modules ] || npm install
    npm --silent run dev &
    WAYFINDER_API_PID=$!
    disown
    wait_for_url "$WAYFINDER_API_URL/health" "Wayfinder backend"
    cd "$SCRIPT_DIR"
}

# Starts the Wayfinder sample's AI agent service. The E2E run owns the agent's LLM config: the
# LLM_PROVIDER/LLM_API_KEY/MODEL_NAME resolved in setup_env always replace whatever the agent's
# .env (or .env.example it was copied from) carried. Only called when LLM_API_KEY is set - see
# phase 2 below.
start_wayfinder_agent() {
    echo "Setting up Wayfinder AI agent..."
    cd "$WAYFINDER_AGENT_DIR"
    [ -f .env ] || cp .env.example .env
    grep -vE '^[[:space:]]*#?[[:space:]]*(LLM_PROVIDER|LLM_API_KEY|MODEL_NAME)=' .env > .env.e2e
    {
        echo "LLM_PROVIDER=${LLM_PROVIDER}"
        echo "LLM_API_KEY=${LLM_API_KEY}"
        echo "MODEL_NAME=${MODEL_NAME}"
    } >> .env.e2e
    mv .env.e2e .env
    [ -d node_modules ] || npm install
    npm --silent run dev &
    WAYFINDER_AGENT_PID=$!
    disown
    wait_for_url "$WAYFINDER_AGENT_URL/health" "Wayfinder AI agent"
    cd "$SCRIPT_DIR"
}

# Starts the mock SMTP server + web inbox that TC006 reads the password-reset email from (see
# mock-email.page.ts). Defaults (127.0.0.1:2525 SMTP, 127.0.0.1:8788 inbox) match the server
# distribution's deployment.yaml email.smtp settings, so no config changes are needed. Mirrors the
# "Start Wayfinder Mock SMTP Server" step in .github/workflows/pr-builder.yml.
start_mock_smtp_server() {
    echo "Setting up Wayfinder mock SMTP server..."
    cd "$SMTP_SERVER_DIR"
    [ -d node_modules ] || npm install
    npm run build
    node src/index.js &
    MOCK_SMTP_PID=$!
    disown
    wait_for_url "$MOCK_SMTP_INBOX_URL/health" "Wayfinder mock SMTP inbox"
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
    # Gates whether the Wayfinder AI agent tests run at all (see .env.example); fall back to .env
    # since, unlike the vars above, it has no CI export step of its own.
    export LLM_PROVIDER="${LLM_PROVIDER:-$(env_file_var LLM_PROVIDER "$SCRIPT_DIR/.env")}"
    export LLM_PROVIDER="${LLM_PROVIDER:-anthropic}"
    export LLM_API_KEY="${LLM_API_KEY:-$(env_file_var LLM_API_KEY "$SCRIPT_DIR/.env")}"
    # Empty is fine: the agent falls back to its per-provider default model.
    export MODEL_NAME="${MODEL_NAME:-$(env_file_var MODEL_NAME "$SCRIPT_DIR/.env")}"
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

setup_env
cd "$SCRIPT_DIR"
pnpm install --frozen-lockfile

rc1=0
rc2=0

if [ -z "$PHASE" ] || [ "$PHASE" = "1" ]; then
    # Phase 1: everything except @wayfinder, against a server with the sample apps imported.
    start_fresh_server --with-mock-social
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

    echo "Running Playwright E2E tests (core)..."
    run_phase npx playwright test "$@" --grep-invert @wayfinder --pass-with-no-tests || rc1=$?
fi

if [ -z "$PHASE" ] || [ "$PHASE" = "2" ]; then
    # Phase 2: only @wayfinder, against a fresh server with just the E2E admin app imported. Run
    # regardless of phase 1's result - this is a different server, so phase 1 tells us nothing about
    # it, and re-running phase 1 to see phase 2 would cost the whole run again.
    if [ -z "$PHASE" ]; then
        # Only phase 1's server/sample-app need tearing down when phase 1 just ran in this
        # invocation - a --phase=2 run never started them.
        kill_job "$SAMPLE_APP_PID" "$SAMPLE_PORT"
        stop_server
    fi
    start_fresh_server
    mint_admin_token
    import_config "$SCRIPT_DIR/thunderid-config.yaml" "" "E2E admin app"
    start_wayfinder_app
    start_mock_smtp_server

    # The AI agent tryout specs need a real LLM key (see tests/e2e/.env.example); without one,
    # leave the backend and agent stopped so those specs skip cleanly instead of failing.
    if [ -n "$LLM_API_KEY" ]; then
        # Import the Wayfinder bundle now, ahead of wayfinder-sample-setup.spec.ts's own import
        # step: start_wayfinder_agent makes the agent authenticate against WAYFINDER-CONCIERGE at
        # startup (before any Playwright test runs), so that client must already exist by then.
        # The setup spec still re-imports the same (upsert:true) bundle through the console UI
        # later - that's what actually tests the import feature itself, and it is what every other
        # @wayfinder spec depends on (see the wayfinder-setup project in playwright.config.ts), so
        # nothing here is needed when the agent does not start.
        import_config "$PROJECT_ROOT/samples/apps/wayfinder-sample/thunderid-config/redirect/thunderid-config.yaml" \
            "$PROJECT_ROOT/samples/apps/wayfinder-sample/thunderid-config/redirect/thunderid.env" \
            "Wayfinder sample"
        start_wayfinder_backend
        start_wayfinder_agent
    else
        echo "LLM_API_KEY not set - skipping Wayfinder AI agent tryout tests."
    fi

    # Own report paths, or this phase's run would delete phase 1's reports and traces outright.
    echo "Running Playwright E2E tests (wayfinder)..."
    PLAYWRIGHT_BLOB_OUTPUT_DIR=blob-report-wayfinder \
    PLAYWRIGHT_HTML_OUTPUT_DIR=playwright-report-wayfinder \
    PLAYWRIGHT_JSON_OUTPUT_FILE=test-results-wayfinder/test-results.json \
    PLAYWRIGHT_JUNIT_OUTPUT_FILE=test-results-wayfinder/junit.xml \
    run_phase npx playwright test "$@" --output=test-results-wayfinder --grep @wayfinder --pass-with-no-tests || rc2=$?
fi

if [ -z "$PHASE" ]; then
    # Both phases just ran in this invocation, each into its own blob dir (see the comment above
    # phase 2's run). Merge them into one HTML report at the default playwright-report/ location,
    # so `playwright show-report` (no args) shows every test, not just phase 1's.
    echo "Merging core and wayfinder reports..."
    rm -rf blob-report-combined
    mkdir -p blob-report-combined
    cp blob-report/*.zip blob-report-wayfinder/*.zip blob-report-combined/
    npx playwright merge-reports --reporter html blob-report-combined
    rm -rf blob-report-combined
fi

if [ $rc1 -ne 0 ] || [ $rc2 -ne 0 ]; then
    [ $rc1 -ne 0 ] && echo "ERROR: core phase failed (exit $rc1)."
    [ $rc2 -ne 0 ] && echo "ERROR: wayfinder phase failed (exit $rc2)."
    exit 1
fi
