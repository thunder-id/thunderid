#!/bin/bash
set -euo pipefail
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

# =============================================================================
# Runtime Database Cleanup Script
#
# Deletes expired rows from runtime database tables. Designed to be run
# manually (one-shot) or scheduled via cron for periodic cleanup.
#
# Tables cleaned (in order):
#   1. FLOW_CONTEXT       - cascades to FLOW_USER_DATA via ON DELETE CASCADE
#   2. AUTHORIZATION_CODE
#   3. AUTHORIZATION_REQUEST
#   4. WEBAUTHN_SESSION
#   5. ATTRIBUTE_CACHE
#   6. PAR_REQUEST
#   7. JTI_RECORD
#
# Usage examples:
#   # SQLite (local development)
#   ./cleanup_runtime_db.sh -type sqlite -path /path/to/runtimedb.db
#
#   # PostgreSQL
#   ./cleanup_runtime_db.sh -type postgres -host localhost -port 5432 \
#       -name thunderidruntime -username thunderid -password secret
#
#   # With options
#   ./cleanup_runtime_db.sh -type sqlite -path /path/to/runtimedb.db \
#       -batch_size 500 -grace_period 120 -deployment_id my-deployment
#
#   # Dry run (show counts without deleting)
#   ./cleanup_runtime_db.sh -type sqlite -path /path/to/runtimedb.db -dry_run
#
#   # Cron (every 30 minutes)
#   */30 * * * * /opt/thunderid/scripts/cleanup_runtime_db.sh \
#       -type postgres -host localhost -port 5432 -name thunderidruntime \
#       -username thunderid -password "$THUNDERID_DB_PASSWORD" \
#       >> /var/log/thunderid-cleanup.log 2>&1
# =============================================================================

# Script common variables.
TYPE=""
BATCH_SIZE=1000
GRACE_PERIOD=60
DEPLOYMENT_ID=""
DRY_RUN="false"

# Database connection details.
HOST=""
PORT=""
NAME=""
DB_PATH=""
USERNAME=""
PASSWORD=""

# Tables to clean (order matters: FLOW_CONTEXT first for cascade).
TABLES=("FLOW_CONTEXT" "AUTHORIZATION_CODE" "AUTHORIZATION_REQUEST" "WEBAUTHN_SESSION" "ATTRIBUTE_CACHE" "PAR_REQUEST" "JTI_RECORD")

# Totals for summary.
TOTAL_DELETED=0

print_help() {
  echo ""
  echo "Runtime Database Cleanup Script"
  echo ""
  echo "Deletes expired rows from runtime database tables. Can be run once"
  echo "or scheduled via cron for periodic cleanup."
  echo ""
  echo "Usage:"
  echo "  $0 [OPTIONS]"
  echo ""
  echo "Options:"
  printf "  %-18s %s\n" "-type"          "Type of the database: postgres or sqlite [required]"
  printf "  %-18s %s\n" "-host"          "Database host (required for postgres)"
  printf "  %-18s %s\n" "-port"          "Database port (required for postgres)"
  printf "  %-18s %s\n" "-name"          "Database name (required for postgres)"
  printf "  %-18s %s\n" "-path"          "Path to the SQLite database file (required for sqlite)"
  printf "  %-18s %s\n" "-username"      "Database username (required for postgres)"
  printf "  %-18s %s\n" "-password"      "Database password (required for postgres)"
  printf "  %-18s %s\n" "-batch_size"    "Rows to delete per batch iteration (default: 1000)"
  printf "  %-18s %s\n" "-grace_period"  "Seconds buffer before now for expiry cutoff (default: 60)"
  printf "  %-18s %s\n" "-deployment_id" "Scope cleanup to a specific DEPLOYMENT_ID (optional)"
  printf "  %-18s %s\n" "-dry_run"       "Print expired row counts without deleting"
  printf "  %-18s %s\n" "-h, --help"     "Show this help message and exit"
  echo ""
}

parse_args() {
  while [[ "$#" -gt 0 ]]; do
    case "$1" in
      -type|-host|-port|-name|-path|-username|-password|-batch_size|-grace_period|-deployment_id)
        if [[ "$#" -lt 2 ]] || [[ -z "$2" ]]; then
          echo "Error: Missing value for $1"
          echo "Use -h or --help for usage information."
          exit 1
        fi
        case "$1" in
          -type)          TYPE="$2";;
          -host)          HOST="$2";;
          -port)          PORT="$2";;
          -name)          NAME="$2";;
          -path)          DB_PATH="$2";;
          -username)      USERNAME="$2";;
          -password)      PASSWORD="$2";;
          -batch_size)    BATCH_SIZE="$2";;
          -grace_period)  GRACE_PERIOD="$2";;
          -deployment_id) DEPLOYMENT_ID="$2";;
        esac
        shift 2
        ;;
      -dry_run)       DRY_RUN="true"; shift;;
      -h|--help)      print_help; exit 0;;
      *) echo "Unknown parameter passed: $1"; exit 1;;
    esac
  done
}

validate_inputs() {
  if [ -z "$TYPE" ]; then
    echo "Error: Database type is required. Please provide it using -type."
    echo "Use -h or --help for usage information."
    exit 1
  fi

  if [ "$TYPE" = "postgres" ]; then
    if [ -z "$HOST" ] || [ -z "$PORT" ] || [ -z "$NAME" ] || [ -z "$USERNAME" ] || [ -z "$PASSWORD" ]; then
      echo "Error: PostgreSQL connection details are required."
      echo "Please provide -host, -port, -name, -username, and -password."
      echo "Use -h or --help for usage information."
      exit 1
    fi
  elif [ "$TYPE" = "sqlite" ]; then
    if [ -z "$DB_PATH" ]; then
      echo "Error: SQLite database path is required. Please provide it using -path."
      echo "Use -h or --help for usage information."
      exit 1
    fi
    if [ ! -f "$DB_PATH" ]; then
      echo "Error: SQLite database file not found: $DB_PATH"
      exit 1
    fi
  else
    echo "Error: Unsupported database type: $TYPE"
    exit 1
  fi

  # Validate batch_size is a positive integer.
  if ! [[ "$BATCH_SIZE" =~ ^[0-9]+$ ]] || [ "$BATCH_SIZE" -le 0 ]; then
    echo "Error: -batch_size must be a positive integer."
    exit 1
  fi

  # Validate grace_period is a non-negative integer.
  if ! [[ "$GRACE_PERIOD" =~ ^[0-9]+$ ]]; then
    echo "Error: -grace_period must be a non-negative integer (seconds)."
    exit 1
  fi
}

# -----------------------------------------------------------------------------
# SQLite cleanup functions
# -----------------------------------------------------------------------------

# Get the count of expired rows for a table in SQLite.
# Arguments: $1 = table name
sqlite_count_expired() {
  local table="$1"
  local deployment_filter=""
  local safe_deployment_id=""
  if [ -n "$DEPLOYMENT_ID" ]; then
    safe_deployment_id="${DEPLOYMENT_ID//\'/\'\'}"
    deployment_filter="AND DEPLOYMENT_ID = '${safe_deployment_id}'"
  fi

  local output
  local stderr_out
  stderr_out=$(mktemp)
  if ! output=$(sqlite3 "$DB_PATH" \
    "SELECT COUNT(*) FROM ${table} WHERE EXPIRY_TIME < datetime('now', '-${GRACE_PERIOD} seconds') ${deployment_filter};" \
    2>"$stderr_out"); then
    echo "Error: sqlite3 COUNT query failed for table ${table}" >&2
    cat "$stderr_out" >&2
    rm -f "$stderr_out"
    exit 1
  fi
  rm -f "$stderr_out"
  echo "$output"
}

# Delete expired rows from a table in SQLite using batch deletes.
# Arguments: $1 = table name
# Returns: total rows deleted via TOTAL_TABLE_DELETED variable.
sqlite_cleanup_table() {
  local table="$1"
  local deployment_filter=""
  local safe_deployment_id=""
  if [ -n "$DEPLOYMENT_ID" ]; then
    safe_deployment_id="${DEPLOYMENT_ID//\'/\'\'}"
    deployment_filter="AND DEPLOYMENT_ID = '${safe_deployment_id}'"
  fi

  TOTAL_TABLE_DELETED=0
  while true; do
    local deleted
    local stderr_out
    stderr_out=$(mktemp)
    if ! deleted=$(sqlite3 "$DB_PATH" \
      "DELETE FROM ${table} WHERE rowid IN (SELECT rowid FROM ${table} WHERE EXPIRY_TIME < datetime('now', '-${GRACE_PERIOD} seconds') ${deployment_filter} LIMIT ${BATCH_SIZE}); SELECT changes();" \
      2>"$stderr_out"); then
      echo "Error: sqlite3 DELETE query failed for table ${table}" >&2
      cat "$stderr_out" >&2
      rm -f "$stderr_out"
      exit 1
    fi
    rm -f "$stderr_out"

    if [ -z "$deleted" ] || [ "$deleted" -eq 0 ]; then
      break
    fi
    TOTAL_TABLE_DELETED=$((TOTAL_TABLE_DELETED + deleted))
  done
}

# -----------------------------------------------------------------------------
# PostgreSQL cleanup functions
# -----------------------------------------------------------------------------

# Helper to run a psql command and return the output.
# Stderr is intentionally not suppressed so DB errors surface to the caller.
# psql exits non-zero on connection or SQL errors (ON_ERROR_STOP=1).
# Arguments: $1 = SQL query
psql_exec() {
  PGPASSWORD="$PASSWORD" psql -h "$HOST" -p "$PORT" -U "$USERNAME" -d "$NAME" \
    -v ON_ERROR_STOP=1 -Atc "$1"
}

# Get the count of expired rows for a table in PostgreSQL.
# Arguments: $1 = table name
postgres_count_expired() {
  local table="$1"
  local deployment_filter=""
  local safe_deployment_id=""
  if [ -n "$DEPLOYMENT_ID" ]; then
    safe_deployment_id="${DEPLOYMENT_ID//\'/\'\'}"
    deployment_filter="AND DEPLOYMENT_ID = '${safe_deployment_id}'"
  fi

  local count
  if ! count=$(psql_exec "SELECT COUNT(*) FROM ${table} WHERE EXPIRY_TIME < NOW() - INTERVAL '${GRACE_PERIOD} seconds' ${deployment_filter};"); then
    echo "Error: psql COUNT query failed for table ${table}" >&2
    exit 1
  fi
  echo "$count"
}

# Delete expired rows from a table in PostgreSQL using batch deletes.
# Arguments: $1 = table name
# Returns: total rows deleted via TOTAL_TABLE_DELETED variable.
postgres_cleanup_table() {
  local table="$1"
  local deployment_filter=""
  local safe_deployment_id=""
  if [ -n "$DEPLOYMENT_ID" ]; then
    safe_deployment_id="${DEPLOYMENT_ID//\'/\'\'}"
    deployment_filter="AND DEPLOYMENT_ID = '${safe_deployment_id}'"
  fi

  TOTAL_TABLE_DELETED=0
  while true; do
    local deleted
    if ! deleted=$(psql_exec \
      "WITH deleted AS (DELETE FROM ${table} WHERE ctid IN (SELECT ctid FROM ${table} WHERE EXPIRY_TIME < NOW() - INTERVAL '${GRACE_PERIOD} seconds' ${deployment_filter} LIMIT ${BATCH_SIZE}) RETURNING 1) SELECT COUNT(*) FROM deleted;"); then
      echo "Error: psql DELETE query failed for table ${table}" >&2
      exit 1
    fi

    if [ -z "$deleted" ] || [ "$deleted" -eq 0 ]; then
      break
    fi
    TOTAL_TABLE_DELETED=$((TOTAL_TABLE_DELETED + deleted))
  done
}

# -----------------------------------------------------------------------------
# Cleanup orchestration
# -----------------------------------------------------------------------------

# Clean a single table (dry-run or actual delete).
# Arguments: $1 = table name
cleanup_table() {
  local table="$1"
  local start_time
  start_time=$(date +%s)

  if [ "$DRY_RUN" = "true" ]; then
    local count
    if [ "$TYPE" = "sqlite" ]; then
      count=$(sqlite_count_expired "$table")
    else
      count=$(postgres_count_expired "$table")
    fi
    printf "  [%-25s] Expired rows: %s (dry run - not deleted)\n" "$table" "${count:-0}"
    return
  fi

  if [ "$TYPE" = "sqlite" ]; then
    sqlite_cleanup_table "$table"
  else
    postgres_cleanup_table "$table"
  fi

  local end_time
  end_time=$(date +%s)
  local elapsed=$((end_time - start_time))

  printf "  [%-25s] Deleted: %d rows (%ds)\n" "$table" "$TOTAL_TABLE_DELETED" "$elapsed"
  TOTAL_DELETED=$((TOTAL_DELETED + TOTAL_TABLE_DELETED))
}

cleanup_all_tables() {
  echo "Cleaning up expired rows from runtime database tables..."
  echo ""

  if [ -n "$DEPLOYMENT_ID" ]; then
    echo "  Deployment filter: $DEPLOYMENT_ID"
  else
    echo "  Deployment filter: all deployments"
  fi
  echo "  Batch size:        $BATCH_SIZE"
  echo "  Grace period:      ${GRACE_PERIOD}s"
  echo "  Database type:     $TYPE"
  if [ "$DRY_RUN" = "true" ]; then
    echo "  Mode:              DRY RUN"
  fi
  echo ""

  for table in "${TABLES[@]}"; do
    cleanup_table "$table"
  done

  echo ""
  if [ "$DRY_RUN" = "true" ]; then
    echo "Dry run completed. No rows were deleted."
  else
    echo "Cleanup completed. Total rows deleted: $TOTAL_DELETED"
  fi
}

# -----------------------------------------------------------------------------
# Main entry point
# -----------------------------------------------------------------------------

main() {
  echo "============================================"
  echo "ThunderID Runtime Database Cleanup"
  echo "$(date '+%Y-%m-%d %H:%M:%S')"
  echo "============================================"
  echo ""

  parse_args "$@"
  validate_inputs
  cleanup_all_tables

  echo ""
}

main "$@"

