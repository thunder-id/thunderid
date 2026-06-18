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
$PRODUCT_NAME_LOWERCASE = $PRODUCT_NAME.ToLower()
$BINARY_NAME = $PRODUCT_NAME_LOWERCASE

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

# Product Setup Script
# Orchestrates the complete setup lifecycle:
# 1. Starts the server with security disabled
# 2. Executes bootstrap scripts (built-in + custom)
# 3. Stops the server
# 4. Exits cleanly

# Exit on any error
$ErrorActionPreference = 'Stop'

# Default settings
$DEBUG_PORT = if ($env:DEBUG_PORT) { [int]$env:DEBUG_PORT } else { 2345 }
$DEBUG_MODE = if ($env:DEBUG_MODE -eq "true") { $true } else { $false }
$VERBOSE_MODE = $false
$SILENT_MODE = $true
$BOOTSTRAP_FAIL_FAST = if ($env:BOOTSTRAP_FAIL_FAST -eq "false") { $false } else { $true }
$BOOTSTRAP_SKIP_PATTERN = if ($env:BOOTSTRAP_SKIP_PATTERN) { $env:BOOTSTRAP_SKIP_PATTERN } else { "" }
$BOOTSTRAP_ONLY_PATTERN = if ($env:BOOTSTRAP_ONLY_PATTERN) { $env:BOOTSTRAP_ONLY_PATTERN } else { "" }
$BOOTSTRAP_DIR = if ($env:BOOTSTRAP_DIR) { $env:BOOTSTRAP_DIR } else { ".\bootstrap" }
$WITH_CONSENT = if ($env:WITH_CONSENT -eq 'false') { $false } else { $true }

# ============================================================================
# Logging Functions
# ============================================================================

function Log-Info {
    param([string]$Message)
    if (-not $VERBOSE_MODE) {
        return
    }
    Write-Host "[INFO] $Message" -ForegroundColor Blue
}

function Log-Success {
    param([string]$Message)
    if (-not $VERBOSE_MODE) {
        return
    }
    Write-Host "[SUCCESS] [OK] $Message" -ForegroundColor Green
}

function Log-Warning {
    param([string]$Message)
    if (-not $VERBOSE_MODE) {
        return
    }
    Write-Host "[WARNING] ! $Message" -ForegroundColor Yellow
}

function Log-Error {
    param([string]$Message)
    Write-Host "[ERROR] X $Message" -ForegroundColor Red
}

function Log-Debug {
    param([string]$Message)
    if ($env:DEBUG -eq "true" -and $VERBOSE_MODE) {
        Write-Host "[DEBUG] $Message" -ForegroundColor Cyan
    }
}

# ============================================================================
# Help Function
# ============================================================================

function Show-Help {
    Write-Host ""
    Write-Host "$PRODUCT_NAME Setup Script"
    Write-Host ""
    Write-Host "Usage: .\setup.ps1 [options]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  --verbose                Enable detailed setup output"
    Write-Host "  --debug                  Enable debug mode with remote debugging"
    Write-Host "  --debug-port PORT        Set debug port (default: 2345)"
    Write-Host "  --without-consent        Disable the bundled consent server"
    Write-Host "  --help                   Show this help message"
    Write-Host ""
    Write-Host "Description:"
    Write-Host "  This script performs initial setup by:"
    Write-Host "  1. Starting $PRODUCT_NAME server temporarily with security disabled"
    Write-Host "  2. Running bootstrap scripts to create default resources"
    Write-Host "  3. Stopping the server cleanly"
    Write-Host ""
    Write-Host "  After setup completes, use '.\start.ps1' to start $PRODUCT_NAME normally."
    Write-Host ""
}

# ============================================================================
# Parse Command Line Arguments
# ============================================================================

$i = 0
while ($i -lt $args.Count) {
    switch ($args[$i]) {
        '--verbose' {
            $VERBOSE_MODE = $true
            $SILENT_MODE = $false
            $i++
            break
        }
        '--debug' {
            $DEBUG_MODE = $true
            $i++
            break
        }
        '--debug-port' {
            $i++
            if ($i -lt $args.Count) {
                $DEBUG_PORT = [int]$args[$i]
                $i++
            }
            else {
                Write-Host "Missing value for --debug-port" -ForegroundColor Red
                exit 1
            }
            break
        }
        '--without-consent' {
            $WITH_CONSENT = $false
            $i++
            break
        }
        '--help' {
            Show-Help
            exit 0
        }
        default {
            Write-Host "Unknown option: $($args[$i])" -ForegroundColor Red
            Write-Host "Use --help for usage information"
            exit 1
        }
    }
}

# ============================================================================
# Read Configuration from deployment.yaml
# ============================================================================

$CONFIG_FILE = ".\deployment.yaml"

function Read-Config {
    $configFile = $CONFIG_FILE

    if (-not (Test-Path $configFile)) {
        # Try alternative path (for packaged distribution)
        $configFile = ".\backend\cmd\server\deployment.yaml"
    }

    if (-not (Test-Path $configFile)) {
        Log-Warning "Configuration file not found, using defaults"
        return $false
    }

    Log-Debug "Reading configuration from: $configFile"

    # Try yq first (YAML parser)
    if (Get-Command yq -ErrorAction SilentlyContinue) {
        $script:HOSTNAME = & yq eval '.server.hostname // "localhost"' $configFile 2>$null
        $script:PORT = & yq eval '.server.port // 8090' $configFile 2>$null
        $script:HTTP_ONLY = & yq eval '.server.http_only // false' $configFile 2>$null
        $script:PUBLIC_URL = & yq eval '.server.public_url // ""' $configFile 2>$null
    }
    else {
        # Fallback: basic parsing with Select-String
        $content = Get-Content $configFile -Raw

        # Parse hostname
        if ($content -match '(?m)^\s*hostname:\s*[''"]?([^''"\s]+)[''"]?') {
            $script:HOSTNAME = $matches[1]
        }
        else {
            $script:HOSTNAME = "localhost"
        }

        # Parse port
        if ($content -match '(?m)^\s*port:\s*(\d+)') {
            $script:PORT = [int]$matches[1]
        }
        else {
            $script:PORT = 8090
        }

        # Parse http_only
        if ($content -match '(?m)http_only:\s*true') {
            $script:HTTP_ONLY = "true"
        }
        else {
            $script:HTTP_ONLY = "false"
        }

        # Parse public_url (quoted or unquoted)
        if ($content -match '(?m)^\s*public_url:\s*[''"]([^''"]+)[''"]') {
            $script:PUBLIC_URL = $matches[1]
        }
        elseif ($content -match '(?m)^\s*public_url:\s*([^\s#]+)') {
            $script:PUBLIC_URL = $matches[1]
        }
        else {
            $script:PUBLIC_URL = ""
        }

    }

    # Determine protocol
    if ($script:HTTP_ONLY -eq "true") {
        $script:PROTOCOL = "http"
    }
    else {
        $script:PROTOCOL = "https"
    }

    return $true
}

# Read configuration
Read-Config | Out-Null

# Construct base URL (internal API endpoint)
$BASE_URL = "$($script:PROTOCOL)://$($script:HOSTNAME):$($script:PORT)"
$script:API_BASE = $BASE_URL

# Construct public URL (external/redirect URLs), strip trailing slash to avoid double slashes in paths
$PUBLIC_URL = if ($script:PUBLIC_URL) { $script:PUBLIC_URL.TrimEnd('/') } else { $BASE_URL }

# Export environment variables for bootstrap scripts
$env:API_BASE = $BASE_URL
$env:PUBLIC_URL = $PUBLIC_URL

Write-Host ""
Write-Host "========================================="
Write-Host "   $PRODUCT_NAME Setup"
Write-Host "========================================="
Write-Host ""
if ($VERBOSE_MODE) {
    Write-Host "Server URL: $BASE_URL" -ForegroundColor Blue
    Write-Host "Public URL: $PUBLIC_URL" -ForegroundColor Blue
    if ($DEBUG_MODE) {
        Write-Host "Debug: Enabled (port $DEBUG_PORT)" -ForegroundColor Blue
    }
    Write-Host ""
}
Log-Debug "Platform: $($PSVersionTable.Platform)"

# ============================================================================
# Kill Existing Processes on Ports
# ============================================================================

function Stop-PortListener {
    param([int]$port)

    Write-Host "Checking for processes listening on TCP port $port..."

    try {
        $pids = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction Stop |
                Select-Object -ExpandProperty OwningProcess -Unique
    }
    catch {
        # Fallback to netstat parsing
        $pids = @()
        try {
            $netstat = & netstat -ano 2>$null | Select-String ":$port"
            foreach ($line in $netstat) {
                $parts = ($line -split '\s+') | Where-Object { $_ -ne '' }
                if ($parts.Count -ge 5) {
                    $procId = $parts[-1]
                    if ([int]::TryParse($procId, [ref]$null)) {
                        $pids += [int]$procId
                    }
                }
            }
        }
        catch { }
    }

    $pids = $pids | Where-Object { $_ -and ($_ -ne 0) } | Select-Object -Unique
    foreach ($procId in $pids) {
        try {
            Write-Host "Killing PID $procId that is listening on port $port"
            Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
        }
        catch {
            Write-Host "Unable to kill PID $procId : $_" -ForegroundColor Yellow
        }
    }
}

if ($DEBUG_MODE) {
    Stop-PortListener -port $DEBUG_PORT
}
Start-Sleep -Seconds 1

# Check for Delve if debug mode is enabled
if ($DEBUG_MODE -and -not (Get-Command dlv -ErrorAction SilentlyContinue)) {
    Write-Host "[ERROR] Debug mode requires Delve debugger" -ForegroundColor Red
    Write-Host ""
    Write-Host "[INFO] Install Delve using:" -ForegroundColor Cyan
    Write-Host "   go install github.com/go-delve/delve/cmd/dlv@latest" -ForegroundColor Cyan
    exit 1
}

# ============================================================================
# Create Default Resources (in-process bootstrap)
# ============================================================================

# Resolve the script directory (used to locate the consent server and start.ps1).
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# Start Consent Server (if enabled)
$consentProc = $null
$consentDir = Join-Path $scriptDir 'consent'
$serverStdOutLog = $null
$serverStdErrLog = $null
$consentStdOutLog = $null
$consentStdErrLog = $null
if ($WITH_CONSENT) {
    if (-not (Test-Path $consentDir)) {
        Log-Error "Consent server is enabled but consent directory not found: $consentDir"
        exit 1
    }
    if ($VERBOSE_MODE) {
        Write-Host "[INFO] Starting Consent Server..." -ForegroundColor Cyan
    }
    $consentPort = if ($env:CONSENT_SERVER_PORT) { $env:CONSENT_SERVER_PORT } else { "9090" }
    $consentBinary = @(
        (Join-Path $consentDir 'consent-server.exe'),
        (Join-Path $consentDir 'consent-server')
    ) | Where-Object { Test-Path $_ } | Select-Object -First 1
    if (-not $consentBinary) {
        Log-Error "Consent server is enabled but consent-server binary not found in: $consentDir"
        exit 1
    }
    $consentProcessArgs = @{
        FilePath = $consentBinary
        WorkingDirectory = $consentDir
        NoNewWindow = $true
        PassThru = $true
    }
    if ($SILENT_MODE) {
        $consentStdOutLog = [System.IO.Path]::GetTempFileName()
        $consentStdErrLog = [System.IO.Path]::GetTempFileName()
        $consentProcessArgs["RedirectStandardOutput"] = $consentStdOutLog
        $consentProcessArgs["RedirectStandardError"] = $consentStdErrLog
    }
    $consentProc = Start-Process @consentProcessArgs
    $consentTimeout = 30
    $consentElapsed = 0
    while ($consentElapsed -lt $consentTimeout) {
        if ($consentProc.HasExited) {
            Log-Error "Consent server process exited unexpectedly (code $($consentProc.ExitCode))"
            exit 1
        }
        try {
            $resp = Invoke-WebRequest -Uri "http://localhost:${consentPort}/health/readiness" -UseBasicParsing -ErrorAction Stop
            if ($resp.StatusCode -eq 200) {
                if ($VERBOSE_MODE) {
                    Write-Host "[INFO] Consent server is ready" -ForegroundColor Cyan
                }
                break
            }
        } catch { }
        Start-Sleep -Seconds 1
        $consentElapsed++
    }
    if ($consentElapsed -ge $consentTimeout) {
        Log-Error "Consent server failed to become ready within ${consentTimeout}s"
        exit 1
    }
}

# Create the default resources by delegating to start.ps1 -Bootstrap. The consent
# server (if enabled) was already started above, so --without-consent is passed so
# start.ps1 does not start a second one. The public URL is exported so the bootstrap
# subcommand picks it up; admin credentials are read from the ADMIN_USERNAME /
# ADMIN_PASSWORD environment variables (default admin / admin) when set.
$env:PUBLIC_URL = $PUBLIC_URL

$startScript = Join-Path $scriptDir 'start.ps1'
if (-not (Test-Path $startScript)) {
    Log-Error "start.ps1 is missing: $startScript"
    exit 1
}

try {
    if ($VERBOSE_MODE) {
        Write-Host "[WAIT] Creating default resources..." -ForegroundColor Blue
    }

    & $startScript --bootstrap --without-consent
    if ($LASTEXITCODE -ne 0) {
        Log-Error "Failed to create default resources"
        exit 1
    }
    Log-Success "Default resources created"

    # ========================================================================
    # Setup Completed
    # ========================================================================

    Write-Host ""
    Write-Host ""
    if ($SILENT_MODE) {
        Write-Host "========================================="
        Write-Host "Setup completed successfully!"
        Write-Host "========================================="
        Write-Host ""
        Write-Host "Console URL: ${PUBLIC_URL}/console"
        Write-Host ""
        Write-Host "Run .\start.ps1 to start ${PRODUCT_NAME}."
        Write-Host ""
    }
    else {
        Write-Host "========================================="
        Write-Host "[OK] Setup completed successfully!" -ForegroundColor Green
        Write-Host "========================================="
        Write-Host ""
        Write-Host "[INFO] Next steps:"
        Write-Host "   1. Start the server: .\start.ps1" -ForegroundColor Cyan
        Write-Host "   2. Access $PRODUCT_NAME at: $BASE_URL" -ForegroundColor Cyan
        Write-Host ""
    }
}
finally {
    # Stop the consent server started above and clean up its temp logs.
    if ($consentProc -and -not $consentProc.HasExited) {
        try { Stop-Process -Id $consentProc.Id -Force -ErrorAction SilentlyContinue } catch { }
    }
    foreach ($tempLog in @($consentStdOutLog, $consentStdErrLog)) {
        if ($tempLog -and (Test-Path $tempLog)) {
            Remove-Item $tempLog -Force -ErrorAction SilentlyContinue
        }
    }
}

exit 0
