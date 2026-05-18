#!/usr/bin/env pwsh
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

$PRODUCT_NAME = "ThunderID"

# Check for PowerShell Version Compatibility
if ($PSVersionTable.PSVersion.Major -lt 7) {
    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Red
    Write-Host " [ERROR] UNSUPPORTED POWERSHELL VERSION" -ForegroundColor Red
    Write-Host "================================================================" -ForegroundColor Red
    Write-Host ""
    Write-Host " You are currently running PowerShell $($PSVersionTable.PSVersion.ToString())" -ForegroundColor Yellow
    Write-Host " $PRODUCT_NAME requires PowerShell 7 (Core) or later." -ForegroundColor Yellow
    Write-Host ""
    Write-Host " Please install the latest version from:"
    Write-Host " https://github.com/PowerShell/PowerShell" -ForegroundColor Cyan
    Write-Host ""
    exit 1
}

# Bootstrap Script: Sample Resources Setup
# Creates resources required to run the product sample experience

$ErrorActionPreference = 'Stop'

# Dot-source common functions from the same directory as this script
. "$PSScriptRoot/common.ps1"

Log-Info "Creating sample $PRODUCT_NAME resources..."
Write-Host ""

# ============================================================================
# Create Customers Organization Unit
# ============================================================================

$customerOuHandle = "customers"

Log-Info "Creating Customers organization unit..."

$customerOuData = @{
    handle = $customerOuHandle
    name = "Customers"
    description = "Organization unit for customer accounts"
    logoUrl = "emoji:🏛️"
} | ConvertTo-Json -Depth 5

$response = Invoke-Api -Method POST -Endpoint "/organization-units" -Data $customerOuData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "Customers organization unit created successfully"
    $body = $response.Body | ConvertFrom-Json
    $CUSTOMER_OU_ID = $body.id
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "Customers organization unit already exists, retrieving ID..."
    # Get existing OU ID by handle to ensure we get the correct "customers" OU
    $response = Invoke-Api -Method GET -Endpoint "/organization-units/tree/$customerOuHandle"
    if ($response.StatusCode -eq 200) {
        $body = $response.Body | ConvertFrom-Json
        $CUSTOMER_OU_ID = $body.id
    }
    else {
        Log-Error "Failed to fetch organization unit by handle '$customerOuHandle' (HTTP $($response.StatusCode))"
        Write-Host "Response: $($response.Body)"
        Log-Result-Failure "Failed to create Customers organization unit"
        exit 1
    }
}
else {
    Log-Error "Failed to create Customers organization unit (HTTP $($response.StatusCode))"
    Write-Host "Response: $($response.Body)"
    Log-Result-Failure "Failed to create Customers organization unit"
    exit 1
}

if (-not $CUSTOMER_OU_ID) {
    Log-Error "Could not determine Customers organization unit ID"
    Log-Result-Failure "Failed to create Customers organization unit"
    exit 1
}

Log-Info "Customers OU ID: $CUSTOMER_OU_ID"
Log-Result-Success "Created Customers organization unit"

Write-Host ""

# ============================================================================
# Create Customer User Type
# ============================================================================

Log-Info "Creating Customer user type..."

$customerUserTypeData = ([ordered]@{
    name = "Customer"
    ouId = $CUSTOMER_OU_ID
    allowSelfRegistration = $true
    schema = [ordered]@{
        username = @{
            type = "string"
            displayName = "Username"
            required = $true
            unique = $true
        }
        password = @{
            type = "string"
            displayName = "Password"
            required = $false
            credential = $true
        }
        email = @{
            type = "string"
            displayName = "Email"
            required = $true
            unique = $true
            regex = "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$"
        }
        given_name = @{
            type = "string"
            displayName = "First Name"
            required = $false
        }
        family_name = @{
            type = "string"
            displayName = "Last Name"
            required = $false
        }
        name = @{
            type = "string"
            displayName = "Full Name"
            required = $false
        }
        mobileNumber = @{
            type = "string"
            displayName = "Mobile Number"
            required = $false
        }
    }
    systemAttributes = [ordered]@{
        display = "username"
    }
} | ConvertTo-Json -Depth 5)

$response = Invoke-Api -Method POST -Endpoint "/user-types" -Data $customerUserTypeData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "Customer user type created successfully"
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "Customer user type already exists, skipping"
}
else {
    Log-Error "Failed to create Customer user type (HTTP $($response.StatusCode))"
    Write-Host "Response: $($response.Body)"
    Log-Result-Failure "Failed to create Customer user type"
    exit 1
}

Log-Result-Success "Created Customer user type"

Write-Host ""

# ============================================================================
# Create Sample Application
# ============================================================================

Log-Info "Creating Sample App application..."

$appData = @{
    name = "Sample App"
    description = "Sample application for testing"
    ouId = $CUSTOMER_OU_ID
    url = "https://localhost:3000"
    logoUrl = "emoji:🎁"
    tosUri = "https://localhost:3000/terms"
    policyUri = "https://localhost:3000/privacy"
    contacts = @("admin@example.com", "support@example.com")
    isRegistrationFlowEnabled = $true
    userAttributes = @("given_name","family_name","email","groups")
    allowedUserTypes = @("Customer")
    inboundAuthConfig = @(
        @{
            type = "oauth2"
            config = @{
                clientId = "sample_app_client"
                redirectUris = @("https://localhost:3000")
                grantTypes = @("authorization_code")
                responseTypes = @("code")
                tokenEndpointAuthMethod = "none"
                pkceRequired = $true
                publicClient = $true
                scopes = @("openid", "profile", "email")
                token = @{
                    accessToken = @{
                        validityPeriod = 3600
                        userAttributes = @("given_name","family_name","email","groups")
                    }
                    idToken = @{
                        validityPeriod = 3600
                        userAttributes = @("given_name","family_name","email","groups")
                    }
                }
                scopeClaims = @{
                    profile = @("name","given_name","family_name","picture")
                    email = @("email","email_verified")
                    phone = @("phone_number","phone_number_verified")
                    group = @("groups")
                }
            }
        }
    )
} | ConvertTo-Json -Depth 15

$response = Invoke-Api -Method POST -Endpoint "/applications" -Data $appData

if ($response.StatusCode -in 200, 201, 202) {
    Log-Success "Sample App created successfully"
    $body = $response.Body | ConvertFrom-Json
    $sampleAppId = $body.id
    if ($sampleAppId) {
        Log-Info "Sample App ID: $sampleAppId"
    }
    else {
        Log-Warning "Could not extract Sample App ID from response"
    }
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "Sample App already exists, skipping"
}
elseif ($response.StatusCode -eq 400 -and ($response.Body -match "Application already exists|APP-1022")) {
    Log-Warning "Sample App already exists, skipping"
}
else {
    Log-Error "Failed to create Sample App (HTTP $($response.StatusCode))"
    Write-Host "Response: $($response.Body)"
    Log-Result-Failure "Failed to create Sample App"
    exit 1
}

Log-Result-Success "Created Sample App"

Write-Host ""

# ============================================================================
# Create React SDK Sample Application
# ============================================================================

Log-Info "Creating React SDK Sample App application..."

$reactSdkAppData = @{
    name = "React SDK Sample"
    description = "Sample React application using $PRODUCT_NAME React SDK"
    ouId = $CUSTOMER_OU_ID
    clientId = "REACT_SDK_SAMPLE"
    url = "https://localhost:3000"
    logoUrl = "emoji:🛍️"
    tosUri = "https://localhost:3000/terms"
    policyUri = "https://localhost:3000/privacy"
    contacts = @("admin@example.com")
    isRegistrationFlowEnabled = $true
    assertion = @{
        validityPeriod = 3600
        userAttributes = $null
    }
    userAttributes = @("given_name","family_name","email","groups","name")
    allowedUserTypes = @("Customer")
    inboundAuthConfig = @(
        @{
            type = "oauth2"
            config = @{
                clientId = "REACT_SDK_SAMPLE"
                redirectUris = @("https://localhost:3000")
                grantTypes = @("authorization_code")
                responseTypes = @("code")
                tokenEndpointAuthMethod = "none"
                pkceRequired = $true
                publicClient = $true
                token = @{
                    accessToken = @{
                        validityPeriod = 3600
                        userAttributes = @("given_name","family_name","email","groups","name")
                    }
                    idToken = @{
                        validityPeriod = 3600
                        userAttributes = @("given_name","family_name","email","groups","name")
                    }
                }
                scopeClaims = @{
                    email = @("email","email_verified")
                    group = @("groups")
                    phone = @("phone_number","phone_number_verified")
                    profile = @("name","given_name","family_name","picture")
                }
            }
        }
    )
} | ConvertTo-Json -Depth 15

$response = Invoke-Api -Method POST -Endpoint "/applications" -Data $reactSdkAppData

if ($response.StatusCode -in 200, 201, 202) {
    Log-Success "React SDK Sample App created successfully"
    $body = $response.Body | ConvertFrom-Json
    $reactSdkAppId = $body.id
    if ($reactSdkAppId) {
        Log-Info "React SDK Sample App ID: $reactSdkAppId"
    }
    else {
        Log-Warning "Could not extract React SDK Sample App ID from response"
    }
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "React SDK Sample App already exists, skipping"
}
elseif ($response.StatusCode -eq 400 -and ($response.Body -match "Application already exists|APP-1022")) {
    Log-Warning "React SDK Sample App already exists, skipping"
}
else {
    Log-Error "Failed to create React SDK Sample App (HTTP $($response.StatusCode))"
    Write-Host "Response: $($response.Body)"
    Log-Result-Failure "Failed to create React SDK Sample App"
    exit 1
}

Log-Result-Success "Created React SDK Sample App"

Write-Host ""

# ============================================================================
# Summary
# ============================================================================

Log-Success "Sample resources setup completed successfully!"
Log-Result-Success "Created sample resources"
Write-Host ""
