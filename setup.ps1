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
# API Call Helper Function
# ============================================================================

function Invoke-Api {
    param(
        [string]$Method,
        [string]$Endpoint,
        [string]$Data = ""
    )

    # Get base URL from environment variable
    $baseUrl = if ($env:API_BASE) {
        $env:API_BASE
    } else {
        Log-Error "API_BASE is not set!"
        return @{
            StatusCode = 0
            Body = ""
            Error = "API_BASE not set"
        }
    }

    $url = "$baseUrl$Endpoint"

    Log-Debug "API Call: $Method $url"
    if ($Data) {
        Log-Debug "Request Body: $Data"
    }

    $responseFile = [System.IO.Path]::GetTempFileName()
    $dataFile = $null

    try {
        $curlArgs = @(
            "-X", $Method,
            "-k",  # Skip SSL verification
            "-s",  # Silent mode
            "-w", "%{http_code}",  # Write status code
            "-H", "Content-Type: application/json",
            "-o", $responseFile  # Output to file
        )

        if ($Data -and ($Method -eq "POST" -or $Method -eq "PUT" -or $Method -eq "PATCH")) {
            # Save data to temp file for curl
            $dataFile = [System.IO.Path]::GetTempFileName()            
            if ($PSVersionTable.PSVersion.Major -ge 6) {
                $Data | Out-File -FilePath $dataFile -Encoding UTF8NoBOM -NoNewline
            } else {
                [System.IO.File]::WriteAllText($dataFile, $Data, [System.Text.UTF8Encoding]::new($false))
            }
            
            $curlArgs += @("-d", "@$dataFile")
        }

        $curlArgs += $url

        Log-Debug "curl command: curl $($curlArgs -join ' ')"

        # Execute curl and capture output
        $curlOutput = & curl.exe @curlArgs 2>&1
        $curlExitCode = $LASTEXITCODE

        # The last line should be the status code
        $statusCode = $curlOutput | Select-Object -Last 1

        # Handle curl errors (nonzero exit code or status code might be empty or non-numeric)
        if ($curlExitCode -ne 0 -or -not $statusCode -or $statusCode -notmatch '^\d+$') {
            Log-Error "Failed to execute curl command or received invalid response (exit code: $curlExitCode)"
            Log-Error "curl output: $($curlOutput -join "`n")"
            return @{
                StatusCode = 0
                Body = ""
                Error = "curl execution failed (exit code: $curlExitCode): $($curlOutput -join '; ')"
            }
        }

        # Read response body (file should always exist, but check defensively)
        $body = if (Test-Path $responseFile) {
            Get-Content -Path $responseFile -Raw
        } else {
            ""
        }

        Log-Debug "Response Status: $statusCode"
        Log-Debug "Response Body: $body"

        $finalBody = if ($body) { $body } else { "" }

        return @{
            StatusCode = [int]$statusCode
            Body = $finalBody
        }
    }
    catch {
        Log-Error "API call failed: $_"
        Log-Error "Exception: $($_.Exception.Message)"

        return @{
            StatusCode = 0
            Body = ""
            Error = $_.Exception.Message
        }
    }
    finally {
        # Clean up temp files
        if (Test-Path $responseFile) {
            Remove-Item $responseFile -Force -ErrorAction SilentlyContinue
        }
        if ($dataFile -and (Test-Path $dataFile)) {
            Remove-Item $dataFile -Force -ErrorAction SilentlyContinue
        }
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
# Start the Server with Security Disabled
# ============================================================================

if ($VERBOSE_MODE) {
    Write-Host "[WARN] Starting temporary server with security disabled..." -ForegroundColor Yellow
    Write-Host ""
}

# Export environment variable to skip security
$hadSkipSecurity = Test-Path Env:SKIP_SECURITY
$previousSkipSecurity = $env:SKIP_SECURITY
$env:SKIP_SECURITY = "true"

# Resolve the server executable path
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$possible = @(
    (Join-Path $scriptDir "${BINARY_NAME}.exe"),
    (Join-Path $scriptDir $BINARY_NAME)
)
$serverExecPath = $possible | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $serverExecPath) {
    $serverExecPath = Join-Path $scriptDir $BINARY_NAME
}

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

$proc = $null
try {
    $serverProcessArgs = @{
        WorkingDirectory = $scriptDir
        NoNewWindow = $true
        PassThru = $true
    }
    if ($SILENT_MODE) {
        $serverStdOutLog = [System.IO.Path]::GetTempFileName()
        $serverStdErrLog = [System.IO.Path]::GetTempFileName()
        $serverProcessArgs["RedirectStandardOutput"] = $serverStdOutLog
        $serverProcessArgs["RedirectStandardError"] = $serverStdErrLog
    }

    if ($DEBUG_MODE) {
        $dlvArgs = @(
            'exec'
            "--listen=:$DEBUG_PORT"
            '--headless=true'
            '--api-version=2'
            '--accept-multiclient'
            '--continue'
            $serverExecPath
        )
        $serverProcessArgs["FilePath"] = "dlv"
        $serverProcessArgs["ArgumentList"] = $dlvArgs
        $proc = Start-Process @serverProcessArgs
    }
    else {
        $serverProcessArgs["FilePath"] = $serverExecPath
        $proc = Start-Process @serverProcessArgs
    }

    $SERVER_PID = $proc.Id

    # Cleanup function
    $cleanup = {
        if ($VERBOSE_MODE) {
            Write-Host ""
            Write-Host "[STOP] Stopping temporary server..." -ForegroundColor Cyan
        }
        if ($proc -and -not $proc.HasExited) {
            try {
                Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
            } catch { }
        }
        if ($consentProc -and -not $consentProc.HasExited) {
            try {
                Stop-Process -Id $consentProc.Id -Force -ErrorAction SilentlyContinue
            } catch { }
        }
    }

    # Register cleanup on exit
    Register-EngineEvent PowerShell.Exiting -Action $cleanup | Out-Null

    # ============================================================================
    # Wait for Server to be Ready
    # ============================================================================

    if ($VERBOSE_MODE) {
        Write-Host "[WAIT] Waiting for server to be ready..." -ForegroundColor Blue
        Write-Host "   Server URL: $BASE_URL" -ForegroundColor Blue
    }

    $TIMEOUT = 60
    $ELAPSED = 0
    $RETRY_DELAY = 2
    $lastError = ""

    while ($ELAPSED -lt $TIMEOUT) {
        Log-Debug "Attempting health check (attempt $([math]::Floor($ELAPSED / $RETRY_DELAY) + 1))..."

        $healthUrl = "$BASE_URL/health/readiness"
        Log-Debug "Making request to: $healthUrl"

        $requestStart = Get-Date
        $statusCode = & curl.exe -k -s -w "%{http_code}" -o NUL $healthUrl 2>&1 | Select-Object -Last 1
        $requestDuration = (Get-Date) - $requestStart

        Log-Debug "Request completed in $([math]::Round($requestDuration.TotalSeconds, 2))s with status: $statusCode"

        if ($statusCode -eq "200") {
            if ($VERBOSE_MODE) {
                Write-Host ""
                Write-Host "[OK] Server is ready" -ForegroundColor Green
                Log-Debug "Health check response: $body"
                Write-Host ""
            }
            break
        }
        else {
            # Server not ready yet
            $currentError = "HTTP $statusCode"

            # Log additional details when error status changes
            if ($currentError -ne $lastError) {
                Write-Host ""
                Log-Debug "Health check failed with status: $statusCode"

                if (-not $statusCode -or $statusCode -eq '000') {
                    Log-Debug "Connection refused - server not yet listening"
                } elseif ($statusCode -match "^50[0-9]$") {
                    Log-Debug "Server error - server might be starting"
                }

                $lastError = $currentError
                Write-Host "." -NoNewline
            } else {
                Write-Host "." -NoNewline
            }
        }

        Start-Sleep -Seconds $RETRY_DELAY
        $ELAPSED += $RETRY_DELAY
    }

    if ($ELAPSED -ge $TIMEOUT) {
        Write-Host ""
        Write-Host "[ERROR] Server health check failed within $TIMEOUT seconds" -ForegroundColor Red
        Write-Host "Expected server at: $BASE_URL" -ForegroundColor Red
        Write-Host "Last status: $lastError" -ForegroundColor Red
        exit 1
    }

    # ============================================================================
    # Run Bootstrap Scripts
    # ============================================================================

    # Export environment variable for bootstrap scripts
    $env:SETUP_SILENT_MODE = if ($SILENT_MODE) { "true" } else { "false" }

    # Check if bootstrap directory exists
    if (-not (Test-Path $BOOTSTRAP_DIR)) {
        Log-Warning "Bootstrap directory not found: $BOOTSTRAP_DIR"
        Log-Info "Skipping bootstrap execution"
    }
    else {
        Log-Info "========================================="
        Log-Info "$PRODUCT_NAME Bootstrap Process"
        Log-Info "========================================="
        Log-Info "Bootstrap directory: $BOOTSTRAP_DIR"
        Log-Info "Fail fast: $BOOTSTRAP_FAIL_FAST"
        Log-Info "Started at: $(Get-Date)"
        Write-Host ""

        # Collect all PowerShell scripts from bootstrap directory
        $scripts = @()

        # Find PowerShell scripts in bootstrap directory
        if (Test-Path $BOOTSTRAP_DIR) {
            Log-Debug "Scanning $BOOTSTRAP_DIR for PowerShell scripts..."
            $scripts = Get-ChildItem -Path $BOOTSTRAP_DIR -Filter "*.ps1" -File -ErrorAction SilentlyContinue

            Log-Debug "Found $($scripts.Count) PowerShell script(s)"
            foreach ($bootstrapScript in $scripts) {
                Log-Debug "  - $($bootstrapScript.Name)"
            }
        }

        # Sort scripts by filename (numeric prefix determines order)
        $sortedScripts = $scripts | Sort-Object Name

        if ($sortedScripts.Count -eq 0) {
            Log-Warning "No bootstrap scripts found"
        }
        else {
            Log-Info "Discovered $($sortedScripts.Count) PowerShell script(s)"
            Log-Debug "Scripts will be executed in this order:"
            foreach ($bootstrapScript in $sortedScripts) {
                Log-Debug "  - $($bootstrapScript.Name)"
            }
            Write-Host ""

            # Execute scripts
            $scriptCount = 0
            $successCount = 0
            $failedCount = 0
            $skippedCount = 0

            foreach ($bootstrapScript in $sortedScripts) {
                $scriptName = $bootstrapScript.Name

                if ($SILENT_MODE) {
                    if ($scriptName -eq "01-default-resources.ps1" -or $scriptName -eq "01-default-resources.sh") {
                        Write-Host ""
                        Write-Host "  Default resources"
                    }
                    elseif ($scriptName -eq "02-sample-resources.ps1" -or $scriptName -eq "02-sample-resources.sh") {
                        Write-Host ""
                        Write-Host "  Sample resources"
                    }
                }

                # Skip if matches skip pattern
                if ($BOOTSTRAP_SKIP_PATTERN -and ($scriptName -match $BOOTSTRAP_SKIP_PATTERN)) {
                    Log-Info "[SKIP] Skipping $scriptName (matches skip pattern regex: $BOOTSTRAP_SKIP_PATTERN)"
                    $skippedCount++
                    continue
                }

                # Skip if doesn't match only pattern
                if ($BOOTSTRAP_ONLY_PATTERN -and ($scriptName -notmatch $BOOTSTRAP_ONLY_PATTERN)) {
                    Log-Info "[SKIP] Skipping $scriptName (doesn't match only pattern: $BOOTSTRAP_ONLY_PATTERN)"
                    $skippedCount++
                    continue
                }

                Log-Info "[EXEC] Executing: $scriptName"
                $scriptCount++

                # Execute PowerShell script
                $startTime = Get-Date

                try {
                    if ($SILENT_MODE) {
                        & $bootstrapScript.FullName *> $null
                    }
                    else {
                        & $bootstrapScript.FullName
                    }
                    $exitCode = $LASTEXITCODE

                    $endTime = Get-Date
                    $duration = [math]::Round(($endTime - $startTime).TotalSeconds, 2)

                    if ($exitCode -eq 0 -or $null -eq $exitCode) {
                        Log-Success "$scriptName completed (${duration}s)"
                        $successCount++
                    }
                    else {
                        Log-Error "$scriptName failed with exit code $exitCode (${duration}s)"
                        $failedCount++

                        if ($BOOTSTRAP_FAIL_FAST) {
                            Log-Error "Stopping bootstrap (BOOTSTRAP_FAIL_FAST=true)"
                            exit 1
                        }
                    }
                }
                catch {
                    $endTime = Get-Date
                    $duration = [math]::Round(($endTime - $startTime).TotalSeconds, 2)

                    Log-Error "$scriptName failed with error: $_  (${duration}s)"
                    $failedCount++

                    if ($BOOTSTRAP_FAIL_FAST) {
                        Log-Error "Stopping bootstrap (BOOTSTRAP_FAIL_FAST=true)"
                        exit 1
                    }
                }

                Write-Host ""
            }

            # Summary
            Write-Host ""
            Log-Info "========================================="
            Log-Info "Bootstrap Summary"
            Log-Info "========================================="
            Log-Info "Total scripts discovered: $($sortedScripts.Count)"
            Log-Info "Executed: $scriptCount"
            Log-Success "Successful: $successCount"

            if ($failedCount -gt 0) {
                Log-Error "Failed: $failedCount"
            }

            if ($skippedCount -gt 0) {
                Log-Info "Skipped: $skippedCount"
            }

            Log-Info "Completed at: $(Get-Date)"
            Log-Info "========================================="

            if ($failedCount -gt 0) {
                exit 1
            }

            Log-Success "Bootstrap completed successfully!"
        }
    }

    # ============================================================================
    # Setup Completed
    # ============================================================================

    Write-Host ""
    Write-Host ""
    if ($SILENT_MODE) {
        Write-Host "========================================="
        Write-Host "✅ Setup completed successfully!"
        Write-Host "========================================="
        Write-Host ""
        Write-Host "Admin credentials:"
        Write-Host "  URL:      ${PUBLIC_URL}/console"
        Write-Host "  Username: admin"
        Write-Host "  Password: admin"
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
        Write-Host "   3. Login with admin credentials:"
        Write-Host "      Username: admin" -ForegroundColor Cyan
        Write-Host "      Password: admin" -ForegroundColor Cyan
        Write-Host ""
    }
}
finally {
    # Cleanup
    if ($VERBOSE_MODE) {
        Write-Host ""
        Write-Host "[STOP] Stopping temporary server..." -ForegroundColor Cyan
    }
    if ($proc -and -not $proc.HasExited) {
        try {
            Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
        } catch { }
    }
    if ($consentProc -and -not $consentProc.HasExited) {
        try {
            Stop-Process -Id $consentProc.Id -Force -ErrorAction SilentlyContinue
        } catch { }
    }

    foreach ($tempLog in @($serverStdOutLog, $serverStdErrLog, $consentStdOutLog, $consentStdErrLog)) {
        if ($tempLog -and (Test-Path $tempLog)) {
            Remove-Item $tempLog -Force -ErrorAction SilentlyContinue
        }
    }

    # Restore SKIP_SECURITY to its previous state
    if (-not $hadSkipSecurity) {
        Remove-Item Env:SKIP_SECURITY -ErrorAction SilentlyContinue
    } else {
        $env:SKIP_SECURITY = $previousSkipSecurity
    }
}

exit 0
