#!/bin/bash
# Copyright 2025-2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

# Sourced by setup.sh and start.sh. Holds what the quickstart pack's bootstrap resources (the
# *.yaml files under quickstart/bootstrap/, layered onto each plane by build.sh's
# package_quickstart) need from the environment, and the helpers both scripts share, so neither
# carries them.

AUTH_DP_PORT=8091
DP_PORT=8090
CP_PORT=8095

# new_gateway_key generates the management key the managed Data Plane trusts and the Control
# Plane's gateway resource (quickstart/bootstrap/cp/00-quickstart-gateway.yaml) registers it with,
# so no key is committed to the repository. It sets QUICKSTART_GATEWAY_KEY and its SHA-256 hex
# digest QUICKSTART_GATEWAY_KEY_HASH, and exports the key for the bootstrap template.
new_gateway_key() {
    QUICKSTART_GATEWAY_KEY="$(openssl rand -hex 32)"
    QUICKSTART_GATEWAY_KEY_HASH="$(printf '%s' "$QUICKSTART_GATEWAY_KEY" | openssl dgst -sha256 | awk '{print $NF}')"
    export QUICKSTART_GATEWAY_KEY
}

# trust_gateway_key makes the managed Data Plane trust the generated key, by writing its digest into
# dp/deployment.yaml as server.security.management_api_key_hash right after the "  security:" line.
# A digest from an earlier setup is replaced, so running setup again re-keys the link.
trust_gateway_key() {
    awk -v hash="$QUICKSTART_GATEWAY_KEY_HASH" '
        /^    management_api_key_hash:/ { next }
        { print }
        /^  security:/ { print "    management_api_key_hash: \"" hash "\"" }
    ' dp/deployment.yaml > dp/deployment.yaml.tmp
    mv dp/deployment.yaml.tmp dp/deployment.yaml
    grep -q "management_api_key_hash: \"$QUICKSTART_GATEWAY_KEY_HASH\"" dp/deployment.yaml || {
        echo "❌ Could not set the Data Plane's management key in dp/deployment.yaml."
        return 1
    }
}

# user_password_secret_name prints the name a user's password is held under in a gateway's secret
# store, which the Control Plane's export derives from the username the same way:
# USER_<USERNAME>_PASSWORD, with a word boundary wherever the case changes (ServiceDesk becomes
# SERVICE_DESK) and every character that is not a letter or digit as an underscore. See
# DeriveVariableName in backend/internal/system/varname.
user_password_secret_name() {
    local snake
    snake="$(awk -v s="$1" 'BEGIN {
        n = length(s); out = ""
        for (i = 1; i <= n; i++) {
            c = substr(s, i, 1)
            if (i > 1 && c ~ /[A-Z]/) {
                p = substr(s, i - 1, 1)
                if (p ~ /[a-z]/) out = out "_"
                else if (p != "_" && i < n && substr(s, i + 1, 1) ~ /[a-z]/) out = out "_"
            }
            out = out c
        }
        print toupper(out)
    }')"
    printf '%s' "USER_${snake}_PASSWORD" | sed 's/[^A-Za-z0-9]/_/g; s/__*/_/g; s/^_//; s/_$//'
}

# export_console_urls exports the console redirect URIs the bootstrap resources are templated with:
# CP_CONSOLE_URL for the CP_CONSOLE application on auth-dp, and ADMIN_CONSOLE_URL for the
# ADMIN_CONSOLE application on the Data Plane and the Control Plane. Call it before the first
# plane's setup runs, since auth-dp's bootstrap already needs CP_CONSOLE_URL.
export_console_urls() {
    export CP_CONSOLE_URL="https://localhost:$CP_PORT/console"
    export ADMIN_CONSOLE_URL="https://localhost:$DP_PORT/console"
}

# export_dp_ca_certificate exports DP_CA_CERTIFICATE, the managed Data Plane's certificate, which
# the Control Plane's gateway resource trusts it by. Real newlines are escaped to a literal
# two-character "\n" so the multi-line PEM survives as one line of bootstrap-template text; the
# YAML parser turns them back into real newlines once the file is templated and decoded (a plain
# double-quoted YAML scalar interprets \n as a line break). Needs dp/config/certs/server.cert, so
# call it once the key material has been shared.
export_dp_ca_certificate() {
    local dp_cert
    dp_cert="$(cat dp/config/certs/server.cert)"
    export DP_CA_CERTIFICATE="${dp_cert//$'\n'/\\n}"
}

# seed_dp_values sets the values auto-apply needs on the managed Data Plane before any write has
# captured them.
#
# Auto-apply (and a manual Capture/Apply, for that matter) refuses every sweep with "the gateway
# does not hold every value this version refers to" until each value a version names exists on the
# gateway. Two come from the Control Plane's own bootstrap, which runs while the Data Plane is not
# up to capture them to:
#
#   - the ClientID and redirect URIs of the ADMIN_CONSOLE application
#     (quickstart/bootstrap/dp-cp/04-quickstart-admin-console.yaml). The redirect URIs are a list,
#     so the variable holds a JSON array;
#   - the identifier of the System resource server, which a gateway holds for itself and is
#     the Data Plane's own MCP URL;
#   - the Service Desk Admin's password. A version carries that user to the Data Plane like any
#     other, and the Data Plane fills its password in from this secret.
#
# All are set directly on the gateway's own stores. Needs SERVICE_DESK_USERNAME and
# SERVICE_DESK_PASSWORD to be exported.
seed_dp_values() {
    local ready=false
    for _ in $(seq 1 60); do
        if curl -sk -f -o /dev/null "https://localhost:$DP_PORT/health/readiness"; then ready=true; break; fi
        sleep 0.5
    done
    if [ "$ready" != "true" ]; then
        echo "❌ The Data Plane did not become ready, so its bootstrap values were not seeded."
        return 1
    fi
    local failed=false password_json
    dp_put() {
        curl -sk -f -o /dev/null -X PUT "https://localhost:$DP_PORT$1" \
            -H "API-Key: $QUICKSTART_GATEWAY_KEY" \
            -H "Content-Type: application/json" \
            -d "$2" || { echo "❌ Failed to seed $1 on the Data Plane."; failed=true; }
    }
    dp_put "/variables/APPLICATION_ADMIN_CONSOLE_CLIENT_ID" '{"value":"ADMIN_CONSOLE"}'
    dp_put "/variables/APPLICATION_ADMIN_CONSOLE_REDIRECT_URIS" "{\"value\":\"[\\\"$ADMIN_CONSOLE_URL\\\"]\"}"
    dp_put "/variables/RESOURCE_SERVER_SYSTEM_IDENTIFIER" "{\"value\":\"https://localhost:$DP_PORT/mcp\"}"
    password_json="$(printf '%s' "$SERVICE_DESK_PASSWORD" | sed 's/\\/\\\\/g; s/"/\\"/g')"
    dp_put "/secrets/$(user_password_secret_name "$SERVICE_DESK_USERNAME")" "{\"value\":\"$password_json\"}"
    [ "$failed" != "true" ]
}

# Stopping by port, not by the PID of the subshell that launched it: start.sh/start-cp.sh each
# background the actual server binary themselves and wait on it, so killing only the outer
# subshell orphans that binary — it keeps running and keeps holding the port, which is what made
# the next run fail with "port already in use" with no obvious process to blame. Killing whatever
# is actually bound to the port is correct regardless of how many process layers are in between.
#
# TERM first, KILL only if it ignores that: the binary already handles SIGTERM itself (graceful
# shutdown, closing its database connections cleanly). A bare SIGKILL gives it no chance to do
# that — harmless the vast majority of the time under SQLite's WAL mode, but it's an unforced risk
# to a database mid-write for no benefit over just asking first.
kill_port() {
    local port="$1"
    local pids
    # lsof exits non-zero when nothing matches — the normal case here — which under `set -e`
    # would otherwise kill this script outright the first time a port is already free.
    pids="$(lsof -ti "tcp:$port" 2>/dev/null || true)"
    [ -z "$pids" ] && return 0
    # shellcheck disable=SC2086
    kill -TERM $pids 2>/dev/null || true
    for _ in 1 2 3 4 5 6 7 8 9 10; do
        lsof -ti "tcp:$port" >/dev/null 2>&1 || return 0
        sleep 0.3
    done
    # shellcheck disable=SC2086
    kill -KILL $pids 2>/dev/null || true
}
