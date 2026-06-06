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

# Parse command line arguments for custom redirect URIs
param(
    [string]$ConsoleRedirectUris = ""
)

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

# Bootstrap Script: Default Resources Setup
# Creates default organization unit, user type, admin user, system resource server, system action, admin role, and Console application


$ErrorActionPreference = 'Stop'

# Dot-source common functions from the same directory as this script
. "$PSScriptRoot/common.ps1"

Log-Info "Creating default $PRODUCT_NAME resources..."
Write-Host ""

$SystemRsIdentifier = "https://localhost:8090/mcp"

# ============================================================================
# Create Default Organization Unit
# ============================================================================

Log-Info "Creating default organization unit..."

$response = Invoke-Api -Method POST -Endpoint "/organization-units" -Data '{
  "handle": "default",
  "name": "Default",
  "description": "Default organization unit",
  "logoUrl": "emoji:🏛️"
}'

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "Organization unit created successfully"
    $body = $response.Body | ConvertFrom-Json
    $DEFAULT_OU_ID = $body.id
    if ($DEFAULT_OU_ID) {
        Log-Info "Default OU ID: $DEFAULT_OU_ID"
        Log-Result-Success "Created default organization unit"
    }
    else {
        Log-Error "Could not extract OU ID from response"
        Log-Result-Failure "Failed to create default organization unit"
        exit 1
    }
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "Organization unit already exists, retrieving OU ID..."
    # Get existing OU ID by handle to ensure we get the correct "default" OU
    $response = Invoke-Api -Method GET -Endpoint "/organization-units/tree/default"

    if ($response.StatusCode -eq 200) {
        $body = $response.Body | ConvertFrom-Json
        $DEFAULT_OU_ID = $body.id
        if ($DEFAULT_OU_ID) {
            Log-Success "Found OU ID: $DEFAULT_OU_ID"
            Log-Result-Success "Created default organization unit"
        }
        else {
            Log-Error "Could not find OU ID in response"
            Log-Result-Failure "Failed to create default organization unit"
            exit 1
        }
    }
    else {
        Log-Error "Failed to fetch organization unit by handle 'default' (HTTP $($response.StatusCode))"
        Log-Result-Failure "Failed to create default organization unit"
        exit 1
    }
}
else {
    Log-Error "Failed to create organization unit (HTTP $($response.StatusCode))"
    Write-Host "Response: $($response.Body)"
    Log-Result-Failure "Failed to create default organization unit"
    exit 1
}

Write-Host ""

# ============================================================================
# Create Default User Type
# ============================================================================

Log-Info "Creating default user type (person)..."

$userTypeData = ([ordered]@{
    name = "Person"
    ouId = $DEFAULT_OU_ID
    schema = [ordered]@{
        username = @{
            type = "string"
            displayName = "Username"
            required = $true
            unique = $true
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
        mobileNumber = @{
            type = "string"
            displayName = "Mobile Number"
            required = $false
        }
        phone_number = @{
            type = "string"
            displayName = "Phone Number"
            required = $false
        }
        sub = @{
            type = "string"
            displayName = "Subject"
            required = $false
        }
        name = @{
            type = "string"
            displayName = "Full Name"
            required = $false
        }
        picture = @{
            type = "string"
            displayName = "Picture"
            required = $false
        }
        password = @{
            type = "string"
            displayName = "Password"
            required = $false
            credential = $true
        }
    }
    systemAttributes = [ordered]@{
        display = "username"
    }
} | ConvertTo-Json -Depth 5)

$response = Invoke-Api -Method POST -Endpoint "/user-types" -Data $userTypeData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "User type created successfully"
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "User type already exists, skipping"
}
else {
    Log-Error "Failed to create user type (HTTP $($response.StatusCode))"
    Log-Result-Failure "Failed to create default user type (Person)"
    exit 1
}

Log-Result-Success "Created default user type (Person)"

Write-Host ""

# ============================================================================
# Create Default Agent Type
# ============================================================================

Log-Info "Creating default agent type..."

$agentTypeData = ([ordered]@{
    name = "default"
    ouId = $DEFAULT_OU_ID
    schema = [ordered]@{
        model = @{
            type = "string"
            displayName = "Model"
            required = $false
            enum = @("claude-opus-4.7", "claude-opus-4.6", "claude-sonnet-4.6", "claude-sonnet-4.5", "claude-haiku-4.5", "openai-gpt-5.4", "openai-gpt-5.3", "gemini-3.5", "gemini-3.1", "gemini-3", "other")
        }
        department = @{
            type = "string"
            displayName = "Department"
            required = $false
        }
        purpose = @{
            type = "string"
            displayName = "Purpose"
            required = $false
        }
    }
} | ConvertTo-Json -Depth 5)

$response = Invoke-Api -Method POST -Endpoint "/agent-types" -Data $agentTypeData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "Agent type created successfully"
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "Agent type already exists, skipping"
}
else {
    Log-Error "Failed to create agent type (HTTP $($response.StatusCode))"
    Log-Result-Failure "Failed to create default agent type"
    exit 1
}

Log-Result-Success "Created default agent type"

Write-Host ""

# ============================================================================
# Create Admin User
# ============================================================================

Log-Info "Creating admin user..."

$adminUserData = ([ordered]@{
    type = "Person"
    ouId = $DEFAULT_OU_ID
    attributes = @{
        username = "admin"
        password = "admin"
        sub = "admin"
        email = "admin@example.com"
        name = "Administrator"
        given_name = "Admin"
        family_name = "User"
        picture = "https://example.com/avatar.jpg"
        phone_number = "+12345678920"
    }
} | ConvertTo-Json -Depth 5)

$response = Invoke-Api -Method POST -Endpoint "/users" -Data $adminUserData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "Admin user created successfully"
    Log-Info "Username: admin"
    Log-Info "Password: admin"

    # Extract admin user ID
    $body = $response.Body | ConvertFrom-Json
    $ADMIN_USER_ID = $body.id
    if (-not $ADMIN_USER_ID) {
        Log-Warning "Could not extract admin user ID from response"
    }
    else {
        Log-Info "Admin user ID: $ADMIN_USER_ID"
    }
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "Admin user already exists, retrieving user ID..."

    # Get existing admin user ID
    $response = Invoke-Api -Method GET -Endpoint "/users"

    if ($response.StatusCode -eq 200) {
        # Parse JSON to find admin user
        $body = $response.Body | ConvertFrom-Json
        $adminUser = $body.users | Where-Object { $_.attributes.username -eq "admin" } | Select-Object -First 1

        if ($adminUser) {
            $ADMIN_USER_ID = $adminUser.id
            Log-Success "Found admin user ID: $ADMIN_USER_ID"
        }
        else {
            Log-Error "Could not find admin user in response"
            Log-Result-Failure "Failed to create admin user"
            exit 1
        }
    }
    else {
        Log-Error "Failed to fetch users (HTTP $($response.StatusCode))"
        Log-Result-Failure "Failed to create admin user"
        exit 1
    }
}
else {
    Log-Error "Failed to create admin user (HTTP $($response.StatusCode))"
    Write-Host "Response: $($response.Body)"
    Log-Result-Failure "Failed to create admin user"
    exit 1
}

Log-Result-Success "Created admin user"

Write-Host ""

# ============================================================================
# Create System Resource Server
# ============================================================================

Log-Info "Creating system resource server..."

if (-not $DEFAULT_OU_ID) {
    Log-Error "Default OU ID is not available. Cannot create resource server."
    Log-Result-Failure "Failed to create system resource server"
    exit 1
}

$resourceServerData = @{
    name = "System"
    description = "System resource server"
    identifier = $SystemRsIdentifier
    ouId = $DEFAULT_OU_ID
} | ConvertTo-Json -Depth 10

$response = Invoke-Api -Method POST -Endpoint "/resource-servers" -Data $resourceServerData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "Resource server created successfully"
    $body = $response.Body | ConvertFrom-Json
    $SYSTEM_RS_ID = $body.id
    if ($SYSTEM_RS_ID) {
        Log-Info "System resource server ID: $SYSTEM_RS_ID"
        Log-Result-Success "Created system resource server"
    }
    else {
        Log-Error "Could not extract resource server ID from response"
        Log-Result-Failure "Failed to create system resource server"
        exit 1
    }
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "Resource server already exists, retrieving ID..."
    # Get existing resource server ID
    $response = Invoke-Api -Method GET -Endpoint "/resource-servers"

    if ($response.StatusCode -eq 200) {
        $body = $response.Body | ConvertFrom-Json
        $systemRS = $body.resourceServers | Where-Object { $_.name -eq "System" } | Select-Object -First 1

        if ($systemRS) {
            $SYSTEM_RS_ID = $systemRS.id
            Log-Success "Found resource server ID: $SYSTEM_RS_ID"
            Log-Result-Success "Created system resource server"
        }
        else {
            Log-Error "Could not find resource server ID in response"
            Log-Result-Failure "Failed to create system resource server"
            exit 1
        }
    }
    else {
        Log-Error "Failed to fetch resource servers (HTTP $($response.StatusCode))"
        Log-Result-Failure "Failed to create system resource server"
        exit 1
    }
}
else {
    Log-Error "Failed to create resource server (HTTP $($response.StatusCode))"
    Write-Host "Response: $($response.Body)"
    Log-Result-Failure "Failed to create system resource server"
    exit 1
}

Write-Host ""

# ============================================================================
# Create System Resource Permissions (hierarchical permission model)
# ============================================================================
#
# Permission auto-derivation:
#   Resource Server identifier ($SystemRsIdentifier)
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

function System-Permissions-Failed {
    Log-Result-Failure "Failed to create system permissions"
    exit 1
}

Log-Info "Creating 'system' resource under the system resource server..."

if (-not $SYSTEM_RS_ID) {
    Log-Error "System resource server ID is not available. Cannot create system resource."
    System-Permissions-Failed
}

$systemResourceData = @{
    name        = "System"
    description = "System resource"
    handle      = "system"
} | ConvertTo-Json -Depth 10

$response = Invoke-Api -Method POST -Endpoint "/resource-servers/$SYSTEM_RS_ID/resources" -Data $systemResourceData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "System resource created successfully (permission: system)"
    $body = $response.Body | ConvertFrom-Json
    $SYSTEM_RESOURCE_ID = $body.id
    if ($SYSTEM_RESOURCE_ID) {
        Log-Info "System resource ID: $SYSTEM_RESOURCE_ID"
    }
    else {
        Log-Error "Could not extract system resource ID from response"
        System-Permissions-Failed
    }
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "System resource already exists, retrieving ID..."
    $response = Invoke-Api -Method GET -Endpoint "/resource-servers/$SYSTEM_RS_ID/resources"

    if ($response.StatusCode -eq 200) {
        $body = $response.Body | ConvertFrom-Json
        $systemResource = $body.resources | Where-Object { $_.handle -eq "system" } | Select-Object -First 1

        if ($systemResource) {
            $SYSTEM_RESOURCE_ID = $systemResource.id
            Log-Success "Found system resource ID: $SYSTEM_RESOURCE_ID"
        }
        else {
            Log-Error "Could not find system resource in response"
            System-Permissions-Failed
        }
    }
    else {
        Log-Error "Failed to fetch resources (HTTP $($response.StatusCode))"
        System-Permissions-Failed
    }
}
else {
    Log-Error "Failed to create system resource (HTTP $($response.StatusCode))"
    Write-Host "Response: $($response.Body)"
    System-Permissions-Failed
}

Log-Info "Creating 'ou' sub-resource under the 'system' resource..."

if (-not $SYSTEM_RESOURCE_ID) {
    Log-Error "System resource ID is not available. Cannot create OU resource."
    System-Permissions-Failed
}

$ouResourceData = @{
    name        = "Organization Unit"
    description = "Organization unit resource"
    handle      = "ou"
    parent      = $SYSTEM_RESOURCE_ID
} | ConvertTo-Json -Depth 10

$response = Invoke-Api -Method POST -Endpoint "/resource-servers/$SYSTEM_RS_ID/resources" -Data $ouResourceData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "OU resource created successfully (permission: system:ou)"
    $body = $response.Body | ConvertFrom-Json
    $OU_RESOURCE_ID = $body.id
    if ($OU_RESOURCE_ID) {
        Log-Info "OU resource ID: $OU_RESOURCE_ID"
    }
    else {
        Log-Error "Could not extract OU resource ID from response"
        System-Permissions-Failed
    }
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "OU resource already exists, retrieving ID..."
    $response = Invoke-Api -Method GET -Endpoint "/resource-servers/$SYSTEM_RS_ID/resources?parentId=$SYSTEM_RESOURCE_ID"

    if ($response.StatusCode -eq 200) {
        $body = $response.Body | ConvertFrom-Json
        $ouResource = $body.resources | Where-Object { $_.handle -eq "ou" } | Select-Object -First 1

        if ($ouResource) {
            $OU_RESOURCE_ID = $ouResource.id
            Log-Success "Found OU resource ID: $OU_RESOURCE_ID"
        }
        else {
            Log-Error "Could not find OU resource in response"
            System-Permissions-Failed
        }
    }
    else {
        Log-Error "Failed to fetch resources (HTTP $($response.StatusCode))"
        System-Permissions-Failed
    }
}
else {
    Log-Error "Failed to create OU resource (HTTP $($response.StatusCode))"
    Log-Error "Response: $($response.Body)"
    System-Permissions-Failed
}

Log-Info "Creating 'view' action under the 'ou' resource..."

$ouViewActionData = @{
    name        = "View"
    description = "Read-only access to organization units"
    handle      = "view"
} | ConvertTo-Json -Depth 10

$response = Invoke-Api -Method POST -Endpoint "/resource-servers/$SYSTEM_RS_ID/resources/$OU_RESOURCE_ID/actions" -Data $ouViewActionData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "OU view action created successfully (permission: system:ou:view)"
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "OU view action already exists, skipping"
}
else {
    Log-Error "Failed to create OU view action (HTTP $($response.StatusCode))"
    Log-Error "Response: $($response.Body)"
    System-Permissions-Failed
}

Log-Info "Creating 'user' sub-resource under the 'system' resource..."

if (-not $SYSTEM_RESOURCE_ID) {
    Log-Error "System resource ID is not available. Cannot create user resource."
    System-Permissions-Failed
}

$userResourceData = @{
    name        = "User"
    description = "User resource"
    handle      = "user"
    parent      = $SYSTEM_RESOURCE_ID
} | ConvertTo-Json -Depth 10

$response = Invoke-Api -Method POST -Endpoint "/resource-servers/$SYSTEM_RS_ID/resources" -Data $userResourceData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "User resource created successfully (permission: system:user)"
    $body = $response.Body | ConvertFrom-Json
    $USER_RESOURCE_ID = $body.id
    if ($USER_RESOURCE_ID) {
        Log-Info "User resource ID: $USER_RESOURCE_ID"
    }
    else {
        Log-Error "Could not extract user resource ID from response"
        System-Permissions-Failed
    }
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "User resource already exists, retrieving ID..."
    $response = Invoke-Api -Method GET -Endpoint "/resource-servers/$SYSTEM_RS_ID/resources?parentId=$SYSTEM_RESOURCE_ID"

    if ($response.StatusCode -eq 200) {
        $body = $response.Body | ConvertFrom-Json
        $userResource = $body.resources | Where-Object { $_.handle -eq "user" } | Select-Object -First 1

        if ($userResource) {
            $USER_RESOURCE_ID = $userResource.id
            Log-Success "Found user resource ID: $USER_RESOURCE_ID"
        }
        else {
            Log-Error "Could not find user resource in response"
            System-Permissions-Failed
        }
    }
    else {
        Log-Error "Failed to fetch resources (HTTP $($response.StatusCode))"
        System-Permissions-Failed
    }
}
else {
    Log-Error "Failed to create user resource (HTTP $($response.StatusCode))"
    Log-Error "Response: $($response.Body)"
    System-Permissions-Failed
}

Log-Info "Creating 'view' action under the 'user' resource..."

$userViewActionData = @{
    name        = "View"
    description = "Read-only access to users"
    handle      = "view"
} | ConvertTo-Json -Depth 10

$response = Invoke-Api -Method POST -Endpoint "/resource-servers/$SYSTEM_RS_ID/resources/$USER_RESOURCE_ID/actions" -Data $userViewActionData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "User view action created successfully (permission: system:user:view)"
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "User view action already exists, skipping"
}
else {
    Log-Error "Failed to create user view action (HTTP $($response.StatusCode))"
    Log-Error "Response: $($response.Body)"
    System-Permissions-Failed
}

Log-Info "Creating 'usertype' sub-resource under the 'system' resource..."

if (-not $SYSTEM_RESOURCE_ID) {
    Log-Error "System resource ID is not available. Cannot create user type resource."
    System-Permissions-Failed
}

$userTypeResourceData = @{
    name        = "User Type"
    description = "User type resource"
    handle      = "usertype"
    parent      = $SYSTEM_RESOURCE_ID
} | ConvertTo-Json -Depth 10

$response = Invoke-Api -Method POST -Endpoint "/resource-servers/$SYSTEM_RS_ID/resources" -Data $userTypeResourceData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "User type resource created successfully (permission: system:usertype)"
    $body = $response.Body | ConvertFrom-Json
    $USER_TYPE_RESOURCE_ID = $body.id
    if ($USER_TYPE_RESOURCE_ID) {
        Log-Info "User type resource ID: $USER_TYPE_RESOURCE_ID"
    }
    else {
        Log-Error "Could not extract user type resource ID from response"
        System-Permissions-Failed
    }
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "User type resource already exists, retrieving ID..."
    $response = Invoke-Api -Method GET -Endpoint "/resource-servers/$SYSTEM_RS_ID/resources?parentId=$SYSTEM_RESOURCE_ID"

    if ($response.StatusCode -eq 200) {
        $body = $response.Body | ConvertFrom-Json
        $userTypeResource = $body.resources | Where-Object { $_.handle -eq "usertype" } | Select-Object -First 1

        if ($userTypeResource) {
            $USER_TYPE_RESOURCE_ID = $userTypeResource.id
            Log-Success "Found user type resource ID: $USER_TYPE_RESOURCE_ID"
        }
        else {
            Log-Error "Could not find user type resource in response"
            System-Permissions-Failed
        }
    }
    else {
        Log-Error "Failed to fetch resources (HTTP $($response.StatusCode))"
        System-Permissions-Failed
    }
}
else {
    Log-Error "Failed to create user type resource (HTTP $($response.StatusCode))"
    Log-Error "Response: $($response.Body)"
    System-Permissions-Failed
}

Log-Info "Creating 'view' action under the 'usertype' resource..."

$userTypeViewActionData = @{
    name        = "View"
    description = "Read-only access to user types"
    handle      = "view"
} | ConvertTo-Json -Depth 10

$response = Invoke-Api -Method POST -Endpoint "/resource-servers/$SYSTEM_RS_ID/resources/$USER_TYPE_RESOURCE_ID/actions" -Data $userTypeViewActionData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "User type view action created successfully (permission: system:usertype:view)"
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "User type view action already exists, skipping"
}
else {
    Log-Error "Failed to create user type view action (HTTP $($response.StatusCode))"
    Log-Error "Response: $($response.Body)"
    System-Permissions-Failed
}

Write-Host ""

Log-Info "Creating 'group' sub-resource under the 'system' resource..."

if (-not $SYSTEM_RESOURCE_ID) {
    Log-Error "System resource ID is not available. Cannot create group resource."
    System-Permissions-Failed
}

$groupResourceData = @{
    name        = "Group"
    description = "Group resource"
    handle      = "group"
    parent      = $SYSTEM_RESOURCE_ID
} | ConvertTo-Json -Depth 10

$response = Invoke-Api -Method POST -Endpoint "/resource-servers/$SYSTEM_RS_ID/resources" -Data $groupResourceData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "Group resource created successfully (permission: system:group)"
    $body = $response.Body | ConvertFrom-Json
    $GROUP_RESOURCE_ID = $body.id
    if ($GROUP_RESOURCE_ID) {
        Log-Info "Group resource ID: $GROUP_RESOURCE_ID"
    }
    else {
        Log-Error "Could not extract group resource ID from response"
        System-Permissions-Failed
    }
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "Group resource already exists, retrieving ID..."
    $response = Invoke-Api -Method GET -Endpoint "/resource-servers/$SYSTEM_RS_ID/resources?parentId=$SYSTEM_RESOURCE_ID"

    if ($response.StatusCode -eq 200) {
        $body = $response.Body | ConvertFrom-Json
        $groupResource = $body.resources | Where-Object { $_.handle -eq "group" } | Select-Object -First 1

        if ($groupResource) {
            $GROUP_RESOURCE_ID = $groupResource.id
            Log-Success "Found group resource ID: $GROUP_RESOURCE_ID"
        }
        else {
            Log-Error "Could not find group resource in response"
            System-Permissions-Failed
        }
    }
    else {
        Log-Error "Failed to fetch resources (HTTP $($response.StatusCode))"
        System-Permissions-Failed
    }
}
else {
    Log-Error "Failed to create group resource (HTTP $($response.StatusCode))"
    Log-Error "Response: $($response.Body)"
    System-Permissions-Failed
}

Log-Info "Creating 'view' action under the 'group' resource..."

$groupViewActionData = @{
    name        = "View"
    description = "Read-only access to groups"
    handle      = "view"
} | ConvertTo-Json -Depth 10

$response = Invoke-Api -Method POST -Endpoint "/resource-servers/$SYSTEM_RS_ID/resources/$GROUP_RESOURCE_ID/actions" -Data $groupViewActionData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "Group view action created successfully (permission: system:group:view)"
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "Group view action already exists, skipping"
}
else {
    Log-Error "Failed to create group view action (HTTP $($response.StatusCode))"
    Log-Error "Response: $($response.Body)"
    System-Permissions-Failed
}

Log-Result-Success "Created system permissions"

Write-Host ""

# ============================================================================
# Create Administrator Group
# ============================================================================

Log-Info "Creating administrator group..."

if (-not $DEFAULT_OU_ID) {
    Log-Error "Default OU ID is not available. Cannot create administrator group."
    Log-Result-Failure "Failed to create Administrators group"
    exit 1
}

if (-not $ADMIN_USER_ID) {
    Log-Error "Admin user ID is not available. Cannot create administrator group with user membership."
    Log-Result-Failure "Failed to create Administrators group"
    exit 1
}

$administratorGroupData = @{
    name = "Administrators"
    description = "System Administrators group"
    ouId = $DEFAULT_OU_ID
    members = @(
        @{
            id = $ADMIN_USER_ID
            type = "user"
        }
    )
} | ConvertTo-Json -Depth 10

$response = Invoke-Api -Method POST -Endpoint "/groups" -Data $administratorGroupData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "Administrator group created successfully"
    $body = $response.Body | ConvertFrom-Json
    $ADMIN_GROUP_ID = $body.id
    if ($ADMIN_GROUP_ID) {
        Log-Info "Administrator group ID: $ADMIN_GROUP_ID"
        Log-Result-Success "Created Administrators group"
    }
    else {
        Log-Error "Could not extract administrator group ID from response"
        Log-Result-Failure "Failed to create Administrators group"
        exit 1
    }
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "Administrator group already exists, retrieving ID..."
    $response = Invoke-Api -Method GET -Endpoint "/groups/tree/default?limit=100"

    if ($response.StatusCode -eq 200) {
        $body = $response.Body | ConvertFrom-Json
        $adminGroup = $body.groups | Where-Object { $_.name -eq "Administrators" } | Select-Object -First 1

        if ($adminGroup) {
            $ADMIN_GROUP_ID = $adminGroup.id
            Log-Success "Found administrator group ID: $ADMIN_GROUP_ID"
            Log-Result-Success "Created Administrators group"
        }
        else {
            Log-Error "Could not find administrator group in response"
            Log-Result-Failure "Failed to create Administrators group"
            exit 1
        }
    }
    else {
        Log-Error "Failed to fetch groups under default OU (HTTP $($response.StatusCode))"
        Log-Result-Failure "Failed to create Administrators group"
        exit 1
    }
}
else {
    Log-Error "Failed to create administrator group (HTTP $($response.StatusCode))"
    Write-Host "Response: $($response.Body)"
    Log-Result-Failure "Failed to create Administrators group"
    exit 1
}

Write-Host ""

# ============================================================================
# Create Admin Role
# ============================================================================

Log-Info "Creating admin role with 'system' permission..."

if (-not $ADMIN_GROUP_ID) {
    Log-Error "Administrator group ID is not available. Cannot create role."
    Log-Result-Failure "Failed to create Administrator role"
    exit 1
}

if (-not $DEFAULT_OU_ID) {
    Log-Error "Default OU ID is not available. Cannot create role."
    Log-Result-Failure "Failed to create Administrator role"
    exit 1
}

if (-not $SYSTEM_RS_ID) {
    Log-Error "System resource server ID is not available. Cannot create role."
    Log-Result-Failure "Failed to create Administrator role"
    exit 1
}

$roleData = @{
    name = "Administrator"
    description = "System administrator role with full permissions"
    ouId = $DEFAULT_OU_ID
    permissions = @(
        @{
            resourceServerId = $SYSTEM_RS_ID
            permissions = @("system")
        }
    )
    assignments = @(
        @{
            id = $ADMIN_GROUP_ID
            type = "group"
        }
    )
} | ConvertTo-Json -Depth 10

$response = Invoke-Api -Method POST -Endpoint "/roles" -Data $roleData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "Admin role created and assigned to administrator group"
    $body = $response.Body | ConvertFrom-Json
    $ADMIN_ROLE_ID = $body.id
    if ($ADMIN_ROLE_ID) {
        Log-Info "Admin role ID: $ADMIN_ROLE_ID"
    }
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "Admin role already exists"
}
else {
    Log-Error "Failed to create admin role (HTTP $($response.StatusCode))"
    Write-Host "Response: $($response.Body)"
    Log-Result-Failure "Failed to create Administrator role"
    exit 1
}

Log-Result-Success "Created Administrator role"

Write-Host ""


Log-Info "Creating default flows..."

# Path to flow definitions directories
$AUTH_FLOWS_DIR = Join-Path $PSScriptRoot "flows" "authentication"
$REG_FLOWS_DIR = Join-Path $PSScriptRoot "flows" "registration"
$USER_ONBOARDING_FLOWS_DIR = Join-Path $PSScriptRoot "flows" "user_onboarding"
$RECOVERY_FLOWS_DIR = Join-Path $PSScriptRoot "flows" "recovery"

# Check if flows directories exist
if (-not (Test-Path $AUTH_FLOWS_DIR) -and -not (Test-Path $REG_FLOWS_DIR) -and -not (Test-Path $USER_ONBOARDING_FLOWS_DIR) -and -not (Test-Path $RECOVERY_FLOWS_DIR)) {
    Log-Warning "Flow definitions directories not found, skipping flow creation"
}
else {
    $flowCount = 0
    $flowSuccess = 0
    $flowSkipped = 0

    # Process authentication flows
    if (Test-Path $AUTH_FLOWS_DIR) {
        $authFlowFiles = Get-ChildItem -Path $AUTH_FLOWS_DIR -Filter "*.json" -File -ErrorAction SilentlyContinue
        
        if ($authFlowFiles.Count -gt 0) {
            Log-Info "Processing authentication flows..."
            
            # Fetch existing auth flows
            $listResponse = Invoke-Api -Method GET -Endpoint "/flows?flowType=AUTHENTICATION&limit=200"
            
            # Store existing auth flows by handle in a hashtable
            $existingAuthFlows = @{}
            if ($listResponse.StatusCode -eq 200) {
                $listBody = $listResponse.Body | ConvertFrom-Json
                foreach ($flow in $listBody.flows) {
                    $existingAuthFlows[$flow.handle] = $flow.id
                }
            }
            
            foreach ($flowFile in $authFlowFiles) {
                $flowCount++
                
                # Get flow handle and name from file
                $flowContent = Get-Content -Path $flowFile.FullName -Raw | ConvertFrom-Json
                $flowHandle = $flowContent.handle
                $flowName = $flowContent.name
                
                # Check if flow exists by handle
                if ($existingAuthFlows.ContainsKey($flowHandle)) {
                    # Update existing flow
                    $flowId = $existingAuthFlows[$flowHandle]
                    Log-Info "Updating existing auth flow: $flowName (handle: $flowHandle)"
                    $result = Update-Flow -FlowId $flowId -FlowFilePath $flowFile.FullName
                    if ($result) {
                        $flowSuccess++
                    }
                }
                else {
                    # Create new flow
                    $flowId = Create-Flow -FlowFilePath $flowFile.FullName
                    if ($flowId) {
                        $flowSuccess++
                    }
                    elseif ($flowId -eq "") {
                        $flowSkipped++
                    }
                }
            }
        }
        else {
            Log-Info "No authentication flow files found"
        }
    }

    # Process registration flows
    if (Test-Path $REG_FLOWS_DIR) {
        $regFlowFiles = Get-ChildItem -Path $REG_FLOWS_DIR -Filter "*.json" -File -ErrorAction SilentlyContinue
        
        if ($regFlowFiles.Count -gt 0) {
            Log-Info "Processing registration flows..."
            
            # Fetch existing registration flows
            $listResponse = Invoke-Api -Method GET -Endpoint "/flows?flowType=REGISTRATION&limit=200"
            
            # Store existing registration flows by handle in a hashtable
            $existingRegFlows = @{}
            if ($listResponse.StatusCode -eq 200) {
                $listBody = $listResponse.Body | ConvertFrom-Json
                foreach ($flow in $listBody.flows) {
                    $existingRegFlows[$flow.handle] = $flow.id
                }
            }

            foreach ($flowFile in $regFlowFiles) {
                $flowCount++
                
                # Get flow handle and name from file
                $flowContent = Get-Content -Path $flowFile.FullName -Raw | ConvertFrom-Json
                $flowHandle = $flowContent.handle
                $flowName = $flowContent.name
                
                # Check if flow exists by handle
                if ($existingRegFlows.ContainsKey($flowHandle)) {
                    # Update existing flow
                    $flowId = $existingRegFlows[$flowHandle]
                    Log-Info "Updating existing registration flow: $flowName (handle: $flowHandle)"
                    $result = Update-Flow -FlowId $flowId -FlowFilePath $flowFile.FullName
                    if ($result) {
                        $flowSuccess++
                    }
                }
                else {
                    # Create new flow
                    $flowId = Create-Flow -FlowFilePath $flowFile.FullName
                    if ($flowId) {
                        $flowSuccess++
                    }
                    elseif ($flowId -eq "") {
                        $flowSkipped++
                    }
                }
            }
        }
        else {
            Log-Info "No registration flow files found"
        }
    }

    # Process user onboarding flows
    if (Test-Path $USER_ONBOARDING_FLOWS_DIR) {
        $onboardingFlowFiles = Get-ChildItem -Path $USER_ONBOARDING_FLOWS_DIR -Filter "*.json" -File -ErrorAction SilentlyContinue
        
        if ($onboardingFlowFiles.Count -gt 0) {
            Log-Info "Processing user onboarding flows..."
            
            # Fetch existing user onboarding flows
            $listResponse = Invoke-Api -Method GET -Endpoint "/flows?flowType=USER_ONBOARDING&limit=200"
            
            # Store existing onboarding flows by handle in a hashtable
            $existingOnboardingFlows = @{}
            if ($listResponse.StatusCode -eq 200) {
                $listBody = $listResponse.Body | ConvertFrom-Json
                foreach ($flow in $listBody.flows) {
                    $existingOnboardingFlows[$flow.handle] = $flow.id
                }
            }
            
            foreach ($flowFile in $onboardingFlowFiles) {
                $flowCount++
                
                # Get flow handle and name from file
                $flowContent = Get-Content -Path $flowFile.FullName -Raw | ConvertFrom-Json
                $flowHandle = $flowContent.handle
                $flowName = $flowContent.name
                
                # Check if flow exists by handle
                if ($existingOnboardingFlows.ContainsKey($flowHandle)) {
                    # Update existing flow
                    $flowId = $existingOnboardingFlows[$flowHandle]
                    Log-Info "Updating existing user onboarding flow: $flowName (handle: $flowHandle)"
                    $result = Update-Flow -FlowId $flowId -FlowFilePath $flowFile.FullName
                    if ($result) {
                        $flowSuccess++
                    }
                }
                else {
                    # Create new flow
                    $flowId = Create-Flow -FlowFilePath $flowFile.FullName
                    if ($flowId) {
                        $flowSuccess++
                    }
                    elseif ($flowId -eq "") {
                        $flowSkipped++
                    }
                }
            }
        }
        else {
            Log-Info "No user onboarding flow files found"
        }
    }

    # Process recovery flows
    if (Test-Path $RECOVERY_FLOWS_DIR) {
        $recoveryFlowFiles = Get-ChildItem -Path $RECOVERY_FLOWS_DIR -Filter "*.json" -File -ErrorAction SilentlyContinue

        if ($recoveryFlowFiles.Count -gt 0) {
            Log-Info "Processing recovery flows..."

            # Fetch existing recovery flows
            $listResponse = Invoke-Api -Method GET -Endpoint "/flows?flowType=RECOVERY&limit=200"

            # Store existing recovery flows by handle in a hashtable
            $existingRecoveryFlows = @{}
            if ($listResponse.StatusCode -eq 200) {
                $listBody = $listResponse.Body | ConvertFrom-Json
                foreach ($flow in $listBody.flows) {
                    $existingRecoveryFlows[$flow.handle] = $flow.id
                }
            }

            foreach ($flowFile in $recoveryFlowFiles) {
                $flowCount++

                # Get flow handle and name from file
                $flowContent = Get-Content -Path $flowFile.FullName -Raw | ConvertFrom-Json
                $flowHandle = $flowContent.handle
                $flowName = $flowContent.name

                # Check if flow exists by handle
                if ($existingRecoveryFlows.ContainsKey($flowHandle)) {
                    # Update existing flow
                    $flowId = $existingRecoveryFlows[$flowHandle]
                    Log-Info "Updating existing recovery flow: $flowName (handle: $flowHandle)"
                    $result = Update-Flow -FlowId $flowId -FlowFilePath $flowFile.FullName
                    if ($result) {
                        $flowSuccess++
                    }
                }
                else {
                    # Create new flow
                    $flowId = Create-Flow -FlowFilePath $flowFile.FullName
                    if ($flowId) {
                        $flowSuccess++
                    }
                    elseif ($flowId -eq "") {
                        $flowSkipped++
                    }
                }
            }
        }
        else {
            Log-Info "No recovery flow files found"
        }
    }

    if ($flowCount -gt 0) {
        Log-Info "Flow creation summary: $flowSuccess created/updated, $flowSkipped skipped, $($flowCount - $flowSuccess - $flowSkipped) failed"
    }
}

Write-Host ""

# ============================================================================
# Create Application-Specific Flows
# ============================================================================

Log-Info "Creating application-specific flows..."

$APPS_FLOWS_DIR = Join-Path $PSScriptRoot "flows" "apps"

# Store application flow IDs in a hashtable
$APP_FLOW_IDS = @{}

if (Test-Path $APPS_FLOWS_DIR) {
    # Fetch all existing flows once
    Log-Info "Fetching existing flows for application flow processing..."
    
    # Get auth flows
    $authResponse = Invoke-Api -Method GET -Endpoint "/flows?flowType=AUTHENTICATION&limit=200"
    $existingAppAuthFlows = @{}
    if ($authResponse.StatusCode -eq 200) {
        $authBody = $authResponse.Body | ConvertFrom-Json
        foreach ($flow in $authBody.flows) {
            $existingAppAuthFlows[$flow.handle] = $flow.id
        }
    }
    
    # Get registration flows
    $regResponse = Invoke-Api -Method GET -Endpoint "/flows?flowType=REGISTRATION&limit=200"
    $existingAppRegFlows = @{}
    if ($regResponse.StatusCode -eq 200) {
        $regBody = $regResponse.Body | ConvertFrom-Json
        foreach ($flow in $regBody.flows) {
            $existingAppRegFlows[$flow.handle] = $flow.id
        }
    }

    # Get recovery flows
    $recoveryResponse = Invoke-Api -Method GET -Endpoint "/flows?flowType=RECOVERY&limit=200"
    $existingAppRecoveryFlows = @{}
    if ($recoveryResponse.StatusCode -eq 200) {
        $recoveryBody = $recoveryResponse.Body | ConvertFrom-Json
        foreach ($flow in $recoveryBody.flows) {
            $existingAppRecoveryFlows[$flow.handle] = $flow.id
        }
    }

    $appDirs = Get-ChildItem -Path $APPS_FLOWS_DIR -Directory -ErrorAction SilentlyContinue

    foreach ($appDir in $appDirs) {
        $appName = $appDir.Name
        $appAuthFlowId = ""
        $appRegFlowId = ""
        $appRecoveryFlowId = ""

        Log-Info "Processing flows for application: $appName"

        # Process authentication flow for app
        $authFlowFiles = Get-ChildItem -Path $appDir.FullName -Filter "auth_*.json" -File -ErrorAction SilentlyContinue

        if ($authFlowFiles.Count -gt 0) {
            $authFlowFile = $authFlowFiles[0]
            $flowContent = Get-Content -Path $authFlowFile.FullName -Raw | ConvertFrom-Json
            $flowHandle = $flowContent.handle
            $flowName = $flowContent.name

            # Check if auth flow exists by handle
            if ($existingAppAuthFlows.ContainsKey($flowHandle)) {
                # Update existing flow
                $appAuthFlowId = $existingAppAuthFlows[$flowHandle]
                Log-Info "Updating existing auth flow: $flowName (handle: $flowHandle)"
                Update-Flow -FlowId $appAuthFlowId -FlowFilePath $authFlowFile.FullName
            }
            else {
                # Create new flow
                $appAuthFlowId = Create-Flow -FlowFilePath $authFlowFile.FullName
            }

            # Re-fetch registration flows after creating auth flow
            if ($appAuthFlowId) {
                $response = Invoke-Api -Method GET -Endpoint "/flows?flowType=REGISTRATION&limit=200"
                if ($response.StatusCode -eq 200) {
                    $existingAppRegFlows = @{}
                    $flows = ($response.Body | ConvertFrom-Json).flows
                    foreach ($flow in $flows) {
                        $existingAppRegFlows[$flow.handle] = $flow.id
                    }
                }
            }
        }
        else {
            Log-Warning "No authentication flow file found for app: $appName"
        }

        # Process registration flow for app
        $regFlowFiles = Get-ChildItem -Path $appDir.FullName -Filter "registration_*.json" -File -ErrorAction SilentlyContinue

        if ($regFlowFiles.Count -gt 0) {
            $regFlowFile = $regFlowFiles[0]
            $flowContent = Get-Content -Path $regFlowFile.FullName -Raw | ConvertFrom-Json
            $flowHandle = $flowContent.handle
            $flowName = $flowContent.name

            # Check if registration flow exists by handle
            if ($existingAppRegFlows.ContainsKey($flowHandle)) {
                # Update existing flow
                $appRegFlowId = $existingAppRegFlows[$flowHandle]
                Log-Info "Updating existing registration flow: $flowName (handle: $flowHandle)"
                Update-Flow -FlowId $appRegFlowId -FlowFilePath $regFlowFile.FullName
            }
            else {
                # Create new flow
                $appRegFlowId = Create-Flow -FlowFilePath $regFlowFile.FullName
            }
        }
        else {
            Log-Warning "No registration flow file found for app: $appName"
        }

        # Process recovery flow for app
        $recoveryFlowFiles = Get-ChildItem -Path $appDir.FullName -Filter "recovery_*.json" -File -ErrorAction SilentlyContinue

        if ($recoveryFlowFiles.Count -gt 0) {
            $recoveryFlowFile = $recoveryFlowFiles[0]
            $flowContent = Get-Content -Path $recoveryFlowFile.FullName -Raw | ConvertFrom-Json
            $flowHandle = $flowContent.handle
            $flowName = $flowContent.name

            # Check if recovery flow exists by handle
            if ($existingAppRecoveryFlows.ContainsKey($flowHandle)) {
                # Update existing flow
                $appRecoveryFlowId = $existingAppRecoveryFlows[$flowHandle]
                Log-Info "Updating existing recovery flow: $flowName (handle: $flowHandle)"
                Update-Flow -FlowId $appRecoveryFlowId -FlowFilePath $recoveryFlowFile.FullName
            }
            else {
                # Create new flow
                $appRecoveryFlowId = Create-Flow -FlowFilePath $recoveryFlowFile.FullName
            }
        }
        else {
            Log-Debug "No recovery flow file found for app: $appName"
        }

        # Store the flow IDs for this app
        $APP_FLOW_IDS[$appName] = @{
            authFlowId     = $appAuthFlowId
            regFlowId      = $appRegFlowId
            recoveryFlowId = $appRecoveryFlowId
        }
    }
}
else {
    Log-Warning "Application flows directory not found at $APPS_FLOWS_DIR"
}

if ($flowCount -gt 0) {
    $flowFailures = $flowCount - $flowSuccess - $flowSkipped
    if ($flowFailures -eq 0) {
        Log-Result-Success "Created default flows"
    }
    else {
        Log-Result-Failure "Failed to create default flows (flowCount=$flowCount, flowSuccess=$flowSuccess, flowSkipped=$flowSkipped, failures=$flowFailures)"
        exit 1
    }
}
else {
    Log-Result-Success "Created default flows"
}

Write-Host ""

# ============================================================================
# Create Console Application
# ============================================================================

Log-Info "Creating Console application..."

# Get flow IDs for console app from the APP_FLOW_IDS created/found during flow processing
$CONSOLE_AUTH_FLOW_ID = ""
$CONSOLE_REG_FLOW_ID = ""
$CONSOLE_RECOVERY_FLOW_ID = ""

if ($APP_FLOW_IDS.ContainsKey("console")) {
    $CONSOLE_AUTH_FLOW_ID = $APP_FLOW_IDS["console"].authFlowId
    $CONSOLE_REG_FLOW_ID = $APP_FLOW_IDS["console"].regFlowId
    $CONSOLE_RECOVERY_FLOW_ID = $APP_FLOW_IDS["console"].recoveryFlowId
}

# Validate that flow IDs are available
if (-not $CONSOLE_AUTH_FLOW_ID) {
    Log-Error "Console authentication flow ID not found, cannot create Console application"
    Log-Error "Make sure flows/apps/console/auth_flow_console.json exists"
    Log-Result-Failure "Failed to create Console application"
    exit 1
}
if (-not $CONSOLE_REG_FLOW_ID) {
    Log-Error "Console registration flow ID not found, cannot create Console application"
    Log-Error "Make sure flows/apps/console/registration_flow_console.json exists"
    Log-Result-Failure "Failed to create Console application"
    exit 1
}
if (-not $CONSOLE_RECOVERY_FLOW_ID) {
    Log-Warning "Console recovery flow ID not found, recovery flow will be disabled"
}

# Use PUBLIC_URL for redirect URIs, fallback to API_BASE if not set
$PUBLIC_URL = if ($env:PUBLIC_URL) { $env:PUBLIC_URL } else { $env:API_BASE }

# Build redirect URIs array - default + custom if provided
$redirectUrisList = @("$PUBLIC_URL/console")
if ($ConsoleRedirectUris) {
    Log-Info "Adding custom redirect URIs: $ConsoleRedirectUris"
    # Split comma-separated URIs and append to array
    $customUris = $ConsoleRedirectUris -split ',' | ForEach-Object { $_.Trim() }
    $redirectUrisList += $customUris
}

$appData = @{
    name = "Console"
    description = "Management application for $PRODUCT_NAME"
    ouId = $DEFAULT_OU_ID
    url = "$PUBLIC_URL/console"
    logoUrl = "emoji:👨‍💻"
    authFlowId = $CONSOLE_AUTH_FLOW_ID
    registrationFlowId = $CONSOLE_REG_FLOW_ID
    isRegistrationFlowEnabled = $false
    allowedUserTypes = @("Person")
    user_attributes = @("given_name", "family_name", "email", "groups", "name", "ouId")
    inboundAuthConfig = @(
        @{
            type = "oauth2"
            config = @{
                clientId = "CONSOLE"
                redirectUris = $redirectUrisList
                grantTypes = @("authorization_code", "refresh_token")
                responseTypes = @("code")
                pkceRequired = $true
                tokenEndpointAuthMethod = "none"
                publicClient = $true
                token = @{
                    accessToken = @{
                        validityPeriod = 3600
                        userAttributes = @("given_name", "family_name", "email", "groups", "name", "ouId")
                    }
                    idToken = @{
                        validityPeriod = 3600
                        userAttributes = @("given_name", "family_name", "email", "groups", "name", "ouId")
                    }
                }
                scopeClaims = @{
                    profile = @("name", "given_name", "family_name", "picture")
                    email = @("email", "email_verified")
                    phone = @("phone_number", "phone_number_verified")
                    group = @("groups")
                    ou = @("ouId")
                }
            }
        }
    )
}

# Add recovery flow fields only if recovery flow ID is provided
if ($CONSOLE_RECOVERY_FLOW_ID) {
    $appData["recoveryFlowId"] = $CONSOLE_RECOVERY_FLOW_ID
    $appData["isRecoveryFlowEnabled"] = $false
}

$appData = $appData | ConvertTo-Json -Depth 10

$response = Invoke-Api -Method POST -Endpoint "/applications" -Data $appData

if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
    Log-Success "Console application created successfully"
}
elseif ($response.StatusCode -eq 409) {
    Log-Warning "Console application already exists, skipping"
}
elseif ($response.StatusCode -eq 400 -and ($response.Body -match "Application already exists|APP-1022")) {
    Log-Warning "Console application already exists, skipping"
}
else {
    Log-Error "Failed to create Console application (HTTP $($response.StatusCode))"
    Write-Host "Response: $($response.Body)"
    Log-Result-Failure "Failed to create Console application"
    exit 1
}

Log-Result-Success "Created Console application"

Write-Host ""

# ============================================================================
# Create Themes
# ============================================================================

Log-Info "Creating themes..."

# Get the script directory to locate theme files
$themesDir = Join-Path $PSScriptRoot "themes"

# Check if themes directory exists
if (-not (Test-Path $themesDir)) {
    Log-Warning "Themes directory not found at $themesDir, skipping theme creation"
}
else {
    $themeFiles = Get-ChildItem -Path $themesDir -Filter "*.json" -File -ErrorAction SilentlyContinue

    if ($themeFiles.Count -gt 0) {
        Log-Info "Processing themes from $themesDir..."

        $themeCount = 0
        $themeCreated = 0
        $themeUpdated = 0

        foreach ($themeFile in $themeFiles) {
            $themeCount++

            # Get theme name from file content
            $themeContent = Get-Content -Path $themeFile.FullName -Raw | ConvertFrom-Json
            $themeName    = if ($themeContent.displayName) { $themeContent.displayName } else { $themeFile.BaseName }
            $themeHandle  = $themeContent.handle
            $themePayload = Get-Content $themeFile.FullName -Raw

            Log-Info "Creating theme: $themeName (from $($themeFile.Name))"
            $response = Invoke-Api -Method POST -Endpoint "/design/themes" -Data $themePayload

            if ($response.StatusCode -in 200, 201) {
                Log-Success "Theme '$themeName' created successfully"
                $body = $response.Body | ConvertFrom-Json
                $themeId = $body.id
                if ($themeId) {
                    Log-Info "Theme ID: $themeId"
                }
                $themeCreated++
            }
            elseif ($response.StatusCode -eq 409 -or ($response.Body -match '"THM-1015"')) {
                Log-Warning "Theme '$themeName' already exists, updating..."
                $response = Invoke-Api -Method GET -Endpoint "/design/themes"
                if ($response.StatusCode -eq 200) {
                    $body = $response.Body | ConvertFrom-Json
                    $existingTheme = $body.themes | Where-Object { $_.handle -eq $themeHandle } | Select-Object -First 1
                    $themeId = $existingTheme.id
                }
                if (-not $themeId) {
                    Log-Error "Failed to retrieve existing theme ID for '$themeName'"
                    exit 1
                }
                Log-Info "Found existing theme ID: $themeId"
                $response = Invoke-Api -Method PUT -Endpoint "/design/themes/$themeId" -Data $themePayload
                if ($response.StatusCode -eq 200) {
                    Log-Success "Theme '$themeName' updated successfully"
                    $themeUpdated++
                }
                else {
                    Log-Error "Failed to update theme '$themeName' (HTTP $($response.StatusCode))"
                    Write-Host "Response: $($response.Body)"
                    Log-Result-Failure "Failed to create themes"
                    exit 1
                }
            }
            else {
                Log-Error "Failed to create theme '$themeName' (HTTP $($response.StatusCode))"
                Write-Host "Response: $($response.Body)"
                Log-Result-Failure "Failed to create themes"
                exit 1
            }
        }

        Write-Host ""
        Log-Info "Theme bootstrap summary: $themeCreated created, $themeUpdated updated (Total: $themeCount)"
    }
    else {
        Log-Warning "No theme files found in $themesDir"
    }
}

Log-Result-Success "Created themes"

Write-Host ""

# ============================================================================
# Seed i18n Translations
# ============================================================================

Log-Info "Seeding i18n translations..."

$i18nDir = Join-Path $PSScriptRoot "i18n"

if (-not (Test-Path $i18nDir)) {
    Log-Warning "i18n directory not found at $i18nDir, skipping translation seeding"
}
else {
    $i18nFiles = Get-ChildItem -Path $i18nDir -Filter "*.json" -File -ErrorAction SilentlyContinue

    if ($i18nFiles.Count -gt 0) {
        Log-Info "Processing i18n translations from $i18nDir..."

        $i18nCount = 0
        $i18nSuccess = 0

        foreach ($i18nFile in $i18nFiles) {
            $i18nCount++
            $language = $i18nFile.BaseName

            Log-Info "Seeding translations for language: $language (from $($i18nFile.Name))"

            $payload = Get-Content $i18nFile.FullName -Raw

            $response = Invoke-Api -Method POST -Endpoint "/i18n/languages/$language/translations" -Data $payload

            if ($response.StatusCode -eq 200) {
                $body = $response.Body | ConvertFrom-Json
                $total = $body.totalResults
                Log-Success "Translations for '$language' seeded successfully ($total translations)"
                $i18nSuccess++
            }
            else {
                Log-Error "Failed to seed translations for '$language' (HTTP $($response.StatusCode))"
                Write-Host "Response: $($response.Body)"
                Log-Result-Failure "Failed to seed i18n translations"
                exit 1
            }
        }

        Write-Host ""
        Log-Info "Translation seeding summary: $i18nSuccess seeded (Total: $i18nCount)"
    }
    else {
        Log-Warning "No i18n translation files found in $i18nDir"
    }
}

Log-Result-Success "Seeded i18n translations"

Write-Host ""

# ============================================================================
# Summary
# ============================================================================

Log-Success "Default resources setup completed successfully!"
Log-Result-Success "Created default resources"
Write-Host ""
