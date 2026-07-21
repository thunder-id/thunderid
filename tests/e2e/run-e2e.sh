#!/usr/bin/env bash
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
#
# run-e2e.sh - Local E2E test runner for ThunderID.
#
# Extracts the built distribution to tests/e2e/distribution/, runs setup.sh
# once to bootstrap default resources, then starts the server, imports sample-app
# resources via the import API (authenticated with OAuth2), and runs the Playwright test suite.
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
SERVER_URL="${BASE_URL:-https://localhost:8090}"
SAMPLE_URL="${SAMPLE_APP_URL:-https://localhost:3000}"
_p="${SERVER_URL##*:}"; SERVER_PORT="${_p%%/*}"
_p="${SAMPLE_URL##*:}"; SAMPLE_PORT="${_p%%/*}"
unset _p

# Resolve the distribution zip for the current platform.
GO_OS=$(go env GOOS)
GO_ARCH=$(go env GOARCH)
[ "$GO_OS" = "darwin" ] && PKG_OS="macos" || PKG_OS="$GO_OS"
VERSION=$(sed 's/^v//' "$PROJECT_ROOT/version.txt")
DIST_FOLDER="thunderid-${VERSION}-${PKG_OS}-${GO_ARCH}"
DIST_HOME="$SCRIPT_DIR/distribution"
DIST_ZIP="$PROJECT_ROOT/target/dist/${DIST_FOLDER}.zip"
SETUP_DONE_FLAG="$DIST_HOME/.setup-done"

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
    kill_port $SAMPLE_PORT
    kill_port $SERVER_PORT
    rm -rf "$DIST_HOME"
}
trap cleanup EXIT

# Abort if a server is already running to avoid silently disrupting it.
if curl -sk "$SERVER_URL/health/liveness" > /dev/null 2>&1; then
    echo "A ThunderID server is already running at $SERVER_URL."
    echo "Stop it before running this script, which needs to manage the server lifecycle."
    echo "To run tests against an already-running server: cd tests/e2e && npx playwright test"
    exit 1
fi

# Remove any leftover distribution from a previously interrupted run.
rm -rf "$DIST_HOME"

# 1. Extract distribution into tests/e2e/distribution/ if not already present.
if [ ! -d "$DIST_HOME" ]; then
    if [ ! -f "$DIST_ZIP" ]; then
        echo "ERROR: Distribution zip not found at $DIST_ZIP. Run 'make build' first."
        exit 1
    fi
    echo "Extracting distribution to $DIST_HOME..."
    mkdir -p "$DIST_HOME"
    unzip -q "$DIST_ZIP" -d "$SCRIPT_DIR/distribution-tmp"
    mv "$SCRIPT_DIR/distribution-tmp/$DIST_FOLDER/"* "$DIST_HOME/"
    rm -rf "$SCRIPT_DIR/distribution-tmp"
fi

ADMIN_USER="${ADMIN_USERNAME:-admin}"
ADMIN_PASS="${ADMIN_PASSWORD:-admin}"

# 2. Run setup.sh once to bootstrap default resources (admin user, console app config, etc.).
if [ ! -f "$SETUP_DONE_FLAG" ]; then
    echo "Running first-time setup..."
    (cd "$DIST_HOME" && ./setup.sh --admin-username "$ADMIN_USER" --admin-password "$ADMIN_PASS")
    touch "$SETUP_DONE_FLAG"
fi

# 3. Start server.
echo "Starting ThunderID server..."
(cd "$DIST_HOME" && ./start.sh) &
wait_for_url "$SERVER_URL/health/liveness" "ThunderID server"

# 4. Obtain an admin token via OAuth2 auth code + PKCE (CONSOLE app, admin credentials).
echo "Obtaining admin token..."
CONSOLE_REDIRECT_URI="https://localhost:8090/console"
CODE_VERIFIER=$(openssl rand -hex 32 | cut -c1-43)
CODE_CHALLENGE=$(printf '%s' "$CODE_VERIFIER" | openssl dgst -sha256 -binary | openssl base64 -A | tr '+/' '-_' | tr -d '=')

curl -sk -o /dev/null -D /tmp/authz_headers.txt \
    -G "$SERVER_URL/oauth2/authorize" \
    --data-urlencode "client_id=CONSOLE" \
    --data-urlencode "redirect_uri=$CONSOLE_REDIRECT_URI" \
    --data-urlencode "scope=system" \
    --data-urlencode "response_type=code" \
    --data-urlencode "code_challenge=$CODE_CHALLENGE" \
    --data-urlencode "code_challenge_method=S256"

LOCATION=$(grep -i "^location:" /tmp/authz_headers.txt | tr -d '\r' | sed 's/^[Ll]ocation: //')
AUTH_ID=$(echo "$LOCATION" | sed 's/.*[?&]authId=\([^&]*\).*/\1/')
EXEC_ID=$(echo "$LOCATION" | sed 's/.*[?&]executionId=\([^&]*\).*/\1/')

if [ -z "$AUTH_ID" ] || [ -z "$EXEC_ID" ]; then
    echo "ERROR: Failed to parse authId/executionId from authorize redirect."
    echo "Location header: $LOCATION"
    exit 1
fi

FLOW_RESP=$(curl -sk -X POST "$SERVER_URL/flow/execute" \
    -H "Content-Type: application/json" \
    -d "{\"executionId\": \"$EXEC_ID\", \"inputs\": {\"username\": \"$ADMIN_USER\", \"password\": \"$ADMIN_PASS\"}, \"action\": \"action_001\"}")
ASSERTION=$(echo "$FLOW_RESP" | python3 -c "import sys, json; print(json.load(sys.stdin).get('assertion', ''))" 2>/dev/null || echo "")

if [ -z "$ASSERTION" ]; then
    echo "ERROR: Flow execution did not return an assertion."
    echo "Response: $FLOW_RESP"
    exit 1
fi

CALLBACK_RESP=$(curl -sk -X POST "$SERVER_URL/oauth2/auth/callback" \
    -H "Content-Type: application/json" \
    -d "{\"authId\": \"$AUTH_ID\", \"assertion\": \"$ASSERTION\"}")
AUTH_CODE=$(echo "$CALLBACK_RESP" | python3 -c "
import sys, json, urllib.parse
data = json.load(sys.stdin)
uri = data.get('redirect_uri', '')
params = urllib.parse.parse_qs(urllib.parse.urlparse(uri).query)
print(params.get('code', [''])[0])
" 2>/dev/null || echo "")

if [ -z "$AUTH_CODE" ]; then
    echo "ERROR: OAuth2 callback did not return an authorization code."
    echo "Response: $CALLBACK_RESP"
    exit 1
fi

TOKEN_RESP=$(curl -sk -X POST "$SERVER_URL/oauth2/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    --data-urlencode "grant_type=authorization_code" \
    --data-urlencode "code=$AUTH_CODE" \
    --data-urlencode "redirect_uri=$CONSOLE_REDIRECT_URI" \
    --data-urlencode "client_id=CONSOLE" \
    --data-urlencode "code_verifier=$CODE_VERIFIER")
ADMIN_TOKEN=$(echo "$TOKEN_RESP" | python3 -c "import sys, json; print(json.load(sys.stdin).get('access_token', ''))" 2>/dev/null || echo "")

if [ -z "$ADMIN_TOKEN" ]; then
    echo "ERROR: Failed to obtain admin access token."
    echo "Response: $TOKEN_RESP"
    exit 1
fi

# 5. Import declarative resources for sample apps.
echo "Importing declarative resources..."
for sample in react-vanilla-sample react-sdk-sample; do
    config="$PROJECT_ROOT/samples/apps/$sample/thunderid-config/thunderid-config.yaml"
    vars_file="$PROJECT_ROOT/samples/apps/$sample/thunderid-config/thunderid.env"

    # react-vanilla-sample keeps its default config under a 'basic/' subdirectory.
    if [ ! -f "$config" ]; then
        config="$PROJECT_ROOT/samples/apps/$sample/thunderid-config/basic/thunderid-config.yaml"
        vars_file="$PROJECT_ROOT/samples/apps/$sample/thunderid-config/basic/thunderid.env"
    fi

    [ -f "$config" ] || { echo "  No config for $sample, skipping."; continue; }

    vars_json="{}"
    if [ -f "$vars_file" ]; then
        vars_json=$(python3 - "$vars_file" <<'PYEOF'
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
)
    fi

    content=$(jq -Rs . < "$config")
    http_status=$(curl -sk -o /tmp/import_response.json -w "%{http_code}" \
        -X POST "$SERVER_URL/import" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "{\"content\": $content, \"variables\": $vars_json, \"options\": {\"upsert\": true}}")

    if [ "$http_status" != "200" ]; then
        echo "  ERROR: import returned HTTP $http_status for $sample:"
        cat /tmp/import_response.json; echo ""; exit 1
    fi
    failed_count=$(python3 -c "
import sys, json
d = json.load(open('/tmp/import_response.json'))
print(d.get('summary', {}).get('failed', 0))
" 2>/dev/null || echo "0")
    if [ "$failed_count" != "0" ]; then
        echo "  ERROR: import of $sample had $failed_count failed resource(s):"
        cat /tmp/import_response.json; echo ""; exit 1
    fi
    echo "  Imported $sample resources."
done

# Import E2E test infrastructure resources (e.g. admin native app for direct flow execution).
e2e_config="$SCRIPT_DIR/thunderid-config.yaml"
if [ -f "$e2e_config" ]; then
    content=$(jq -Rs . < "$e2e_config")
    http_status=$(curl -sk -o /tmp/import_response.json -w "%{http_code}" \
        -X POST "$SERVER_URL/import" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "{\"content\": $content, \"options\": {\"upsert\": true}}")
    if [ "$http_status" != "200" ]; then
        echo "ERROR: import returned HTTP $http_status for E2E config:"
        cat /tmp/import_response.json; echo ""; exit 1
    fi
    failed_count=$(python3 -c "
import sys, json
d = json.load(open('/tmp/import_response.json'))
print(d.get('summary', {}).get('failed', 0))
" 2>/dev/null || echo "0")
    if [ "$failed_count" != "0" ]; then
        echo "ERROR: E2E config import had $failed_count failed resource(s):"
        cat /tmp/import_response.json; echo ""; exit 1
    fi
    echo "Imported E2E test infrastructure resources."
fi

# Use the vanilla "Sample App" ID (stable UUID v7 from react-vanilla-sample/thunderid-config/basic).
# The vanilla sample is unaffected by MFA test setup/teardown, unlike the SDK sample.
SAMPLE_APP_ID="019e3a5c-0500-7f3e-a66e-66fc7918c3a7"

# 6. Build sample app (if not already built) and start it.
echo "Setting up sample app..."
cd "$SAMPLE_APP_DIR"
if [ ! -d "dist" ]; then
    echo "Building sample app..."
    pnpm install --frozen-lockfile
    pnpm run build
fi
pnpm start &
wait_for_url "$SAMPLE_URL" "Sample app"

# 7. Install E2E dependencies and run Playwright tests.
echo "Running Playwright E2E tests..."
cd "$SCRIPT_DIR"

# Auto-create .env with local defaults if not present.
if [ ! -f "$SCRIPT_DIR/.env" ]; then
    echo "Creating default .env for E2E tests..."
    cat > "$SCRIPT_DIR/.env" <<EOF
BASE_URL=$SERVER_URL
SERVER_URL=$SERVER_URL
ADMIN_USERNAME=${ADMIN_USERNAME:-admin}
ADMIN_PASSWORD=${ADMIN_PASSWORD:-admin}
ENVIRONMENT=local
SAMPLE_APP_URL=$SAMPLE_URL
SAMPLE_APP_ID=$SAMPLE_APP_ID
SAMPLE_APP_USERNAME=e2e-test-user
SAMPLE_APP_PASSWORD=e2e-test-password
TEST_USER_USERNAME=testuser
TEST_USER_PASSWORD=admin
MOCK_SMS_SERVER_PORT=8098
AUTO_SETUP_MFA=true
PLAYWRIGHT_WORKERS=1
EOF
fi

pnpm install --frozen-lockfile
npx playwright test "$@"
