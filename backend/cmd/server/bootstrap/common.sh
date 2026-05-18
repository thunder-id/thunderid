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

# Common functions and variables for bootstrap scripts
# Source this file at the beginning of each bootstrap script

PRODUCT_NAME="ThunderID"
QUIET_MODE="${SETUP_SILENT_MODE:-false}"
RESULT_COLOR_ENABLED=false

if [ -t 3 ] && [ -z "${NO_COLOR:-}" ]; then
    RESULT_COLOR_ENABLED=true
fi

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Logging Functions
log_info() {
    if [[ "$QUIET_MODE" != "true" ]]; then
        echo -e "${BLUE}[INFO]${NC} $1" >&2
    fi
}

log_success() {
    if [[ "$QUIET_MODE" != "true" ]]; then
        echo -e "${GREEN}[SUCCESS]${NC} ✓ $1" >&2
    fi
}

log_warning() {
    if [[ "$QUIET_MODE" != "true" ]]; then
        echo -e "${YELLOW}[WARNING]${NC} ⚠ $1" >&2
    fi
}

log_error() {
    if [[ "$QUIET_MODE" != "true" ]]; then
        echo -e "${RED}[ERROR]${NC} ✗ $1" >&2
    fi
}

log_debug() {
    if [ "${DEBUG:-false}" = "true" ] && [[ "$QUIET_MODE" != "true" ]]; then
        echo -e "${CYAN}[DEBUG]${NC} $1" >&2
    fi
}

log_result_success() {
    if [[ "$QUIET_MODE" == "true" ]]; then
        if [[ "$RESULT_COLOR_ENABLED" == "true" ]]; then
            echo -e "      ${GREEN}✓${NC} $1" >&3
        else
            echo "      ✓ $1" >&3
        fi
    fi
}

log_result_failure() {
    if [[ "$QUIET_MODE" == "true" ]]; then
        if [[ "$RESULT_COLOR_ENABLED" == "true" ]]; then
            echo -e "      ${RED}✗${NC} $1" >&3
        else
            echo "      ✗ $1" >&3
        fi
    fi
}

# API Call Helper Function
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

# Helper function to create a flow and return its ID
# Usage: create_flow <flow_file_path>
# Returns: Flow ID via echo, or empty string on failure
create_flow() {
    local FLOW_FILE="$1"
    local FLOW_PAYLOAD=$(cat "$FLOW_FILE")
    local FLOW_DISPLAY_NAME=$(grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' "$FLOW_FILE" | head -1 | sed 's/"name"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
    
    if [[ -z "$FLOW_DISPLAY_NAME" ]]; then
        log_warning "Could not extract flow name from $(basename "$FLOW_FILE"), skipping"
        echo ""
        return 1
    fi
    
    log_info "Creating flow: $FLOW_DISPLAY_NAME"
    
    local RESPONSE=$(api_call POST "/flows" "$FLOW_PAYLOAD")
    local HTTP_CODE="${RESPONSE: -3}"
    local BODY="${RESPONSE%???}"
    
    if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
        local FLOW_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
        log_success "Flow '$FLOW_DISPLAY_NAME' created successfully (ID: $FLOW_ID)"
        echo "$FLOW_ID"
        return 0
    elif [[ "$HTTP_CODE" == "409" ]]; then
        log_warning "Flow '$FLOW_DISPLAY_NAME' already exists, skipping"
        echo ""
        return 2
    else
        log_error "Failed to create flow '$FLOW_DISPLAY_NAME' (HTTP $HTTP_CODE)"
        log_error "Response: $BODY"
        echo ""
        return 1
    fi
}

# Helper function to update a flow
# Usage: update_flow <flow_id> <flow_file_path>
# Returns: 0 on success, 1 on failure
update_flow() {
    local FLOW_ID="$1"
    local FLOW_FILE="$2"
    local FLOW_PAYLOAD=$(cat "$FLOW_FILE")
    local FLOW_DISPLAY_NAME=$(grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' "$FLOW_FILE" | head -1 | sed 's/"name"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
    
    if [[ -z "$FLOW_DISPLAY_NAME" ]]; then
        log_warning "Could not extract flow name from $(basename "$FLOW_FILE"), skipping"
        return 1
    fi
    
    log_info "Updating existing flow: $FLOW_DISPLAY_NAME (ID: $FLOW_ID)"
    
    local RESPONSE=$(api_call PUT "/flows/$FLOW_ID" "$FLOW_PAYLOAD")
    local HTTP_CODE="${RESPONSE: -3}"
    local BODY="${RESPONSE%???}"
    
    if [[ "$HTTP_CODE" == "200" ]]; then
        log_success "Flow '$FLOW_DISPLAY_NAME' updated successfully"
        return 0
    else
        log_error "Failed to update flow '$FLOW_DISPLAY_NAME' (HTTP $HTTP_CODE)"
        log_error "Response: $BODY"
        return 1
    fi
}
