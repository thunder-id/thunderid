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

# Bootstrap Script: Sample Resources Setup
# Creates resources required to run the product sample experience

set -e

# Source common functions from the same directory as this script
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]:-$0}")"
source "${SCRIPT_DIR}/common.sh"

log_info "Creating sample ${PRODUCT_NAME} resources..."
echo ""

# ============================================================================
# Create Customers Organization Unit
# ============================================================================

CUSTOMER_OU_HANDLE="customers"

log_info "Creating Customers organization unit..."

read -r -d '' CUSTOMERS_OU_PAYLOAD <<JSON || true
{
  "handle": "${CUSTOMER_OU_HANDLE}",
  "name": "Customers",
  "description": "Organization unit for customer accounts",
  "logoUrl": "emoji:🏛️"
}
JSON

RESPONSE=$(api_call POST "/organization-units" "${CUSTOMERS_OU_PAYLOAD}")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "Customers organization unit created successfully"
    CUSTOMER_OU_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "Customers organization unit already exists, retrieving ID..."
    # Get existing OU ID by handle to ensure we get the correct "customers" OU
    RESPONSE=$(api_call GET "/organization-units/tree/${CUSTOMER_OU_HANDLE}")
    HTTP_CODE="${RESPONSE: -3}"
    BODY="${RESPONSE%???}"

    if [[ "$HTTP_CODE" == "200" ]]; then
        CUSTOMER_OU_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    else
        log_error "Failed to fetch organization unit by handle '${CUSTOMER_OU_HANDLE}' (HTTP $HTTP_CODE)"
        echo "Response: $BODY"
        exit 1
    fi
else
    log_error "Failed to create Customers organization unit (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
  log_result_failure "Failed to create Customers organization unit"
    exit 1
fi

if [[ -z "$CUSTOMER_OU_ID" ]]; then
    log_error "Could not determine Customers organization unit ID"
  log_result_failure "Failed to create Customers organization unit"
    exit 1
fi

log_info "Customers OU ID: $CUSTOMER_OU_ID"
log_result_success "Created Customers organization unit"

echo ""

# ============================================================================
# Create Customer User Type
# ============================================================================

log_info "Creating Customer user type..."

read -r -d '' CUSTOMER_USER_TYPE_PAYLOAD <<JSON || true
{
  "name": "Customer",
  "ouId": "${CUSTOMER_OU_ID}",
  "allowSelfRegistration": true,
  "schema": {
    "username": {
      "type": "string",
      "displayName": "Username",
      "required": true,
      "unique": true
    },
    "password": {
      "type": "string",
      "displayName": "Password",
      "required": false,
      "credential": true
    },
    "email": {
      "type": "string",
      "displayName": "Email",
      "required": true,
      "unique": true,
      "regex": "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\\\.[a-zA-Z]{2,}$"
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
    "name": {
      "type": "string",
      "displayName": "Full Name",
      "required": false
    },
    "mobileNumber": {
      "type": "string",
      "displayName": "Mobile Number",
      "required": false
    }
  },
  "systemAttributes": {
    "display": "username"
  }
}
JSON

RESPONSE=$(api_call POST "/user-types" "${CUSTOMER_USER_TYPE_PAYLOAD}")
HTTP_CODE="${RESPONSE: -3}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]]; then
    log_success "Customer user type created successfully"
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "Customer user type already exists, skipping"
else
    log_error "Failed to create Customer user type (HTTP $HTTP_CODE)"
  log_result_failure "Failed to create Customer user type"
    exit 1
fi
log_result_success "Created Customer user type"

echo ""

# ============================================================================
# Create Sample Application
# ============================================================================

log_info "Creating Sample App application..."

read -r -d '' SAMPLE_APP_PAYLOAD <<JSON || true
{
  "name": "Sample App",
  "description": "Sample application for testing",
  "ouId": "${CUSTOMER_OU_ID}",
  "url": "https://localhost:3000",
  "logoUrl": "emoji:🎁",
  "tosUri": "https://localhost:3000/terms",
  "policyUri": "https://localhost:3000/privacy",
  "contacts": ["admin@example.com", "support@example.com"],
  "isRegistrationFlowEnabled": true,
  "userAttributes": ["given_name","family_name","email","groups"],
  "allowedUserTypes": ["Customer"],
  "inboundAuthConfig": [{
    "type": "oauth2",
    "config": {
      "clientId": "sample_app_client",
      "redirectUris": ["https://localhost:3000"],
      "grantTypes": ["authorization_code"],
      "responseTypes": ["code"],
      "tokenEndpointAuthMethod": "none",
      "pkceRequired": true,
      "publicClient": true,
      "scopes": ["openid", "profile", "email"],
      "token": {
        "accessToken": {
          "validityPeriod": 3600,
          "userAttributes": ["given_name","family_name","email","groups"]
        },
        "idToken": {
          "validityPeriod": 3600,
          "userAttributes": ["given_name","family_name","email","groups"]
        }
      },
      "scopeClaims": {
        "profile": ["name","given_name","family_name","picture"],
        "email": ["email","email_verified"],
        "phone": ["phone_number","phone_number_verified"],
        "group": ["groups"]
      }
    }
  }]
}
JSON

RESPONSE=$(api_call POST "/applications" "${SAMPLE_APP_PAYLOAD}")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]] || [[ "$HTTP_CODE" == "202" ]]; then
    log_success "Sample App created successfully"
    SAMPLE_APP_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -n "$SAMPLE_APP_ID" ]]; then
        log_info "Sample App ID: $SAMPLE_APP_ID"
    else
        log_warning "Could not extract Sample App ID from response"
    fi
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "Sample App already exists, skipping"
elif [[ "$HTTP_CODE" == "400" ]] && [[ "$BODY" =~ (Application already exists|APP-1022) ]]; then
    log_warning "Sample App already exists, skipping"
else
    log_error "Failed to create Sample App (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
  log_result_failure "Failed to create Sample App"
    exit 1
fi
log_result_success "Created Sample App"

echo ""

# ============================================================================
# Create React SDK Sample Application
# ============================================================================

log_info "Creating React SDK Sample App application..."

read -r -d '' REACT_SDK_APP_PAYLOAD <<JSON || true
{
  "name": "React SDK Sample",
  "description": "Sample React application using ${PRODUCT_NAME} React SDK",
  "ouId": "${CUSTOMER_OU_ID}",
  "clientId": "REACT_SDK_SAMPLE",
  "url": "https://localhost:3000",
  "logoUrl": "emoji:🛍️",
  "tosUri": "https://localhost:3000/terms",
  "policyUri": "https://localhost:3000/privacy",
  "contacts": ["admin@example.com"],
  "isRegistrationFlowEnabled": true,
  "assertion": {
    "validityPeriod": 3600,
    "userAttributes": null
  },
  "userAttributes": ["given_name","family_name","email","groups","name"],
  "allowedUserTypes": ["Customer"],
  "inboundAuthConfig": [{
    "type": "oauth2",
    "config": {
      "clientId": "REACT_SDK_SAMPLE",
      "redirectUris": ["https://localhost:3000"],
      "grantTypes": ["authorization_code"],
      "responseTypes": ["code"],
      "tokenEndpointAuthMethod": "none",
      "pkceRequired": true,
      "publicClient": true,
      "token": {
        "accessToken": {
          "validityPeriod": 3600,
          "userAttributes": ["given_name","family_name","email","groups","name"]
        },
        "idToken": {
          "validityPeriod": 3600,
          "userAttributes": ["given_name","family_name","email","groups","name"]
        }
      },
      "scopeClaims": {
        "email": ["email","email_verified"],
        "group": ["groups"],
        "phone": ["phone_number","phone_number_verified"],
        "profile": ["name","given_name","family_name","picture"]
      }
    }
  }]
}
JSON

RESPONSE=$(api_call POST "/applications" "${REACT_SDK_APP_PAYLOAD}")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"

if [[ "$HTTP_CODE" == "201" ]] || [[ "$HTTP_CODE" == "200" ]] || [[ "$HTTP_CODE" == "202" ]]; then
    log_success "React SDK Sample App created successfully"
    REACT_SDK_APP_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -n "$REACT_SDK_APP_ID" ]]; then
        log_info "React SDK Sample App ID: $REACT_SDK_APP_ID"
    else
        log_warning "Could not extract React SDK Sample App ID from response"
    fi
elif [[ "$HTTP_CODE" == "409" ]]; then
    log_warning "React SDK Sample App already exists, skipping"
elif [[ "$HTTP_CODE" == "400" ]] && [[ "$BODY" =~ (Application already exists|APP-1022) ]]; then
    log_warning "React SDK Sample App already exists, skipping"
else
    log_error "Failed to create React SDK Sample App (HTTP $HTTP_CODE)"
    echo "Response: $BODY"
  log_result_failure "Failed to create React SDK Sample App"
    exit 1
fi
log_result_success "Created React SDK Sample App"

echo ""

# ============================================================================
# Summary
# ============================================================================

log_success "Sample resources setup completed successfully!"
echo ""
