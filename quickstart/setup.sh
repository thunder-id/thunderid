#!/bin/bash
# Copyright 2025-2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

# Quickstart setup: the one-time setup for three deployments that run together, already wired to
# each other: a Control Plane, the Data Plane it manages (pushes applications/connections/roles/
# flows to), and a second, otherwise-ordinary Data Plane (auth-dp) dedicated to the Control
# Plane's own sign-in. Keeping that sign-in on its own deployment means logging into the Control
# Plane never depends on the one deployment it also manages: auth-dp's only job is to
# authenticate Control Plane operators, not to be managed itself.
#
# Run this once, then use start.sh to start all three. It creates the certs, key material and
# default resources for all three, and registers the managed Data Plane as a gateway on the
# Control Plane. Running it again is safe (it skips what already exists and upserts what doesn't
# need to change), but it also resets any setting changed since, so start.sh does not run it.
#
# auth-dp, the managed Data Plane, and the Control Plane share one generated certificate rather
# than each minting its own, which is what lets the Control Plane's bootstrap gateway resource
# name a fixed caCertificate: it is the one certificate this script generates, not a guess at what
# any of the three would have generated on their own. Sharing the certificate with the managed
# Data Plane too (rather than only with auth-dp, which is all correctness actually requires) keeps
# this to one self-signed certificate for an operator's browser to trust instead of two.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [ ! -d "auth-dp" ] || [ ! -d "dp" ] || [ ! -d "cp" ]; then
    echo "❌ Expected an auth-dp/, a dp/ and a cp/ directory next to this script (this is meant to"
    echo "   be run from inside the unzipped quickstart distribution)."
    exit 1
fi

# shellcheck source=quickstart/bootstrap.sh
source "$SCRIPT_DIR/quickstart-bootstrap.sh"

# Defensive: a prior run that didn't exit cleanly (e.g. the terminal was closed) can leave any of
# the three binaries still running. Clear all three ports before setup starts, so this run doesn't
# fail on a port conflict the operator never caused themselves.
kill_port "$CP_PORT"
kill_port "$DP_PORT"
kill_port "$AUTH_DP_PORT"

# A marker from an earlier run must not let start.sh proceed if this run fails part-way.
rm -f .quickstart-setup-done

echo "========================================="
echo "   ThunderID Quickstart Setup"
echo "========================================="
echo ""

# ============================================================================
# Account credentials: requested once, here, for two separate accounts.
#
#   Admin: the operator who signs into the Control Plane console. Created on auth-dp, which
#   authenticates it (ADMIN_USERNAME / ADMIN_PASSWORD, the names auth-dp's setup already reads).
#
#   Service Desk Admin: authored on the Control Plane and synced to the managed Data Plane, where it
#   signs into the Admin Console. Both planes get the same account, so the Data Plane has it from
#   the start and the Control Plane's copy arrives on the first sync unchanged
#   (SERVICE_DESK_USERNAME / SERVICE_DESK_PASSWORD).
#
# Left to each setup script, an operator would be prompted three times. This mirrors setup.sh's
# own prompt (a generated password offered as the default, interactive only when stdin is a
# terminal) and exports the result so none of the nested scripts prompts again or generates a
# password of its own.
# ============================================================================
ADMIN_PASSWORD_GENERATED=false
SERVICE_DESK_PASSWORD_GENERATED=false

# random_password prints a 12-character password mixing letters, digits, and special characters,
# the same way setup.sh does: the value flows through environment variables and a YAML bootstrap
# template, so the special set stays limited to shell- and YAML-safe punctuation.
random_password() {
    local charset='A-Za-z0-9@#%+=_.?-' candidate
    while true; do
        candidate="$(LC_ALL=C tr -dc "$charset" < /dev/urandom 2>/dev/null | head -c 12 || true)"
        [ "${#candidate}" -eq 12 ] || continue
        case "$candidate" in *[0-9]*) ;; *) continue ;; esac
        case "$candidate" in *[@#%+=_.?-]*) printf '%s' "$candidate"; return 0 ;; esac
    done
}

# configure_account label default-username username-var password-var generated-var
# fills an account's username and password from the environment, the prompt, or a default, in that
# order. Non-interactive with nothing supplied (e.g. CI): falls back to a generated password rather
# than leaving it empty, the same as setup.sh does on its own.
configure_account() {
    local label="$1" default_user="$2" user_var="$3" pass_var="$4" generated_var="$5"
    local username="${!user_var:-}" password="${!pass_var:-}" generated=false input

    if [ -t 0 ] && [[ -z "$username" || -z "$password" ]]; then
        echo "Configure the $label (press Enter to accept defaults):"
        echo ""
        if [ -z "$username" ]; then
            read -r -p "  Username [$default_user]: " input
            username="${input:-$default_user}"
        fi
        if [ -z "$password" ]; then
            # Generated up front so it can be shown as the prompt default; a typed value overrides it.
            password="$(random_password)"
            generated=true
            read -r -s -p "  Password [$password]: " input
            echo ""
            if [ -n "$input" ]; then
                password="$input"
                generated=false
            fi
        fi
        echo ""
    fi
    username="${username:-$default_user}"
    if [ -z "$password" ]; then
        password="$(random_password)"
        generated=true
    fi
    printf -v "$user_var" '%s' "$username"
    printf -v "$pass_var" '%s' "$password"
    printf -v "$generated_var" '%s' "$generated"
}

configure_account "admin who signs into the Control Plane console (created on auth-dp)" admin \
    ADMIN_USERNAME ADMIN_PASSWORD ADMIN_PASSWORD_GENERATED
configure_account "Service Desk Admin (created on the Control Plane, synced to the Data Plane)" servicedeskadmin \
    SERVICE_DESK_USERNAME SERVICE_DESK_PASSWORD SERVICE_DESK_PASSWORD_GENERATED
export ADMIN_USERNAME ADMIN_PASSWORD SERVICE_DESK_USERNAME SERVICE_DESK_PASSWORD

export_console_urls

echo "⚙️  Setting up auth-dp (backs the Control Plane's own sign-in)..."
# auth-dp carries one application, the Control Plane console's own client (CP_CONSOLE); it has no
# console UI of its own.
(cd auth-dp && ./setup.sh)

# Share auth-dp's key material with the managed Data Plane and the Control Plane before their
# setups run, so configure_certificates there finds everything already present and generates
# nothing of its own: all three end up presenting the identical TLS certificate, and — just as
# important for the Control Plane — signing with the identical JWT signing keys. That second part
# is what lets a token auth-dp issued (the Control Plane mints none of its own, and the managed
# Data Plane issues its own, independent tokens for its own Gate/Admin Console) verify on the
# Control Plane too, once its deployment.yaml is patched (at package time) to claim auth-dp's
# issuer as its own.
echo ""
echo "⚙️  Sharing auth-dp's key material with the Data Plane and the Control Plane..."
mkdir -p dp/config/certs cp/config/certs
for target in dp cp; do
    cp auth-dp/config/certs/server.cert auth-dp/config/certs/server.key \
       auth-dp/config/certs/signing.cert auth-dp/config/certs/signing.key \
       auth-dp/config/certs/ecdsa-signing.cert auth-dp/config/certs/ecdsa-signing.key \
       auth-dp/config/certs/crypto.key \
       "$target/config/certs/"
done

# The Control Plane's bootstrap gateway resource trusts the managed Data Plane by its certificate.
export_dp_ca_certificate

# The Control Plane registers the managed Data Plane as a gateway with a key generated here, and the
# Data Plane trusts that key's digest, so the link needs no key committed to the repository.
new_gateway_key
trust_gateway_key

echo ""
echo "⚙️  Setting up the managed Data Plane..."
(cd dp && ADMIN_USERNAME="$SERVICE_DESK_USERNAME" ADMIN_PASSWORD="$SERVICE_DESK_PASSWORD" ./setup.sh)

echo ""
echo "⚙️  Setting up the Control Plane, and registering the managed Data Plane as its gateway..."
# The Data Plane's own console is its own application (ADMIN_CONSOLE), bootstrapped from the same
# file on the Data Plane and here with the same fixed ID, so the copy this Control Plane syncs
# replaces the Data Plane's rather than duplicating it.
(cd cp && ADMIN_USERNAME="$SERVICE_DESK_USERNAME" ADMIN_PASSWORD="$SERVICE_DESK_PASSWORD" ./setup-cp.sh)

echo ""
echo "⚙️  Seeding the bootstrap values auto-apply needs on the managed Data Plane..."
# Needs the Data Plane running, so it is started just long enough to hold these values, which
# persist in its own stores.
(cd dp && ./start.sh) &
seed_dp_values || { kill_port "$DP_PORT"; exit 1; }
kill_port "$DP_PORT"
wait 2>/dev/null || true
touch .quickstart-setup-done

echo ""
echo "✅ Setup complete. Run ./start.sh to start the quickstart."
echo ""
echo "  Accounts:"
echo "    admin (signs into the Control Plane console, created on auth-dp)"
echo "      Username: ${ADMIN_USERNAME}"
if [ "$ADMIN_PASSWORD_GENERATED" = "true" ]; then echo "      Password: ${ADMIN_PASSWORD} (generated)"; fi
echo "    Service Desk Admin (created on the Control Plane, synced to the Data Plane)"
echo "      Username: ${SERVICE_DESK_USERNAME}"
if [ "$SERVICE_DESK_PASSWORD_GENERATED" = "true" ]; then echo "      Password: ${SERVICE_DESK_PASSWORD} (generated)"; fi
