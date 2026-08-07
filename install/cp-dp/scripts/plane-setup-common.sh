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

# Provisioning steps shared by setup-control-plane.sh and setup-data-plane.sh.
#
# Sourced, not executed. These run once, from outside the deployment: from an operator's machine or a
# platform task, against an unpacked ThunderID distribution and the deployment's database. They are
# not part of the image.
#
# What they do lands in the database, and nothing else: the schema, and the baseline resources. Key
# material is generated and mounted by whatever provisions the deployment, and every other secret is
# supplied through the environment, so neither is created here.
#
# Every function is safe to run again, so a re-run after a failure part way through picks up where it
# left off rather than failing on what is already there.
#
# deployment.yaml is read and never written. It is the deployment's own configuration, and a
# provisioning step that rewrote it would discard what was deployed.

set -euo pipefail

log()  { echo "==> $*"; }
skip() { echo "    (already done) $*"; }
warn() { echo "WARNING: $*" >&2; }
die()  { echo "ERROR: $*" >&2; exit 1; }

# The four datasources, each with the directory its schema lives in and the environment variable
# carrying its password. The names match the Helm chart's, so one set of secrets serves both.
DATASOURCES="config:configdb:DB_CONFIG_PASSWORD
runtime_transient:runtime-transient:DB_RUNTIME_TRANSIENT_PASSWORD
entity:entitydb:DB_ENTITY_PASSWORD
runtime_persistent:runtime-persistent:DB_RUNTIME_PERSISTENT_PASSWORD"

# An unpacked ThunderID distribution to provision from: the directory holding the binary, dbscripts/,
# scripts/ and this deployment's deployment.yaml. Set THUNDERID_HOME, or pass --home.
#
# The distribution supplies the tools; the deployment.yaml in it decides what is provisioned and
# where. Point it at a copy holding the configuration of the deployment being provisioned.
resolve_home() {
    local home="${THUNDERID_HOME:-}"
    while [ $# -gt 0 ]; do
        case "$1" in
            --home) home="${2:-}"; shift 2 ;;
            *) die "unknown argument: $1" ;;
        esac
    done

    [ -n "$home" ] || die "set THUNDERID_HOME, or pass --home PATH, to an unpacked ThunderID
distribution holding this deployment's deployment.yaml."
    [ -d "$home" ] || die "no such directory: $home"

    SERVER_HOME="$(cd "$home" && pwd)"
    CONFIG_FILE="$SERVER_HOME/deployment.yaml"
    DB_SCRIPTS="$SERVER_HOME/dbscripts"
    INIT_DB="$SERVER_HOME/scripts/init_script.sh"

    [ -x "$SERVER_HOME/thunderid" ] || die "$SERVER_HOME holds no thunderid binary.
Unpack the distribution for the plane being provisioned, and run this against that."
    [ -d "$DB_SCRIPTS" ] || die "$SERVER_HOME holds no dbscripts directory."
    [ -f "$INIT_DB" ] || die "$SERVER_HOME holds no scripts/init_script.sh."
}

# yaml_get prints the scalar at a dotted path in deployment.yaml, or nothing when it is absent.
#
# This handles the plain nested mappings this file uses and nothing more; it is not a YAML parser.
# It is enough because only structural values are read here. Secrets are not: in a deployment they
# are template placeholders in this file, resolved from the environment by the server at startup, so
# reading one from here would yield the placeholder rather than the secret.
yaml_get() {
    local path="$1"
    awk -v want="$path" '
        { line = $0; sub(/#.*/, "", line) }
        line ~ /^[[:space:]]*$/ { next }
        line ~ /^[[:space:]]*-/ { next }          # list entries hold nothing this reads
        {
            match(line, /^ */); depth = int(RLENGTH / 2)
            key = line; sub(/^ */, "", key)
            val = ""
            if (match(key, /:/)) {
                val = substr(key, RSTART + 1)
                key = substr(key, 1, RSTART - 1)
            }
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", val)
            gsub(/^"|"$/, "", val)
            stack[depth] = key
            for (i = depth + 1; i in stack; i++) delete stack[i]
            full = stack[0]
            for (i = 1; i <= depth; i++) full = full "." stack[i]
            if (full == want) { print val; exit }
        }
    ' "$CONFIG_FILE"
}

# require_config fails unless the deployment has a configuration to be provisioned against.
require_config() {
    [ -f "$CONFIG_FILE" ] || die "no deployment.yaml at $CONFIG_FILE.
Put this deployment's configuration there before provisioning. This script never writes it."
    log "Configuration: $CONFIG_FILE (read only)"
}

# expect_mode fails when the configuration is for a different plane than the script provisioning it,
# which would otherwise provision the wrong baseline against the right database.
expect_mode() {
    local want="$1" have
    have="$(yaml_get server.mode)"
    [ -n "$have" ] || die "server.mode is not set in $CONFIG_FILE (expected \"$want\")."
    [ "$have" = "$want" ] || die "server.mode is \"$have\" but this script provisions a \"$want\".
Run setup-$( [ "$have" = cp ] && echo control-plane || echo data-plane ).sh instead."
}

# require_referenced_files fails unless every file the configuration points at is readable.
#
# Key material is generated and mounted by whatever provisions the deployment, not here. Seeding the
# baseline still loads this configuration, and loading it reads every file reference in it, so the
# same material the pods mount has to be reachable from wherever this runs. Checking up front turns a
# missing mount into one clear message rather than a failure part way through.
require_referenced_files() {
    local missing=0 path refs
    refs="$(sed 's/#.*//' "$CONFIG_FILE" | grep -o 'file://[^"[:space:]]*' | sed 's|^file://||' || true)"
    [ -n "$refs" ] || return 0

    log "Key material"
    while read -r path; do
        [ -n "$path" ] || continue
        case "$path" in /*) ;; *) path="$SERVER_HOME/$path" ;; esac
        if [ -r "$path" ]; then
            echo "    found $path"
        else
            warn "the configuration points at a file that is not readable: $path"
            missing=1
        fi
    done <<< "$refs"

    [ "$missing" -eq 0 ] || die "the configuration points at files that are not there.
Key material is generated and mounted outside this script. Put the same material this deployment will
run with under $SERVER_HOME before provisioning, because seeding the baseline loads this
configuration and loading it reads those files."
}

# db_is_populated reports whether a datasource already holds tables.
#
# The schema scripts create tables unconditionally, so applying one to a populated database fails.
# Asking whether anything is there is what keeps this safe to rerun, and it needs no sentinel table
# kept in step with the schema.
db_is_populated() {
    case "$1" in
        sqlite)
            [ -f "$2" ] || return 1
            [ "$(sqlite3 "$2" "SELECT count(*) FROM sqlite_master WHERE type='table';")" != "0" ]
            ;;
        postgres)
            local count
            count="$(PGPASSWORD="$PG_PASSWORD" psql -qtAX -h "$PG_HOST" -p "$PG_PORT" \
                -U "$PG_USER" -d "$PG_NAME" \
                -c "SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema();" \
                2>/dev/null | tr -d '[:space:]')"
            [ -n "$count" ] && [ "$count" != "0" ]
            ;;
    esac
}

# load_postgres_schema loads one datasource's schema when its database is empty.
#
# The distribution's own init_script.sh does the loading, and creates the database when it is absent.
load_postgres_schema() {
    local ds="$1" dir="$2" pw_var="$3"

    PG_HOST="$(yaml_get "database.$ds.postgres.hostname")"
    PG_PORT="$(yaml_get "database.$ds.postgres.port")"
    PG_NAME="$(yaml_get "database.$ds.postgres.name")"
    PG_USER="$(yaml_get "database.$ds.postgres.username")"
    PG_PORT="${PG_PORT:-5432}"
    PG_PASSWORD="${!pw_var:-}"

    [ -n "$PG_HOST" ] && [ -n "$PG_NAME" ] || die "database.$ds.postgres is incomplete in $CONFIG_FILE"
    [ -n "$PG_PASSWORD" ] || die "\$$pw_var is not set.
The password is not read from deployment.yaml: there it is a placeholder the server resolves from its
environment. Provide the same value here, from the same secret."

    command -v psql >/dev/null 2>&1 || die "psql is required to load the $ds schema.
Run this where psql is available, or load the schema yourself, once:
  psql -h $PG_HOST -p $PG_PORT -U $PG_USER -d $PG_NAME -f $DB_SCRIPTS/$dir/postgres.sql"

    if db_is_populated postgres; then
        skip "$ds: $PG_NAME on $PG_HOST already holds tables"
        return
    fi
    log "    $ds: loading schema into $PG_NAME on $PG_HOST"
    "$INIT_DB" -type postgres -schema "$DB_SCRIPTS/$dir/postgres.sql" \
        -host "$PG_HOST" -port "$PG_PORT" -name "$PG_NAME" \
        -username "$PG_USER" -password "$PG_PASSWORD" >/dev/null \
        || die "failed to load the $ds schema into $PG_NAME"
}

# load_sqlite_schema creates one datasource's database file when it is not there yet.
#
# SQLite is for a single instance only: the file belongs to one container, so a deployment running
# several of them does not share it. Anything with more than one replica needs PostgreSQL.
load_sqlite_schema() {
    local ds="$1" dir="$2" path
    path="$(yaml_get "database.$ds.sqlite.path")"
    [ -n "$path" ] || die "database.$ds.sqlite.path is not set in $CONFIG_FILE"
    case "$path" in /*) ;; *) path="$SERVER_HOME/$path" ;; esac

    if db_is_populated sqlite "$path"; then
        skip "$ds: $path already holds tables"
        return
    fi
    log "    $ds: creating $path"
    mkdir -p "$(dirname "$path")"
    "$INIT_DB" -type sqlite -schema "$DB_SCRIPTS/$dir/sqlite.sql" \
        -name "$ds" -path "$path" >/dev/null || die "failed to create $path"
}

# ensure_schema loads whichever schema each of the four datasources is configured for.
ensure_schema() {
    log "Database schema"
    local ds dir pw_var engine
    while IFS=: read -r ds dir pw_var; do
        [ -n "$ds" ] || continue
        engine="$(yaml_get "database.$ds.type")"
        case "$engine" in
            postgres) load_postgres_schema "$ds" "$dir" "$pw_var" ;;
            sqlite)   load_sqlite_schema "$ds" "$dir" ;;
            redis)    skip "$ds: redis needs no schema" ;;
            "")       die "database.$ds.type is not set in $CONFIG_FILE" ;;
            *)        die "database.$ds.type \"$engine\" is not supported" ;;
        esac
    done <<< "$DATASOURCES"
}

# run_bootstrap seeds the baseline resources: the admin user, the roles, and the console application.
#
# A deployment id scopes the baseline to one tenant, which is how a Control Plane in token mode
# provisions each. Bootstrap upserts, and a tenant's baseline ids are derived from its deployment id,
# so running this again against a provisioned tenant changes nothing.
run_bootstrap() {
    local deployment_id="${1:-}" args=()
    [ -n "$deployment_id" ] && args+=(--deployment-id "$deployment_id")

    [ -n "${ADMIN_PASSWORD:-}" ] || die "\$ADMIN_PASSWORD is not set.
Set it, with \$ADMIN_USERNAME, to the credentials the first administrator signs in with."

    if [ -n "$deployment_id" ]; then
        log "Baseline resources for tenant \"$deployment_id\""
    else
        log "Baseline resources"
    fi
    # "${args[@]+...}" guards the expansion: under set -u, bash 3.2 treats an empty array as an unbound
    # variable, which is exactly the data plane's case, where no deployment id is passed.
    ( cd "$SERVER_HOME" && \
        ADMIN_USERNAME="${ADMIN_USERNAME:-admin}" ADMIN_PASSWORD="$ADMIN_PASSWORD" \
        ./thunderid bootstrap ${args[@]+"${args[@]}"} \
            --admin-username "${ADMIN_USERNAME:-admin}" --admin-password "$ADMIN_PASSWORD" ) \
        || die "bootstrap failed"
}
