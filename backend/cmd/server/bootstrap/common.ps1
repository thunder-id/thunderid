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

# Common functions for bootstrap scripts
# Dot-source this file at the beginning of each bootstrap script

$PRODUCT_NAME = "ThunderID"
$QUIET_MODE = if ($env:SETUP_SILENT_MODE -eq "true") { $true } else { $false }
$RESULT_COLOR_ENABLED = $false

# Check if FD3 (like in bash) is available - PowerShell doesn't have file descriptors,
# so we'll simulate by checking if we're in a TTY context
if ([Environment]::UserInteractive -and -not [Console]::IsInputRedirected) {
    $RESULT_COLOR_ENABLED = $true
}

# Configure TLS to use modern protocols (required for HTTPS requests on Windows)
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13
} catch {
    # Fallback to TLS 1.2 if TLS 1.3 is not available
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
}

# Logging Functions
function Log-Info {
    param([string]$Message)
    if (-not $QUIET_MODE) {
        Write-Host "[INFO] $Message" -ForegroundColor Blue
    }
}

function Log-Success {
    param([string]$Message)
    if (-not $QUIET_MODE) {
        Write-Host "[SUCCESS] ✓ $Message" -ForegroundColor Green
    }
}

function Log-Warning {
    param([string]$Message)
    if (-not $QUIET_MODE) {
        Write-Host "[WARNING] ⚠ $Message" -ForegroundColor Yellow
    }
}

function Log-Error {
    param([string]$Message)
    if (-not $QUIET_MODE) {
        Write-Host "[ERROR] ✗ $Message" -ForegroundColor Red
    }
}

function Log-Debug {
    param([string]$Message)
    if ($env:DEBUG -eq "true" -and -not $QUIET_MODE) {
        Write-Host "[DEBUG] $Message" -ForegroundColor Cyan
    }
}

function Log-Result-Success {
    param([string]$Message)
    if ($QUIET_MODE) {
        if ($RESULT_COLOR_ENABLED) {
            [Console]::WriteLine("      $($PSStyle.Foreground.Green)✓$($PSStyle.Reset) $Message")
        }
        else {
            [Console]::WriteLine("      ✓ $Message")
        }
    }
}

function Log-Result-Failure {
    param([string]$Message)
    if ($QUIET_MODE) {
        if ($RESULT_COLOR_ENABLED) {
            [Console]::WriteLine("      $($PSStyle.Foreground.Red)✗$($PSStyle.Reset) $Message")
        }
        else {
            [Console]::WriteLine("      ✗ $Message")
        }
    }
}

# API Call Helper Function
function Invoke-Api {
    param(
        [Parameter(Mandatory=$true)]
        [string]$Method,
        [Parameter(Mandatory=$true)]
        [string]$Endpoint,
        [Parameter(Mandatory=$false)]
        [string]$Data = $null
    )

    $url = "$($env:API_BASE)$Endpoint"
    
    Log-Debug "API Call: $Method $url"

    try {
        $headers = @{
            "Content-Type" = "application/json"
        }

        $params = @{
            Uri = $url
            Method = $Method
            Headers = $headers
            SkipCertificateCheck = $true
        }

        if ($Data) {
            $params["Body"] = $Data
        }

        $response = Invoke-WebRequest @params -ErrorAction Stop
        
        return @{
            StatusCode = $response.StatusCode
            Body = $response.Content
        }
    }
    catch {
        $statusCode = 500
        $body = ""
        
        # Try to extract status code and response body from the exception
        if ($_.Exception.Response) {
            $statusCode = [int]$_.Exception.Response.StatusCode
        }
        
        # PowerShell 7+ provides error details directly in ErrorDetails
        if ($_.ErrorDetails.Message) {
            $body = $_.ErrorDetails.Message
        }
        elseif ($_.Exception.Message) {
            $body = $_.Exception.Message
        }
        
        return @{
            StatusCode = $statusCode
            Body = $body
        }
    }
}

# Helper function to create a flow and return its ID
# Returns: Flow ID on success, empty string on failure
function Create-Flow {
    param(
        [Parameter(Mandatory=$true)]
        [string]$FlowFilePath
    )
    
    $flowPayload = Get-Content -Path $FlowFilePath -Raw
    $flowJson = $flowPayload | ConvertFrom-Json
    $flowDisplayName = $flowJson.name
    
    if (-not $flowDisplayName) {
        Log-Warning "Could not extract flow name from $(Split-Path $FlowFilePath -Leaf), skipping"
        return ""
    }
    
    Log-Info "Creating flow: $flowDisplayName"
    
    $response = Invoke-Api -Method POST -Endpoint "/flows" -Data $flowPayload
    
    if ($response.StatusCode -eq 201 -or $response.StatusCode -eq 200) {
        $body = $response.Body | ConvertFrom-Json
        $flowId = $body.id
        Log-Success "Flow '$flowDisplayName' created successfully (ID: $flowId)"
        return $flowId
    }
    elseif ($response.StatusCode -eq 409) {
        Log-Warning "Flow '$flowDisplayName' already exists, skipping"
        return ""
    }
    else {
        Log-Error "Failed to create flow '$flowDisplayName' (HTTP $($response.StatusCode))"
        Log-Error "Response: $($response.Body)"
        return ""
    }
}

# Helper function to update a flow
# Returns: $true on success, $false on failure
function Update-Flow {
    param(
        [Parameter(Mandatory=$true)]
        [string]$FlowId,
        [Parameter(Mandatory=$true)]
        [string]$FlowFilePath
    )
    
    $flowPayload = Get-Content -Path $FlowFilePath -Raw
    $flowJson = $flowPayload | ConvertFrom-Json
    $flowDisplayName = $flowJson.name
    
    if (-not $flowDisplayName) {
        Log-Warning "Could not extract flow name from $(Split-Path $FlowFilePath -Leaf), skipping"
        return $false
    }
    
    Log-Info "Updating existing flow: $flowDisplayName (ID: $FlowId)"
    
    $response = Invoke-Api -Method PUT -Endpoint "/flows/$FlowId" -Data $flowPayload
    
    if ($response.StatusCode -eq 200) {
        Log-Success "Flow '$flowDisplayName' updated successfully"
        return $true
    }
    else {
        Log-Error "Failed to update flow '$flowDisplayName' (HTTP $($response.StatusCode))"
        Log-Error "Response: $($response.Body)"
        return $false
    }
}
