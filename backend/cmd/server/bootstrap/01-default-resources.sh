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

# Bootstrap Script: Default Resources Setup
# Creates default organization unit, user type, admin user, system resource server, system action, admin role, and CONSOLE application

set -e

# Parse command line arguments for custom redirect URIs
CUSTOM_CONSOLE_REDIRECT_URIS=""
while [[ $# -gt 0 ]]; do
    case $1 in
        --console-redirect-uris)
            CUSTOM_CONSOLE_REDIRECT_URIS="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done

# Source common functions from the same directory as this script
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]:-$0}")"
source "${SCRIPT_DIR}/common.sh"

log_info "Creating default ${PRODUCT_NAME} resources..."
echo ""

SYSTEM_RS_IDENTIFIER="https://localhost:8090/mcp"

# ============================================================================
# Create Default Organization Unit
# ============================================================================

log_info "Creating default organization unit..."

RESPONSE=$(api_call POST "/organization-units" '{
  "handle": "default",
  "name": "Default",
  "description": "Default organization unit",
  "logoUrl": "emoji:🏛️"
}')

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "Organization unit created successfully"
    DEFAULT_OU_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -n "$DEFAULT_OU_ID" ]]; then
        log_info "Default OU ID: $DEFAULT_OU_ID"
        log_result_success "Created default organization unit"
    else
        log_error "Could not extract OU ID from response"
        log_result_failure "Failed to create default organization unit"
        exit 1
    fi
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "Organization unit already exists, retrieving OU ID..."
    # Get existing OU ID by handle to ensure we get the correct "default" OU
    RESPONSE=$(api_call GET "/organization-units/tree/default")
    HTTP_CODE="${RESPONSE: -3}"
    BODY="${RESPONSE%???}"

    if [[ "$HTTP_CODE" == "200" ]]; then
        DEFAULT_OU_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [[ -n "$DEFAULT_OU_ID" ]]; then
            log_success "Found OU ID: $DEFAULT_OU_ID"
            log_result_success "Created default organization unit"
        else
            log_error "Could not find OU ID in response"
            log_result_failure "Failed to create default organization unit"
            exit 1
        fi
    else
        log_error "Failed to fetch organization unit by handle 'default' (HTTP $HTTP_CODE)"
        log_result_failure "Failed to create default organization unit"
        exit 1
    fi
else
    log_error "Failed to create organization unit (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    log_result_failure "Failed to create default organization unit"
    exit 1
fi

echo ""

# ============================================================================
# Create Default User Type
# ============================================================================

log_info "Creating default user type (person)..."

RESPONSE=$(api_call POST "/user-types" '{
  "name": "Person",
  "ouId": "'${DEFAULT_OU_ID}'",
  "schema": {
    "username": {
      "type": "string",
      "displayName": "Username",
      "required": true,
      "unique": true
    },
    "email": {
      "type": "string",
      "displayName": "Email",
      "required": true,
      "unique": true,
      "regex": "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
    },
    "given_name": {
      "type": "string",
      "displayName": "First Name",
      "required": false
    },
    "family_name": {
      "type": "string",
      "displayName": "Last Name",
      "required": false
    },
    "mobileNumber": {
      "type": "string",
      "displayName": "Mobile Number",
      "required": false
    },
    "phone_number": {
      "type": "string",
      "displayName": "Phone Number",
      "required": false
    },
    "sub": {
      "type": "string",
      "displayName": "Subject",
      "required": false
    },
    "name": {
      "type": "string",
      "displayName": "Full Name",
      "required": false
    },
    "picture": {
      "type": "string",
      "displayName": "Picture",
      "required": false
    },
    "password": {
      "type": "string",
      "displayName": "Password",
      "required": false,
      "credential": true
    }
  },
  "systemAttributes": {
    "display": "username"
  }
}')

HTTP_CODE="${RESPONSE: -3}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "User type created successfully"
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "User type already exists, skipping"
else
    log_error "Failed to create user type (HTTP $HTTP_CODE)"
    log_result_failure "Failed to create default user type (Person)"
    exit 1
fi
log_result_success "Created default user type (Person)"

echo ""

# ============================================================================
# Create Default Agent Type
# ============================================================================

log_info "Creating default agent type..."

RESPONSE=$(api_call POST "/agent-types" '{
  "name": "default",
  "ouId": "'${DEFAULT_OU_ID}'",
  "schema": {
    "model": {
      "type": "string",
      "displayName": "Model",
      "required": false,
      "enum": ["claude-opus-4.7", "claude-opus-4.6", "claude-sonnet-4.6", "claude-sonnet-4.5", "claude-haiku-4.5", "openai-gpt-5.4", "openai-gpt-5.3", "gemini-3.5", "gemini-3.1", "gemini-3", "other"]
    },
    "department": {
      "type": "string",
      "displayName": "Department",
      "required": false
    },
    "purpose": {
      "type": "string",
      "displayName": "Purpose",
      "required": false
    }
  }
}')

HTTP_CODE="${RESPONSE: -3}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "Agent type created successfully"
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "Agent type already exists, skipping"
else
    log_error "Failed to create agent type (HTTP $HTTP_CODE)"
    log_result_failure "Failed to create default agent type"
    exit 1
fi
log_result_success "Created default agent type"

echo ""

# ============================================================================
# Create Admin User
# ============================================================================

ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}"

if [[ "$ADMIN_USERNAME" == "admin" && "$ADMIN_PASSWORD" == "admin" ]]; then
    log_warning "Using default admin credentials (admin/admin). Set ADMIN_USERNAME and ADMIN_PASSWORD or use --admin-username/--admin-password to override."
fi

log_info "Creating admin user..."

# JSON-escape username and password to safely embed arbitrary characters in the payload.
json_escape() {
    local s="$1"
    s="${s//\\/\\\\}"
    s="${s//\"/\\\"}"
    s="${s//$'\n'/\\n}"
    s="${s//$'\r'/\\r}"
    s="${s//$'\t'/\\t}"
    printf '%s' "$s"
}
ADMIN_USERNAME_JSON=$(json_escape "$ADMIN_USERNAME")
ADMIN_PASSWORD_JSON=$(json_escape "$ADMIN_PASSWORD")

RESPONSE=$(api_call POST "/users" '{
  "type": "Person",
  "ouId": "'${DEFAULT_OU_ID}'",
  "attributes": {
    "username": "'"${ADMIN_USERNAME_JSON}"'",
    "password": "'"${ADMIN_PASSWORD_JSON}"'",
    "sub": "'"${ADMIN_USERNAME_JSON}"'",
    "email": "admin@example.com",
    "name": "Administrator",
    "given_name": "Admin",
    "family_name": "User",
    "picture": "https://example.com/avatar.jpg",
    "phone_number": "+12345678920"
  }
}')

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "Admin user created successfully"
    log_info "Username: ${ADMIN_USERNAME}"

    # Extract admin user ID
    ADMIN_USER_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -z "$ADMIN_USER_ID" ]]; then
        log_warning "Could not extract admin user ID from response"
    else
        log_info "Admin user ID: $ADMIN_USER_ID"
    fi
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "Admin user already exists, retrieving user ID..."

    # Get existing admin user ID
    RESPONSE=$(api_call GET "/users")
    HTTP_CODE="${RESPONSE: -3}"
    BODY="${RESPONSE%???}"

    if [[ "$HTTP_CODE" == "200" ]]; then
        # Escape regex metacharacters so usernames like "admin.test" match literally.
        ADMIN_USERNAME_REGEX=$(printf '%s' "$ADMIN_USERNAME" | sed 's/[.[*^$()+?{|\\]/\\&/g')

        # Parse JSON to find admin user
        ADMIN_USER_ID=$(echo "$BODY" | grep -o "\"id\":\"[^\"]*\",\"[^\"]*\":\"[^\"]*\",\"attributes\":{[^}]*\"username\":\"${ADMIN_USERNAME_REGEX}\"" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

        # Fallback parsing
        if [[ -z "$ADMIN_USER_ID" ]]; then
            ADMIN_USER_ID=$(echo "$BODY" | sed 's/},{/}\n{/g' | grep "\"username\":\"${ADMIN_USERNAME_REGEX}\"" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
        fi

        if [[ -n "$ADMIN_USER_ID" ]]; then
            log_success "Found admin user ID: $ADMIN_USER_ID"
        else
            log_error "Could not find admin user in response"
            log_result_failure "Failed to create admin user"
            exit 1
        fi
    else
        log_error "Failed to fetch users (HTTP $HTTP_CODE)"
        log_result_failure "Failed to create admin user"
        exit 1
    fi
else
    log_error "Failed to create admin user (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    log_result_failure "Failed to create admin user"
    exit 1
fi
log_result_success "Created admin user"

echo ""

# ============================================================================
# Create System Resource Server
# ============================================================================

log_info "Creating system resource server..."

if [[ -z "$DEFAULT_OU_ID" ]]; then
    log_error "Default OU ID is not available. Cannot create resource server."
    log_result_failure "Failed to create system resource server"
    exit 1
fi

RESPONSE=$(api_call POST "/resource-servers" "{
  \"name\": \"System\",
  \"description\": \"System resource server\",
  \"identifier\": \"${SYSTEM_RS_IDENTIFIER}\",
  \"ouId\": \"${DEFAULT_OU_ID}\"
}")

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "Resource server created successfully"
    SYSTEM_RS_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -n "$SYSTEM_RS_ID" ]]; then
        log_info "System resource server ID: $SYSTEM_RS_ID"
        log_result_success "Created system resource server"
    else
        log_error "Could not extract resource server ID from response"
        log_result_failure "Failed to create system resource server"
        exit 1
    fi
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "Resource server already exists, retrieving ID..."
    # Get existing resource server ID
    RESPONSE=$(api_call GET "/resource-servers")
    HTTP_CODE="${RESPONSE: -3}"
    BODY="${RESPONSE%???}"

    if [[ "$HTTP_CODE" == "200" ]]; then
        SYSTEM_RS_ID=$(echo "$BODY" | grep -o '"id":"[^"]*","[^"]*":"System"' | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

        # Fallback parsing by name
        if [[ -z "$SYSTEM_RS_ID" ]]; then
            SYSTEM_RS_ID=$(echo "$BODY" | sed 's/},{/}\n{/g' | grep '"name":"System"' | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
        fi

        if [[ -n "$SYSTEM_RS_ID" ]]; then
            log_success "Found resource server ID: $SYSTEM_RS_ID"
            log_result_success "Created system resource server"
        else
            log_error "Could not find resource server ID in response"
            log_result_failure "Failed to create system resource server"
            exit 1
        fi
    else
        log_error "Failed to fetch resource servers (HTTP $HTTP_CODE)"
        log_result_failure "Failed to create system resource server"
        exit 1
    fi
else
    log_error "Failed to create resource server (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    log_result_failure "Failed to create system resource server"
    exit 1
fi

echo ""

# ============================================================================
# Create System Resource Permissions (hierarchical permission model)
# ============================================================================
#
# Permission auto-derivation:
#   Resource Server identifier (SYSTEM_RS_IDENTIFIER)
#   └── Resource handle "system"           → permission "system"
#       └── Resource handle "ou"           → permission "system:ou"
#           └── Action handle "view"       → permission "system:ou:view"
#       └── Resource handle "user"         → permission "system:user"
#           └── Action handle "view"       → permission "system:user:view"
#       └── Resource handle "group"        → permission "system:group"
#           └── Action handle "view"       → permission "system:group:view"
#       └── Resource handle "usertype"      → permission "system:usertype"
#           └── Action handle "view"       → permission "system:usertype:view"
# ============================================================================

system_permissions_failed() {
    log_result_failure "Failed to create system permissions"
    exit 1
}

log_info "Creating 'system' resource under the system resource server..."

if [[ -z "$SYSTEM_RS_ID" ]]; then
    log_error "System resource server ID is not available. Cannot create system resource."
    system_permissions_failed
fi

RESPONSE=$(api_call POST "/resource-servers/${SYSTEM_RS_ID}/resources" '{
  "name": "System",
  "description": "System resource",
  "handle": "system"
}')

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "System resource created successfully (permission: system)"
    SYSTEM_RESOURCE_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -n "$SYSTEM_RESOURCE_ID" ]]; then
        log_info "System resource ID: $SYSTEM_RESOURCE_ID"
    else
        log_error "Could not extract system resource ID from response"
        system_permissions_failed
    fi
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "System resource already exists, retrieving ID..."
    RESPONSE=$(api_call GET "/resource-servers/${SYSTEM_RS_ID}/resources")
    HTTP_CODE="${RESPONSE: -3}"
    BODY="${RESPONSE%???}"

    if [[ "$HTTP_CODE" == "200" ]]; then
        SYSTEM_RESOURCE_ID=$(echo "$BODY" | sed 's/},{/}\n{/g' | grep '"handle":"system"' | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [[ -n "$SYSTEM_RESOURCE_ID" ]]; then
            log_success "Found system resource ID: $SYSTEM_RESOURCE_ID"
        else
            log_error "Could not find system resource in response"
            system_permissions_failed
        fi
    else
        log_error "Failed to fetch resources (HTTP $HTTP_CODE)"
        system_permissions_failed
    fi
else
    log_error "Failed to create system resource (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    system_permissions_failed
fi

log_info "Creating 'ou' sub-resource under the 'system' resource..."

RESPONSE=$(api_call POST "/resource-servers/${SYSTEM_RS_ID}/resources" "{
  \"name\": \"Organization Unit\",
  \"description\": \"Organization unit resource\",
  \"handle\": \"ou\",
  \"parent\": \"${SYSTEM_RESOURCE_ID}\"
}")

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "OU resource created successfully (permission: system:ou)"
    OU_RESOURCE_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -n "$OU_RESOURCE_ID" ]]; then
        log_info "OU resource ID: $OU_RESOURCE_ID"
    else
        log_error "Could not extract OU resource ID from response"
        system_permissions_failed
    fi
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "OU resource already exists, retrieving ID..."
    RESPONSE=$(api_call GET "/resource-servers/${SYSTEM_RS_ID}/resources?parentId=${SYSTEM_RESOURCE_ID}")
    HTTP_CODE="${RESPONSE: -3}"
    BODY="${RESPONSE%???}"

    if [[ "$HTTP_CODE" == "200" ]]; then
        OU_RESOURCE_ID=$(echo "$BODY" | sed 's/},{/}\n{/g' | grep '"handle":"ou"' | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [[ -n "$OU_RESOURCE_ID" ]]; then
            log_success "Found OU resource ID: $OU_RESOURCE_ID"
        else
            log_error "Could not find OU resource in response"
            system_permissions_failed
        fi
    else
        log_error "Failed to fetch resources (HTTP $HTTP_CODE)"
        system_permissions_failed
    fi
else
    log_error "Failed to create OU resource (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    system_permissions_failed
fi

log_info "Creating 'view' action under the 'ou' resource..."

RESPONSE=$(api_call POST "/resource-servers/${SYSTEM_RS_ID}/resources/${OU_RESOURCE_ID}/actions" '{
  "name": "View",
  "description": "Read-only access to organization units",
  "handle": "view"
}')

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "OU view action created successfully (permission: system:ou:view)"
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "OU view action already exists, skipping"
else
    log_error "Failed to create OU view action (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    system_permissions_failed
fi

log_info "Creating 'user' sub-resource under the 'system' resource..."

RESPONSE=$(api_call POST "/resource-servers/${SYSTEM_RS_ID}/resources" "{
  \"name\": \"User\",
  \"description\": \"User resource\",
  \"handle\": \"user\",
  \"parent\": \"${SYSTEM_RESOURCE_ID}\"
}")

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "User resource created successfully (permission: system:user)"
    USER_RESOURCE_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -n "$USER_RESOURCE_ID" ]]; then
        log_info "User resource ID: $USER_RESOURCE_ID"
    else
        log_error "Could not extract user resource ID from response"
        system_permissions_failed
    fi
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "User resource already exists, retrieving ID..."
    RESPONSE=$(api_call GET "/resource-servers/${SYSTEM_RS_ID}/resources?parentId=${SYSTEM_RESOURCE_ID}")
    HTTP_CODE="${RESPONSE: -3}"
    BODY="${RESPONSE%???}"

    if [[ "$HTTP_CODE" == "200" ]]; then
        USER_RESOURCE_ID=$(echo "$BODY" | sed 's/},{/}\n{/g' | grep '"handle":"user"' | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [[ -n "$USER_RESOURCE_ID" ]]; then
            log_success "Found user resource ID: $USER_RESOURCE_ID"
        else
            log_error "Could not find user resource in response"
            system_permissions_failed
        fi
    else
        log_error "Failed to fetch resources (HTTP $HTTP_CODE)"
        system_permissions_failed
    fi
else
    log_error "Failed to create user resource (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    system_permissions_failed
fi

log_info "Creating 'view' action under the 'user' resource..."

RESPONSE=$(api_call POST "/resource-servers/${SYSTEM_RS_ID}/resources/${USER_RESOURCE_ID}/actions" '{
  "name": "View",
  "description": "Read-only access to users",
  "handle": "view"
}')

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "User view action created successfully (permission: system:user:view)"
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "User view action already exists, skipping"
else
    log_error "Failed to create user view action (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    system_permissions_failed
fi

log_info "Creating 'usertype' sub-resource under the 'system' resource..."

RESPONSE=$(api_call POST "/resource-servers/${SYSTEM_RS_ID}/resources" "{
  \"name\": \"User Type\",
  \"description\": \"User type resource\",
  \"handle\": \"usertype\",
  \"parent\": \"${SYSTEM_RESOURCE_ID}\"
}")

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "User type resource created successfully (permission: system:usertype)"
    USER_TYPE_RESOURCE_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -n "$USER_TYPE_RESOURCE_ID" ]]; then
        log_info "User type resource ID: $USER_TYPE_RESOURCE_ID"
    else
        log_error "Could not extract user type resource ID from response"
        system_permissions_failed
    fi
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "User type resource already exists, retrieving ID..."
    RESPONSE=$(api_call GET "/resource-servers/${SYSTEM_RS_ID}/resources?parentId=${SYSTEM_RESOURCE_ID}")
    HTTP_CODE="${RESPONSE: -3}"
    BODY="${RESPONSE%???}"

    if [[ "$HTTP_CODE" == "200" ]]; then
        USER_TYPE_RESOURCE_ID=$(echo "$BODY" | sed 's/},{/}\n{/g' | grep '"handle":"usertype"' | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [[ -n "$USER_TYPE_RESOURCE_ID" ]]; then
            log_success "Found user type resource ID: $USER_TYPE_RESOURCE_ID"
        else
            log_error "Could not find user type resource in response"
            system_permissions_failed
        fi
    else
        log_error "Failed to fetch resources (HTTP $HTTP_CODE)"
        system_permissions_failed
    fi
else
    log_error "Failed to create user type resource (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    system_permissions_failed
fi

log_info "Creating 'view' action under the 'usertype' resource..."

RESPONSE=$(api_call POST "/resource-servers/${SYSTEM_RS_ID}/resources/${USER_TYPE_RESOURCE_ID}/actions" '{
  "name": "View",
  "description": "Read-only access to user types",
  "handle": "view"
}')

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "User type view action created successfully (permission: system:usertype:view)"
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "User type view action already exists, skipping"
else
    log_error "Failed to create user type view action (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    system_permissions_failed
fi

echo ""

log_info "Creating 'group' sub-resource under the 'system' resource..."

RESPONSE=$(api_call POST "/resource-servers/${SYSTEM_RS_ID}/resources" "{
  \"name\": \"Group\",
  \"description\": \"Group resource\",
  \"handle\": \"group\",
  \"parent\": \"${SYSTEM_RESOURCE_ID}\"
}")

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "Group resource created successfully (permission: system:group)"
    GROUP_RESOURCE_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -n "$GROUP_RESOURCE_ID" ]]; then
        log_info "Group resource ID: $GROUP_RESOURCE_ID"
    else
        log_error "Could not extract group resource ID from response"
        system_permissions_failed
    fi
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "Group resource already exists, retrieving ID..."
    RESPONSE=$(api_call GET "/resource-servers/${SYSTEM_RS_ID}/resources?parentId=${SYSTEM_RESOURCE_ID}")
    HTTP_CODE="${RESPONSE: -3}"
    BODY="${RESPONSE%???}"

    if [[ "$HTTP_CODE" == "200" ]]; then
        GROUP_RESOURCE_ID=$(echo "$BODY" | sed 's/},{/}\n{/g' | grep '"handle":"group"' | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [[ -n "$GROUP_RESOURCE_ID" ]]; then
            log_success "Found group resource ID: $GROUP_RESOURCE_ID"
        else
            log_error "Could not find group resource in response"
            system_permissions_failed
        fi
    else
        log_error "Failed to fetch resources (HTTP $HTTP_CODE)"
        system_permissions_failed
    fi
else
    log_error "Failed to create group resource (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    system_permissions_failed
fi

log_info "Creating 'view' action under the 'group' resource..."

RESPONSE=$(api_call POST "/resource-servers/${SYSTEM_RS_ID}/resources/${GROUP_RESOURCE_ID}/actions" '{
  "name": "View",
  "description": "Read-only access to groups",
  "handle": "view"
}')

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "Group view action created successfully (permission: system:group:view)"
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "Group view action already exists, skipping"
else
    log_error "Failed to create group view action (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    system_permissions_failed
fi
log_result_success "Created system permissions"

echo ""

# ============================================================================
# Create Administrator Group
# ============================================================================

log_info "Creating administrator group..."

if [[ -z "$DEFAULT_OU_ID" ]]; then
    log_error "Default OU ID is not available. Cannot create administrator group."
    log_result_failure "Failed to create Administrators group"
    exit 1
fi

if [[ -z "$ADMIN_USER_ID" ]]; then
    log_error "Admin user ID is not available. Cannot create administrator group with user membership."
    log_result_failure "Failed to create Administrators group"
    exit 1
fi

RESPONSE=$(api_call POST "/groups" "{
  \"name\": \"Administrators\",
  \"description\": \"System administrators group\",
    \"ouId\": \"${DEFAULT_OU_ID}\",
    \"members\": [
        {
            \"id\": \"${ADMIN_USER_ID}\",
            \"type\": \"user\"
        }
    ]
}")

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "Administrator group created successfully"
    ADMIN_GROUP_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -n "$ADMIN_GROUP_ID" ]]; then
        log_info "Administrator group ID: $ADMIN_GROUP_ID"
        log_result_success "Created Administrators group"
    else
        log_error "Could not extract administrator group ID from response"
        log_result_failure "Failed to create Administrators group"
        exit 1
    fi
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "Administrator group already exists, retrieving ID..."
    RESPONSE=$(api_call GET "/groups/tree/default?limit=100")
    HTTP_CODE="${RESPONSE: -3}"
    BODY="${RESPONSE%???}"

    if [[ "$HTTP_CODE" == "200" ]]; then
        ADMIN_GROUP_ID=$(echo "$BODY" | sed 's/},{/}\n{/g' | grep '"name":"Administrators"' | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [[ -n "$ADMIN_GROUP_ID" ]]; then
            log_success "Found administrator group ID: $ADMIN_GROUP_ID"
            log_result_success "Created Administrators group"
        else
            log_error "Could not find administrator group in response"
            log_result_failure "Failed to create Administrators group"
            exit 1
        fi
    else
        log_error "Failed to fetch groups under default OU (HTTP $HTTP_CODE)"
        log_result_failure "Failed to create Administrators group"
        exit 1
    fi
else
    log_error "Failed to create administrator group (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    log_result_failure "Failed to create Administrators group"
    exit 1
fi

echo ""

# ============================================================================
# Create Admin Role
# ============================================================================

log_info "Creating admin role with 'system' permission..."

if [[ -z "$ADMIN_GROUP_ID" ]]; then
    log_error "Administrator group ID is not available. Cannot create role."
    log_result_failure "Failed to create Administrator role"
    exit 1
fi

if [[ -z "$DEFAULT_OU_ID" ]]; then
    log_error "Default OU ID is not available. Cannot create role."
    log_result_failure "Failed to create Administrator role"
    exit 1
fi

if [[ -z "$SYSTEM_RS_ID" ]]; then
    log_error "System resource server ID is not available. Cannot create role."
    log_result_failure "Failed to create Administrator role"
    exit 1
fi

RESPONSE=$(api_call POST "/roles" "{
  \"name\": \"Administrator\",
  \"description\": \"System administrator role with full permissions\",
  \"ouId\": \"${DEFAULT_OU_ID}\",
  \"permissions\": [
    {
      \"resourceServerId\": \"${SYSTEM_RS_ID}\",
      \"permissions\": [\"system\"]
    }
  ],
  \"assignments\": [
    {
            \"id\": \"${ADMIN_GROUP_ID}\",
            \"type\": \"group\"
    }
  ]
}")

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "Admin role created and assigned to administrator group"
    ADMIN_ROLE_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -n "$ADMIN_ROLE_ID" ]]; then
        log_info "Admin role ID: $ADMIN_ROLE_ID"
    fi
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "Admin role already exists"
else
    log_error "Failed to create admin role (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    log_result_failure "Failed to create Administrator role"
    exit 1
fi
log_result_success "Created Administrator role"

echo ""

# ============================================================================
# Create Default Flows
# ============================================================================

log_info "Creating default flows..."

# Path to flow definitions directories
AUTH_FLOWS_DIR="${SCRIPT_DIR}/flows/authentication"
REG_FLOWS_DIR="${SCRIPT_DIR}/flows/registration"
USER_ONBOARDING_FLOWS_DIR="${SCRIPT_DIR}/flows/user_onboarding"
RECOVERY_FLOWS_DIR="${SCRIPT_DIR}/flows/recovery"

# Check if flows directory exists
if [[ ! -d "$AUTH_FLOWS_DIR" ]] && [[ ! -d "$REG_FLOWS_DIR" ]] && [[ ! -d "$USER_ONBOARDING_FLOWS_DIR" ]] && [[ ! -d "$RECOVERY_FLOWS_DIR" ]]; then
    log_warning "Flow definition directories not found, skipping flow creation"
else
    FLOW_COUNT=0
    FLOW_SUCCESS=0
    FLOW_SKIPPED=0

    # Process authentication flows
    if [[ -d "$AUTH_FLOWS_DIR" ]]; then
        shopt -s nullglob
        AUTH_FILES=("$AUTH_FLOWS_DIR"/*.json)
        shopt -u nullglob

        if [[ ${#AUTH_FILES[@]} -gt 0 ]]; then
            log_info "Processing authentication flows..."
            
            # Fetch existing auth flows
            RESPONSE=$(api_call GET "/flows?flowType=AUTHENTICATION&limit=200")
            HTTP_CODE="${RESPONSE: -3}"
            BODY="${RESPONSE%???}"

            # Store existing auth flows as "handle|id" pairs
            EXISTING_AUTH_FLOWS=""
            if [[ "$HTTP_CODE" == "200" ]]; then
                while IFS= read -r line; do
                    FLOW_ID=$(echo "$line" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
                    FLOW_HANDLE=$(echo "$line" | grep -o '"handle":"[^"]*"' | cut -d'"' -f4)
                    if [[ -n "$FLOW_ID" ]] && [[ -n "$FLOW_HANDLE" ]]; then
                        EXISTING_AUTH_FLOWS="${EXISTING_AUTH_FLOWS}${FLOW_HANDLE}|${FLOW_ID}"$'\n'
                        log_debug "Found existing auth flow: handle=$FLOW_HANDLE (ID: $FLOW_ID)"
                    fi
                done < <(echo "$BODY" | grep -o '{[^}]*"id":"[^"]*"[^}]*"handle":"[^"]*"[^}]*}')
            fi
            
            log_debug "Total existing auth flows found: $(echo "$EXISTING_AUTH_FLOWS" | grep -c '|' || echo 0)"
            
            for FLOW_FILE in "$AUTH_FLOWS_DIR"/*.json; do
                [[ ! -f "$FLOW_FILE" ]] && continue

                FLOW_COUNT=$((FLOW_COUNT + 1))
                FLOW_HANDLE=$(grep -o '"handle"[[:space:]]*:[[:space:]]*"[^"]*"' "$FLOW_FILE" | head -1 | sed 's/"handle"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
                FLOW_NAME=$(grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' "$FLOW_FILE" | head -1 | sed 's/"name"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
                log_debug "Processing flow file: $FLOW_FILE with handle: $FLOW_HANDLE, name: $FLOW_NAME"
                
                # Check if flow exists by handle
                if echo "$EXISTING_AUTH_FLOWS" | grep -q "^${FLOW_HANDLE}|"; then
                    # Update existing flow
                    FLOW_ID=$(echo "$EXISTING_AUTH_FLOWS" | grep "^${FLOW_HANDLE}|" | cut -d'|' -f2)
                    log_info "Updating existing auth flow: $FLOW_NAME (handle: $FLOW_HANDLE)"
                    update_flow "$FLOW_ID" "$FLOW_FILE"
                    RESULT=$?
                    if [[ $RESULT -eq 0 ]]; then
                        FLOW_SUCCESS=$((FLOW_SUCCESS + 1))
                    fi
                else
                    # Create new flow
                    create_flow "$FLOW_FILE"
                    RESULT=$?
                    if [[ $RESULT -eq 0 ]]; then
                        FLOW_SUCCESS=$((FLOW_SUCCESS + 1))
                    elif [[ $RESULT -eq 2 ]]; then
                        FLOW_SKIPPED=$((FLOW_SKIPPED + 1))
                    fi
                fi
            done
        else
            log_warning "No authentication flow files found"
        fi
    fi

    # Process registration flows
    if [[ -d "$REG_FLOWS_DIR" ]]; then
        shopt -s nullglob
        REG_FILES=("$REG_FLOWS_DIR"/*.json)
        shopt -u nullglob
        
        if [[ ${#REG_FILES[@]} -gt 0 ]]; then
            log_info "Processing registration flows..."
            
            # Fetch existing registration flows
            RESPONSE=$(api_call GET "/flows?flowType=REGISTRATION&limit=200")
            HTTP_CODE="${RESPONSE: -3}"
            BODY="${RESPONSE%???}"

            # Store existing registration flows as "handle|id" pairs
            EXISTING_REG_FLOWS=""
            if [[ "$HTTP_CODE" == "200" ]]; then
                while IFS= read -r line; do
                    FLOW_ID=$(echo "$line" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
                    FLOW_HANDLE=$(echo "$line" | grep -o '"handle":"[^"]*"' | cut -d'"' -f4)
                    if [[ -n "$FLOW_ID" ]] && [[ -n "$FLOW_HANDLE" ]]; then
                        EXISTING_REG_FLOWS="${EXISTING_REG_FLOWS}${FLOW_HANDLE}|${FLOW_ID}"$'\n'
                    fi
                done < <(echo "$BODY" | grep -o '{[^}]*"id":"[^"]*"[^}]*"handle":"[^"]*"[^}]*}')
            fi

            for FLOW_FILE in "$REG_FLOWS_DIR"/*.json; do
                [[ ! -f "$FLOW_FILE" ]] && continue

                FLOW_COUNT=$((FLOW_COUNT + 1))
                FLOW_HANDLE=$(grep -o '"handle"[[:space:]]*:[[:space:]]*"[^"]*"' "$FLOW_FILE" | head -1 | sed 's/"handle"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
                FLOW_NAME=$(grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' "$FLOW_FILE" | head -1 | sed 's/"name"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
                
                # Check if flow exists by handle
                if echo "$EXISTING_REG_FLOWS" | grep -q "^${FLOW_HANDLE}|"; then
                    # Update existing flow
                    FLOW_ID=$(echo "$EXISTING_REG_FLOWS" | grep "^${FLOW_HANDLE}|" | cut -d'|' -f2)
                    log_info "Updating existing registration flow: $FLOW_NAME (handle: $FLOW_HANDLE)"
                    update_flow "$FLOW_ID" "$FLOW_FILE"
                    RESULT=$?
                    if [[ $RESULT -eq 0 ]]; then
                        FLOW_SUCCESS=$((FLOW_SUCCESS + 1))
                    fi
                else
                    # Create new flow
                    create_flow "$FLOW_FILE"
                    RESULT=$?
                    if [[ $RESULT -eq 0 ]]; then
                        FLOW_SUCCESS=$((FLOW_SUCCESS + 1))
                    elif [[ $RESULT -eq 2 ]]; then
                        FLOW_SKIPPED=$((FLOW_SKIPPED + 1))
                    fi
                fi
            done
        else
            log_warning "No registration flow files found"
        fi
    fi

    # Process user onboarding flows
    if [[ -d "$USER_ONBOARDING_FLOWS_DIR" ]]; then
        shopt -s nullglob
        INVITE_FILES=("$USER_ONBOARDING_FLOWS_DIR"/*.json)
        shopt -u nullglob
        
        if [[ ${#INVITE_FILES[@]} -gt 0 ]]; then
            log_info "Processing user onboarding flows..."
            
            # Fetch existing user onboarding flows
            RESPONSE=$(api_call GET "/flows?flowType=USER_ONBOARDING&limit=200")
            HTTP_CODE="${RESPONSE: -3}"
            BODY="${RESPONSE%???}"

            # Store existing user onboarding flows as "handle|id" pairs
            EXISTING_INVITE_FLOWS=""
            if [[ "$HTTP_CODE" == "200" ]]; then
                while IFS= read -r line; do
                    FLOW_ID=$(echo "$line" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
                    FLOW_HANDLE=$(echo "$line" | grep -o '"handle":"[^"]*"' | cut -d'"' -f4)
                    if [[ -n "$FLOW_ID" ]] && [[ -n "$FLOW_HANDLE" ]]; then
                        EXISTING_INVITE_FLOWS="${EXISTING_INVITE_FLOWS}${FLOW_HANDLE}|${FLOW_ID}"$'\n'
                    fi
                done < <(echo "$BODY" | grep -o '{[^}]*"id":"[^"]*"[^}]*"handle":"[^"]*"[^}]*}')
            fi

            for FLOW_FILE in "$USER_ONBOARDING_FLOWS_DIR"/*.json; do
                [[ ! -f "$FLOW_FILE" ]] && continue

                FLOW_COUNT=$((FLOW_COUNT + 1))
                FLOW_HANDLE=$(grep -o '"handle"[[:space:]]*:[[:space:]]*"[^"]*"' "$FLOW_FILE" | head -1 | sed 's/"handle"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
                FLOW_NAME=$(grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' "$FLOW_FILE" | head -1 | sed 's/"name"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
                
                # Check if flow exists by handle
                if echo "$EXISTING_INVITE_FLOWS" | grep -q "^${FLOW_HANDLE}|"; then
                    # Update existing flow
                    FLOW_ID=$(echo "$EXISTING_INVITE_FLOWS" | grep "^${FLOW_HANDLE}|" | cut -d'|' -f2)
                    log_info "Updating existing user onboarding flow: $FLOW_NAME (handle: $FLOW_HANDLE)"
                    update_flow "$FLOW_ID" "$FLOW_FILE"
                    RESULT=$?
                    if [[ $RESULT -eq 0 ]]; then
                        FLOW_SUCCESS=$((FLOW_SUCCESS + 1))
                    fi
                else
                    # Create new flow
                    create_flow "$FLOW_FILE"
                    RESULT=$?
                    if [[ $RESULT -eq 0 ]]; then
                        FLOW_SUCCESS=$((FLOW_SUCCESS + 1))
                    elif [[ $RESULT -eq 2 ]]; then
                        FLOW_SKIPPED=$((FLOW_SKIPPED + 1))
                    fi
                fi
            done
        else
            log_debug "No user onboarding flow files found"
        fi
    fi

    # Process recovery flows
    if [[ -d "$RECOVERY_FLOWS_DIR" ]]; then
        shopt -s nullglob
        RECOVERY_FILES=("$RECOVERY_FLOWS_DIR"/*.json)
        shopt -u nullglob

        if [[ ${#RECOVERY_FILES[@]} -gt 0 ]]; then
            log_info "Processing recovery flows..."

            # Fetch existing recovery flows
            RESPONSE=$(api_call GET "/flows?flowType=RECOVERY&limit=200")
            HTTP_CODE="${RESPONSE: -3}"
            BODY="${RESPONSE%???}"

            # Store existing recovery flows as "handle|id" pairs
            EXISTING_RECOVERY_FLOWS=""
            if [[ "$HTTP_CODE" == "200" ]]; then
                while IFS= read -r line; do
                    FLOW_ID=$(echo "$line" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
                    FLOW_HANDLE=$(echo "$line" | grep -o '"handle":"[^"]*"' | cut -d'"' -f4)
                    if [[ -n "$FLOW_ID" ]] && [[ -n "$FLOW_HANDLE" ]]; then
                        EXISTING_RECOVERY_FLOWS="${EXISTING_RECOVERY_FLOWS}${FLOW_HANDLE}|${FLOW_ID}"$'\n'
                    fi
                done < <(echo "$BODY" | grep -o '{[^}]*"id":"[^"]*"[^}]*"handle":"[^"]*"[^}]*}')
            fi

            for FLOW_FILE in "$RECOVERY_FLOWS_DIR"/*.json; do
                [[ ! -f "$FLOW_FILE" ]] && continue

                FLOW_COUNT=$((FLOW_COUNT + 1))
                FLOW_HANDLE=$(grep -o '"handle"[[:space:]]*:[[:space:]]*"[^"]*"' "$FLOW_FILE" | head -1 | sed 's/"handle"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
                FLOW_NAME=$(grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' "$FLOW_FILE" | head -1 | sed 's/"name"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')

                # Check if flow exists by handle
                if echo "$EXISTING_RECOVERY_FLOWS" | grep -q "^${FLOW_HANDLE}|"; then
                    # Update existing flow
                    FLOW_ID=$(echo "$EXISTING_RECOVERY_FLOWS" | grep "^${FLOW_HANDLE}|" | cut -d'|' -f2)
                    log_info "Updating existing recovery flow: $FLOW_NAME (handle: $FLOW_HANDLE)"
                    update_flow "$FLOW_ID" "$FLOW_FILE"
                    RESULT=$?
                    if [[ $RESULT -eq 0 ]]; then
                        FLOW_SUCCESS=$((FLOW_SUCCESS + 1))
                    fi
                else
                    # Create new flow
                    create_flow "$FLOW_FILE"
                    RESULT=$?
                    if [[ $RESULT -eq 0 ]]; then
                        FLOW_SUCCESS=$((FLOW_SUCCESS + 1))
                    elif [[ $RESULT -eq 2 ]]; then
                        FLOW_SKIPPED=$((FLOW_SKIPPED + 1))
                    fi
                fi
            done
        else
            log_debug "No recovery flow files found"
        fi
    fi

    if [[ $FLOW_COUNT -gt 0 ]]; then
        log_info "Flow creation summary: $FLOW_SUCCESS created/updated, $FLOW_SKIPPED skipped, $((FLOW_COUNT - FLOW_SUCCESS - FLOW_SKIPPED)) failed"
    fi
fi

echo ""

# ============================================================================
# Create Application-Specific Flows
# ============================================================================

log_info "Creating application-specific flows..."

APPS_FLOWS_DIR="${SCRIPT_DIR}/flows/apps"

# Store application flow IDs as "app_name|auth_flow_id|reg_flow_id|recovery_flow_id" pairs
APP_FLOW_IDS=""

if [[ -d "$APPS_FLOWS_DIR" ]]; then
    # Fetch all existing flows once
    log_info "Fetching existing flows for application flow processing..."
    
    # Get auth flows
    RESPONSE=$(api_call GET "/flows?flowType=AUTHENTICATION&limit=200")
    HTTP_CODE="${RESPONSE: -3}"
    BODY="${RESPONSE%???}"
    EXISTING_APP_AUTH_FLOWS=""
    if [[ "$HTTP_CODE" == "200" ]]; then
        while IFS= read -r line; do
            FLOW_ID=$(echo "$line" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
            FLOW_HANDLE=$(echo "$line" | grep -o '"handle":"[^"]*"' | cut -d'"' -f4)
            if [[ -n "$FLOW_ID" ]] && [[ -n "$FLOW_HANDLE" ]]; then
                EXISTING_APP_AUTH_FLOWS="${EXISTING_APP_AUTH_FLOWS}${FLOW_HANDLE}|${FLOW_ID}"$'\n'
            fi
        done < <(echo "$BODY" | grep -o '{[^}]*"id":"[^"]*"[^}]*"handle":"[^"]*"[^}]*}')
    fi
    
    # Get registration flows
    RESPONSE=$(api_call GET "/flows?flowType=REGISTRATION&limit=200")
    HTTP_CODE="${RESPONSE: -3}"
    BODY="${RESPONSE%???}"
    EXISTING_APP_REG_FLOWS=""
    if [[ "$HTTP_CODE" == "200" ]]; then
        while IFS= read -r line; do
            FLOW_ID=$(echo "$line" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
            FLOW_HANDLE=$(echo "$line" | grep -o '"handle":"[^"]*"' | cut -d'"' -f4)
            if [[ -n "$FLOW_ID" ]] && [[ -n "$FLOW_HANDLE" ]]; then
                EXISTING_APP_REG_FLOWS="${EXISTING_APP_REG_FLOWS}${FLOW_HANDLE}|${FLOW_ID}"$'\n'
            fi
        done < <(echo "$BODY" | grep -o '{[^}]*"id":"[^"]*"[^}]*"handle":"[^"]*"[^}]*}')
    fi

    # Get recovery flows
    RESPONSE=$(api_call GET "/flows?flowType=RECOVERY&limit=200")
    HTTP_CODE="${RESPONSE: -3}"
    BODY="${RESPONSE%???}"
    EXISTING_APP_RECOVERY_FLOWS=""
    if [[ "$HTTP_CODE" == "200" ]]; then
        while IFS= read -r line; do
            FLOW_ID=$(echo "$line" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
            FLOW_HANDLE=$(echo "$line" | grep -o '"handle":"[^"]*"' | cut -d'"' -f4)
            if [[ -n "$FLOW_ID" ]] && [[ -n "$FLOW_HANDLE" ]]; then
                EXISTING_APP_RECOVERY_FLOWS="${EXISTING_APP_RECOVERY_FLOWS}${FLOW_HANDLE}|${FLOW_ID}"$'\n'
            fi
        done < <(echo "$BODY" | grep -o '{[^}]*"id":"[^"]*"[^}]*"handle":"[^"]*"[^}]*}')
    fi

    # Process each application directory
    for APP_DIR in "$APPS_FLOWS_DIR"/*; do
        [[ ! -d "$APP_DIR" ]] && continue

        APP_NAME=$(basename "$APP_DIR")
        APP_AUTH_FLOW_ID=""
        APP_REG_FLOW_ID=""
        APP_RECOVERY_FLOW_ID=""

        log_info "Processing flows for application: $APP_NAME"

        # Process authentication flow for app
        shopt -s nullglob
        AUTH_FLOW_FILES=("$APP_DIR"/auth_*.json)
        shopt -u nullglob

        if [[ ${#AUTH_FLOW_FILES[@]} -gt 0 ]]; then
            AUTH_FLOW_FILE="${AUTH_FLOW_FILES[0]}"
            FLOW_HANDLE=$(grep -o '"handle"[[:space:]]*:[[:space:]]*"[^"]*"' "$AUTH_FLOW_FILE" | head -1 | sed 's/"handle"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
            FLOW_NAME=$(grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' "$AUTH_FLOW_FILE" | head -1 | sed 's/"name"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')

            # Check if auth flow exists by handle
            if echo "$EXISTING_APP_AUTH_FLOWS" | grep -q "^${FLOW_HANDLE}|"; then
                # Update existing flow
                APP_AUTH_FLOW_ID=$(echo "$EXISTING_APP_AUTH_FLOWS" | grep "^${FLOW_HANDLE}|" | cut -d'|' -f2)
                log_info "Updating existing auth flow: $FLOW_NAME (handle: $FLOW_HANDLE)"
                update_flow "$APP_AUTH_FLOW_ID" "$AUTH_FLOW_FILE"
            else
                # Create new flow
                APP_AUTH_FLOW_ID=$(create_flow "$AUTH_FLOW_FILE")
            fi

            # Re-fetch registration flows after creating auth flow
            if [[ -n "$APP_AUTH_FLOW_ID" ]]; then
                RESPONSE=$(api_call GET "/flows?flowType=REGISTRATION&limit=200")
                HTTP_CODE="${RESPONSE: -3}"
                BODY="${RESPONSE%???}"
                EXISTING_APP_REG_FLOWS=""
                if [[ "$HTTP_CODE" == "200" ]]; then
                    while IFS= read -r line; do
                        FLOW_ID=$(echo "$line" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
                        FLOW_HANDLE_TEMP=$(echo "$line" | grep -o '"handle":"[^"]*"' | cut -d'"' -f4)
                        if [[ -n "$FLOW_ID" ]] && [[ -n "$FLOW_HANDLE_TEMP" ]]; then
                            EXISTING_APP_REG_FLOWS="${EXISTING_APP_REG_FLOWS}${FLOW_HANDLE_TEMP}|${FLOW_ID}"$'\n'
                        fi
                    done < <(echo "$BODY" | grep -o '{[^}]*"id":"[^"]*"[^}]*"handle":"[^"]*"[^}]*}')
                fi
            fi
        else
            log_warning "No authentication flow file found for app: $APP_NAME"
        fi

        # Process registration flow for app
        shopt -s nullglob
        REG_FLOW_FILES=("$APP_DIR"/registration_*.json)
        shopt -u nullglob

        if [[ ${#REG_FLOW_FILES[@]} -gt 0 ]]; then
            REG_FLOW_FILE="${REG_FLOW_FILES[0]}"
            FLOW_HANDLE=$(grep -o '"handle"[[:space:]]*:[[:space:]]*"[^"]*"' "$REG_FLOW_FILE" | head -1 | sed 's/"handle"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
            FLOW_NAME=$(grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' "$REG_FLOW_FILE" | head -1 | sed 's/"name"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')

            # Check if registration flow exists by handle
            if echo "$EXISTING_APP_REG_FLOWS" | grep -q "^${FLOW_HANDLE}|"; then
                # Update existing flow
                APP_REG_FLOW_ID=$(echo "$EXISTING_APP_REG_FLOWS" | grep "^${FLOW_HANDLE}|" | cut -d'|' -f2)
                log_info "Updating existing registration flow: $FLOW_NAME (handle: $FLOW_HANDLE)"
                update_flow "$APP_REG_FLOW_ID" "$REG_FLOW_FILE"
            else
                # Create new flow
                APP_REG_FLOW_ID=$(create_flow "$REG_FLOW_FILE")
            fi
        else
            log_warning "No registration flow file found for app: $APP_NAME"
        fi

        # Process recovery flow for app
        shopt -s nullglob
        RECOVERY_FLOW_FILES=("$APP_DIR"/recovery_*.json)
        shopt -u nullglob

        if [[ ${#RECOVERY_FLOW_FILES[@]} -gt 0 ]]; then
            RECOVERY_FLOW_FILE="${RECOVERY_FLOW_FILES[0]}"
            FLOW_HANDLE=$(grep -o '"handle"[[:space:]]*:[[:space:]]*"[^"]*"' "$RECOVERY_FLOW_FILE" | head -1 | sed 's/"handle"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
            FLOW_NAME=$(grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' "$RECOVERY_FLOW_FILE" | head -1 | sed 's/"name"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')

            # Check if recovery flow exists by handle
            if echo "$EXISTING_APP_RECOVERY_FLOWS" | grep -q "^${FLOW_HANDLE}|"; then
                # Update existing flow
                APP_RECOVERY_FLOW_ID=$(echo "$EXISTING_APP_RECOVERY_FLOWS" | grep "^${FLOW_HANDLE}|" | cut -d'|' -f2)
                log_info "Updating existing recovery flow: $FLOW_NAME (handle: $FLOW_HANDLE)"
                update_flow "$APP_RECOVERY_FLOW_ID" "$RECOVERY_FLOW_FILE"
            else
                # Create new flow
                APP_RECOVERY_FLOW_ID=$(create_flow "$RECOVERY_FLOW_FILE")
            fi
        else
            log_debug "No recovery flow file found for app: $APP_NAME"
        fi

        # Store the flow IDs for this app
        log_debug "Storing flow IDs for $APP_NAME: auth=$APP_AUTH_FLOW_ID, reg=$APP_REG_FLOW_ID, recovery=$APP_RECOVERY_FLOW_ID"
        APP_FLOW_IDS="${APP_FLOW_IDS}${APP_NAME}|${APP_AUTH_FLOW_ID}|${APP_REG_FLOW_ID}|${APP_RECOVERY_FLOW_ID}"$'\n'
    done
else
    log_warning "Application flows directory not found at $APPS_FLOWS_DIR"
fi

log_result_success "Created default flows"

echo ""

# ============================================================================
# Create CONSOLE Application
# ============================================================================

log_info "Creating CONSOLE application..."

# Get flow IDs for console app from the APP_FLOW_IDS created/found during flow processing
CONSOLE_AUTH_FLOW_ID=$(echo "$APP_FLOW_IDS" | grep "^console|" | cut -d'|' -f2)
CONSOLE_REG_FLOW_ID=$(echo "$APP_FLOW_IDS" | grep "^console|" | cut -d'|' -f3)
CONSOLE_RECOVERY_FLOW_ID=$(echo "$APP_FLOW_IDS" | grep "^console|" | cut -d'|' -f4)
log_debug "Extracted flow IDs: auth=$CONSOLE_AUTH_FLOW_ID, reg=$CONSOLE_REG_FLOW_ID, recovery=$CONSOLE_RECOVERY_FLOW_ID"

# Validate that flow IDs are available
if [[ -z "$CONSOLE_AUTH_FLOW_ID" ]]; then
    log_error "Console authentication flow ID not found, cannot create CONSOLE application"
    log_result_failure "Failed to create Console application"
    exit 1
fi
if [[ -z "$CONSOLE_REG_FLOW_ID" ]]; then
    log_error "Console registration flow ID not found, cannot create CONSOLE application"
    log_result_failure "Failed to create Console application"
    exit 1
fi
if [[ -z "$CONSOLE_RECOVERY_FLOW_ID" ]]; then
    log_warning "Console recovery flow ID not found, recovery flow will be disabled"
fi

# Use PUBLIC_URL for redirect URIs, fallback to API_BASE if not set
PUBLIC_URL="${PUBLIC_URL:-$API_BASE}"

# Build redirect URIs array - default + custom if provided
REDIRECT_URIS="\"${PUBLIC_URL}/console\""
if [[ -n "$CUSTOM_CONSOLE_REDIRECT_URIS" ]]; then
    log_info "Adding custom redirect URIs: $CUSTOM_CONSOLE_REDIRECT_URIS"
    # Split comma-separated URIs and append to array
    IFS=',' read -ra URI_ARRAY <<< "$CUSTOM_CONSOLE_REDIRECT_URIS"
    for uri in "${URI_ARRAY[@]}"; do
        # Trim whitespace
        uri=$(echo "$uri" | xargs)
        REDIRECT_URIS="${REDIRECT_URIS},\"${uri}\""
    done
fi

PAYLOAD="{
  \"name\": \"Console\",
  \"description\": \"Management application for ${PRODUCT_NAME}\",
  \"ouId\": \"${DEFAULT_OU_ID}\",
  \"url\": \"${PUBLIC_URL}/console\",
    \"logoUrl\": \"emoji:👨‍💻\",
    \"authFlowId\": \"${CONSOLE_AUTH_FLOW_ID}\",
    \"registrationFlowId\": \"${CONSOLE_REG_FLOW_ID}\",
    \"isRegistrationFlowEnabled\": false"

# Add recovery flow fields only if recovery flow ID is provided
if [[ -n "$CONSOLE_RECOVERY_FLOW_ID" ]]; then
    PAYLOAD="${PAYLOAD},
    \"recoveryFlowId\": \"${CONSOLE_RECOVERY_FLOW_ID}\",
    \"isRecoveryFlowEnabled\": false"
fi

PAYLOAD="${PAYLOAD},
    \"allowedUserTypes\": [\"Person\"],
  \"user_attributes\": [\"given_name\",\"family_name\",\"email\",\"groups\", \"name\", \"ouId\"],
    \"inboundAuthConfig\": [{
    \"type\": \"oauth2\",
    \"config\": {
            \"clientId\": \"CONSOLE\",
            \"redirectUris\": [${REDIRECT_URIS}],
            \"grantTypes\": [\"authorization_code\", \"refresh_token\"],
            \"responseTypes\": [\"code\"],
            \"pkceRequired\": true,
            \"tokenEndpointAuthMethod\": \"none\",
            \"publicClient\": true,
      \"token\": {
                \"accessToken\": {
                    \"validityPeriod\": 3600,
                    \"userAttributes\": [\"given_name\",\"family_name\",\"email\",\"groups\", \"name\", \"ouId\"]
        },
                \"idToken\": {
                    \"validityPeriod\": 3600,
                    \"userAttributes\": [\"given_name\",\"family_name\",\"email\",\"groups\", \"name\", \"ouId\"]
        }
      },
            \"scopeClaims\": {
        \"profile\": [\"name\",\"given_name\",\"family_name\",\"picture\"],
        \"email\": [\"email\",\"email_verified\"],
        \"phone\": [\"phone_number\",\"phone_number_verified\"],
        \"group\": [\"groups\"],
        \"ou\": [\"ouId\"]
      }
    }
  }]
}"

RESPONSE=$(api_call POST "/applications" "${PAYLOAD}")

HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "CONSOLE application created successfully"
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "CONSOLE application already exists, skipping"
elif [[ "$HTTP_CODE" == "400" ]] && [[ "$BODY" =~ (Application already exists|APP-1022) ]]; then
    log_warning "CONSOLE application already exists, skipping"
else
    log_error "Failed to create CONSOLE application (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
    log_result_failure "Failed to create Console application"
    exit 1
fi
log_result_success "Created Console application"

echo ""

# ============================================================================
# Create Themes
# ============================================================================

log_info "Creating themes..."

THEMES_DIR="${SCRIPT_DIR}/themes"

if [[ ! -d "$THEMES_DIR" ]]; then
    log_warning "Themes directory not found at ${THEMES_DIR}, skipping theme creation"
else
    shopt -s nullglob
    THEME_FILES=("$THEMES_DIR"/*.json)
    shopt -u nullglob

    if [[ ${#THEME_FILES[@]} -gt 0 ]]; then
        log_info "Processing themes from ${THEMES_DIR}..."

        THEME_COUNT=0
        THEME_CREATED=0
        THEME_UPDATED=0

        for THEME_FILE in "${THEME_FILES[@]}"; do
            [[ ! -f "$THEME_FILE" ]] && continue

            THEME_COUNT=$((THEME_COUNT + 1))
            THEME_NAME=$(grep -o '"displayName"[[:space:]]*:[[:space:]]*"[^"]*"' "$THEME_FILE" | head -1 | sed 's/"displayName"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')
            if [[ -z "$THEME_NAME" ]]; then
                THEME_NAME=$(basename "$THEME_FILE" .json)
            fi
            THEME_HANDLE=$(grep -o '"handle"[[:space:]]*:[[:space:]]*"[^"]*"' "$THEME_FILE" | head -1 | sed 's/"handle"[[:space:]]*:[[:space:]]*"\([^"]*\)"/\1/')

            THEME_PAYLOAD=$(cat "$THEME_FILE")

            log_info "Creating theme: ${THEME_NAME} (from $(basename "$THEME_FILE"))"
            RESPONSE=$(api_call POST "/design/themes" "${THEME_PAYLOAD}")
            HTTP_CODE="${RESPONSE: -3}"
            BODY="${RESPONSE%???}"

            if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
                log_success "Theme '${THEME_NAME}' created successfully"
                THEME_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
                if [[ -n "$THEME_ID" ]]; then
                    log_info "Theme ID: $THEME_ID"
                fi
                THEME_CREATED=$((THEME_CREATED + 1))
            elif [[ "$HTTP_CODE" == "409" ]] || (echo "$BODY" | grep -q '"THM-1015"'); then
                log_warning "Theme '${THEME_NAME}' already exists, updating..."
                RESPONSE=$(api_call GET "/design/themes")
                HTTP_CODE="${RESPONSE: -3}"
                BODY="${RESPONSE%???}"
                THEME_ID=$(echo "$BODY" | grep -o '"id":"[^"]*","handle":"'"${THEME_HANDLE}"'"' | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
                if [[ -z "$THEME_ID" ]]; then
                    log_error "Failed to retrieve existing theme ID for '${THEME_NAME}'"
                    exit 1
                fi
                log_info "Found existing theme ID: $THEME_ID"
                RESPONSE=$(api_call PUT "/design/themes/${THEME_ID}" "${THEME_PAYLOAD}")
                HTTP_CODE="${RESPONSE: -3}"
                BODY="${RESPONSE%???}"
                if [[ "$HTTP_CODE" == "200" ]]; then
                    log_success "Theme '${THEME_NAME}' updated successfully"
                    THEME_UPDATED=$((THEME_UPDATED + 1))
                else
                    log_error "Failed to update theme '${THEME_NAME}' (HTTP $HTTP_CODE)"
                    echo "Response: $BODY"
                    exit 1
                fi
            else
                log_error "Failed to create theme '${THEME_NAME}' (HTTP $HTTP_CODE)"
                echo "Response: $BODY"
                exit 1
            fi
        done

        echo ""
        log_info "Theme creation summary: ${THEME_CREATED} created, ${THEME_UPDATED} updated (Total: ${THEME_COUNT})"
    else
        log_warning "No theme files found in ${THEMES_DIR}"
    fi
fi
log_result_success "Created themes"

echo ""

# ============================================================================
# Seed i18n Translations
# ============================================================================

log_info "Seeding i18n translations..."

I18N_DIR="${SCRIPT_DIR}/i18n"

if [[ ! -d "$I18N_DIR" ]]; then
    log_warning "i18n directory not found at ${I18N_DIR}, skipping translation seeding"
else
    shopt -s nullglob
    I18N_FILES=("$I18N_DIR"/*.json)
    shopt -u nullglob

    if [[ ${#I18N_FILES[@]} -gt 0 ]]; then
        log_info "Processing i18n translations from ${I18N_DIR}..."

        I18N_COUNT=0
        I18N_SUCCESS=0

        for I18N_FILE in "${I18N_FILES[@]}"; do
            [[ ! -f "$I18N_FILE" ]] && continue

            I18N_COUNT=$((I18N_COUNT + 1))

            # Extract language from filename (e.g., en-US.json -> en-US)
            LANGUAGE=$(basename "$I18N_FILE" .json)

            log_info "Seeding translations for language: ${LANGUAGE} (from $(basename "$I18N_FILE"))"

            PAYLOAD=$(cat "$I18N_FILE")

            RESPONSE=$(api_call POST "/i18n/languages/${LANGUAGE}/translations" "$PAYLOAD")
            HTTP_CODE="${RESPONSE: -3}"
            BODY="${RESPONSE%???}"

            if [[ "$HTTP_CODE" == "200" ]]; then
                TOTAL=$(echo "$BODY" | grep -o '"totalResults":[0-9]*' | cut -d':' -f2)
                log_success "Translations for '${LANGUAGE}' seeded successfully (${TOTAL:-?} translations)"
                I18N_SUCCESS=$((I18N_SUCCESS + 1))
            else
                log_error "Failed to seed translations for '${LANGUAGE}' (HTTP $HTTP_CODE)"
                log_error "Response: $BODY"
                log_result_failure "Failed to seed i18n translations"
                exit 1
            fi
        done

        echo ""
        log_info "Translation seeding summary: ${I18N_SUCCESS} seeded (Total: ${I18N_COUNT})"
    else
        log_warning "No i18n translation files found in ${I18N_DIR}"
    fi
fi
log_result_success "Seeded i18n translations"

echo ""

# ============================================================================
# Summary
# ============================================================================

log_success "Default resources setup completed successfully!"
echo ""
log_info "👤 Admin credentials:"
log_info "   Username: admin"
log_info "   Password: admin"
log_info "   Role: Administrator (system permission via Administrators group)"
echo ""
