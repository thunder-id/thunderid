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

$BACKEND_PORT = if ($env:BACKEND_PORT) { [int]$env:BACKEND_PORT } else { 8090 }
$DEBUG_PORT = if ($env:DEBUG_PORT) { [int]$env:DEBUG_PORT } else { 2345 }
$DEBUG_MODE = $false
$WITH_CONSENT = if ($env:WITH_CONSENT -eq 'false') { $false } else { $true }
$RESOURCES_FILE = ""
$ENV_FILE = ""

# Parse command line arguments
$i = 0
while ($i -lt $args.Count) {
    switch ($args[$i]) {
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
        '--port' {
            $i++
            if ($i -lt $args.Count) {
                $BACKEND_PORT = [int]$args[$i]
                $i++
            }
            else {
                Write-Host "Missing value for --port" -ForegroundColor Red
                exit 1
            }
            break
        }
        '--without-consent' {
            $WITH_CONSENT = $false
            $i++
            break
        }
        '--env' {
            $i++
            if ($i -lt $args.Count) {
                $ENV_FILE = $args[$i]
                $i++
            }
            else {
                Write-Host "Missing value for --env" -ForegroundColor Red
                exit 1
            }
            break
        }
        '--help' {
            Write-Host "$PRODUCT_NAME Server Startup Script"
            Write-Host ""
            Write-Host "Usage: .\start.ps1 [resources_file] [options]"
            Write-Host ""
            Write-Host "Arguments:"
            Write-Host "  resources_file       Path to exported resources YAML file (optional)"
            Write-Host ""
            Write-Host "Options:"
            Write-Host "  --env FILE           Path to env file with KEY=VALUE variables"
            Write-Host "  --debug              Enable debug mode with remote debugging"
            Write-Host "  --port PORT          Set application port (default: 8090)"
            Write-Host "  --debug-port PORT    Set debug port (default: 2345)"
            Write-Host "  --without-consent    Disable the bundled consent server"
            Write-Host "  --help               Show this help message"
            Write-Host ""
            Write-Host "First-Time Setup:"
            Write-Host "  For initial setup, use the setup script:"
            Write-Host "    .\setup.ps1"
            Write-Host ""
            Write-Host "  Then start the server normally:"
            Write-Host "    .\start.ps1"
            Write-Host ""
            Write-Host "Examples:"
            Write-Host "  .\start.ps1                                   Start server normally"
            Write-Host "  .\start.ps1 --debug                           Start in debug mode"
            Write-Host "  .\start.ps1 --port 9090                       Start on custom port"
            Write-Host "  .\start.ps1 cloud.yml --env my.env            Start with exported resources and env"
            exit 0
        }
        default {
            if (-not $args[$i].StartsWith('-') -and $RESOURCES_FILE -eq "") {
                $RESOURCES_FILE = $args[$i]
                $i++
            }
            else {
                Write-Host "Unknown option: $($args[$i])" -ForegroundColor Yellow
                Write-Host "Use --help for usage information"
                exit 1
            }
        }
    }
}

# Resolve relative paths to absolute.
if ($RESOURCES_FILE -ne "" -and -not [System.IO.Path]::IsPathRooted($RESOURCES_FILE)) {
    $RESOURCES_FILE = Join-Path (Get-Location).Path $RESOURCES_FILE
}
if ($ENV_FILE -ne "" -and -not [System.IO.Path]::IsPathRooted($ENV_FILE)) {
    $ENV_FILE = Join-Path (Get-Location).Path $ENV_FILE
}

# Exit on any error
$ErrorActionPreference = 'Stop'

function Setup-DeclarativeResources {
    $scriptDir = Split-Path -Parent $MyInvocation.ScriptName
    $resourcesBase = Join-Path $scriptDir "repository\resources"

    # Load and export env vars from the env file.
    if ($ENV_FILE -ne "") {
        if (-not (Test-Path $ENV_FILE)) {
            Write-Host "Error: env file not found: $ENV_FILE" -ForegroundColor Red
            exit 1
        }
        Write-Host "Loading environment from $ENV_FILE ..."
        Get-Content $ENV_FILE | ForEach-Object {
            $line = $_.TrimEnd()
            if ($line -eq "" -or $line.StartsWith('#')) { return }
            if ($line -match '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$') {
                $key = $Matches[1]
                $value = $Matches[2]
                if ($value -match '^\[') {
                    # JSON array — expand into KEY_0, KEY_1, ...
                    try {
                        $arr = $value | ConvertFrom-Json
                        $idx = 0
                        foreach ($elem in $arr) {
                            [System.Environment]::SetEnvironmentVariable("${key}_${idx}", $elem, 'Process')
                            $idx++
                        }
                    } catch {
                        Write-Warning "Failed to parse JSON array for key '$key': $_"
                        [System.Environment]::SetEnvironmentVariable($key, $value, 'Process')
                    }
                } else {
                    [System.Environment]::SetEnvironmentVariable($key, $value, 'Process')
                }
            }
        }
    }

    # Split the combined resources YAML and place each document in its type directory.
    if ($RESOURCES_FILE -ne "") {
        if (-not (Test-Path $RESOURCES_FILE)) {
            Write-Host "Error: resources file not found: $RESOURCES_FILE" -ForegroundColor Red
            exit 1
        }
        Write-Host "Setting up declarative resources from $RESOURCES_FILE ..."

        $typeMap = @{
            'application'         = 'applications'
            'flow'                = 'flows'
            'group'               = 'groups'
            'identity_provider'   = 'identity_providers'
            'layout'              = 'layouts'
            'notification_sender' = 'notification_senders'
            'organization_unit'   = 'organization_units'
            'resource_server'     = 'resource_servers'
            'role'                = 'roles'
            'theme'               = 'themes'
            'translation'         = 'translations'
            'user'                = 'users'
            'user_schema'         = 'user_schemas'
        }

        $lines = Get-Content $RESOURCES_FILE
        $docLines = [System.Collections.Generic.List[string]]::new()
        $currentFile = ""
        $currentType = ""

        $flushDoc = {
            if ($docLines.Count -eq 0 -or $currentType -eq "") {
                $docLines.Clear()
                $script:currentFile = ""
                $script:currentType = ""
                return
            }
            $dir = if ($typeMap.ContainsKey($currentType)) { $typeMap[$currentType] } else { "${currentType}s" }
            $targetDir = Join-Path $resourcesBase $dir
            if (-not (Test-Path $targetDir)) { New-Item -ItemType Directory -Path $targetDir -Force | Out-Null }
            $fname = if ($currentFile -ne "") { $currentFile } else { "resource.yaml" }
            $outPath = Join-Path $targetDir $fname
            $docLines | Set-Content -Path $outPath -Encoding UTF8
            Write-Host "  Placed: $dir\$fname"
            $docLines.Clear()
            $script:currentFile = ""
            $script:currentType = ""
        }

        foreach ($line in $lines) {
            if ($line -match '^---\s*$') {
                & $flushDoc
            }
            elseif ($line -match '^# File:\s*(.+)$') {
                $currentFile = $Matches[1].Trim()
            }
            elseif ($line -match '^# resource_type:\s*(.+)$') {
                $currentType = $Matches[1].Trim()
            }
            else {
                $docLines.Add($line)
            }
        }
        & $flushDoc
        Write-Host "Declarative resources ready."
    }
}

# Set up declarative resources if a resources file or env file was provided.
if ($RESOURCES_FILE -ne "" -or $ENV_FILE -ne "") {
    Setup-DeclarativeResources
}

function Stop-PortListener {
    param (
        [int]$port
    )

    Write-Host "Checking for processes listening on TCP port $port..."

    # Try Get-NetTCPConnection first (Windows 8/Server 2012+)
    try {
        $pids = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction Stop | Select-Object -ExpandProperty OwningProcess -Unique
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
                    if ([int]::TryParse($procId, [ref]$null)) { $pids += [int]$procId }
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

# Kill ports before binding
Stop-PortListener -port $BACKEND_PORT
if ($DEBUG_MODE) { Stop-PortListener -port $DEBUG_PORT }
Start-Sleep -Seconds 1

# Check if Delve is available for debug mode
if ($DEBUG_MODE) {
    if (-not (Get-Command dlv -ErrorAction SilentlyContinue)) {
        Write-Host "[ERROR] Debug mode requires Delve debugger" -ForegroundColor Red
        Write-Host ""
        Write-Host "[INFO] Install Delve using:" -ForegroundColor Cyan
        Write-Host "   go install github.com/go-delve/delve/cmd/dlv@latest" -ForegroundColor Cyan
        Write-Host ""
        Write-Host "[INFO] Add Delve to PATH and re-run this script with --debug" -ForegroundColor Cyan
        exit 1
    }
}

# Resolve the server executable path
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$possible = @(
    (Join-Path $scriptDir "${BINARY_NAME}.exe"),
    (Join-Path $scriptDir $BINARY_NAME)
)
$serverExecPath = $possible | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $serverExecPath) {
    # Fallback (will work if PATH or current dir has it)
    $serverExecPath = Join-Path $scriptDir $BINARY_NAME
}

$proc = $null
try {
    # Start consent server if enabled
    $consentProc = $null
    if ($WITH_CONSENT) {
        $consentDir = Join-Path $scriptDir "consent"
        if (-not (Test-Path $consentDir)) {
            Write-Host "[ERROR] Consent server is enabled but consent directory not found: $consentDir" -ForegroundColor Red
            exit 1
        }
        $consentBinary = @(
            (Join-Path $consentDir 'consent-server.exe'),
            (Join-Path $consentDir 'consent-server')
        ) | Where-Object { Test-Path $_ } | Select-Object -First 1
        if (-not $consentBinary) {
            Write-Host "[ERROR] Consent server is enabled but consent-server binary not found in: $consentDir" -ForegroundColor Red
            exit 1
        }
        Write-Host "[INFO] Starting Consent Server..."
        $consentPort = if ($env:CONSENT_SERVER_PORT) { $env:CONSENT_SERVER_PORT } else { "9090" }
        $consentProc = Start-Process -FilePath $consentBinary -WorkingDirectory $consentDir -NoNewWindow -PassThru
        $consentTimeout = 30
        $consentElapsed = 0
        while ($consentElapsed -lt $consentTimeout) {
            if ($consentProc.HasExited) {
                Write-Host "[ERROR] Consent server process exited unexpectedly (code $($consentProc.ExitCode))" -ForegroundColor Red
                exit 1
            }
            try {
                $resp = Invoke-WebRequest -Uri "http://localhost:${consentPort}/health/readiness" -UseBasicParsing -ErrorAction Stop
                if ($resp.StatusCode -eq 200) {
                    Write-Host "[INFO] Consent server is ready"
                    break
                }
            } catch { }
            Start-Sleep -Seconds 1
            $consentElapsed++
        }
        if ($consentElapsed -ge $consentTimeout) {
            Write-Host "[ERROR] Consent server failed to become ready within ${consentTimeout}s" -ForegroundColor Red
            exit 1
        }
    }

    if ($DEBUG_MODE) {
        Write-Host "[INFO] Starting $PRODUCT_NAME Server in DEBUG mode..."
        Write-Host "[INFO] Application will run on: https://localhost:$BACKEND_PORT"
        Write-Host "[INFO] Remote debugger will listen on: localhost:$DEBUG_PORT"
        Write-Host ""
        Write-Host "[INFO] Connect using remote debugging configuration:" -ForegroundColor Gray
        Write-Host "   Host: 127.0.0.1, Port: $DEBUG_PORT" -ForegroundColor Gray
        Write-Host ""

        # Start Delve in headless mode
        $dlvArgs =  @(
            'exec'
            "--listen=:$DEBUG_PORT"
            '--headless=true'
            '--api-version=2'
            '--accept-multiclient'
            '--continue'
            $serverExecPath
        )
        $proc = Start-Process -FilePath dlv -ArgumentList $dlvArgs -WorkingDirectory $scriptDir -NoNewWindow -PassThru
    }
    else {
        Write-Host "[INFO] Starting $PRODUCT_NAME Server ..."

        # Export BACKEND_PORT for the child process
        $env:BACKEND_PORT = $BACKEND_PORT
        $proc = Start-Process -FilePath $serverExecPath -WorkingDirectory $scriptDir -NoNewWindow -PassThru
    }

    Write-Host ""
    Write-Host "[INFO] Server running. PID: $($proc.Id)"
    Write-Host ""
    Write-Host "[INFO] Frontend Apps:"
    Write-Host "   [GATE] Gate (Login/Register): $BACKEND_PORT/gate"
    Write-Host "   [DEV]  Console (System Management): $BACKEND_PORT/console"
    Write-Host ""

    Write-Host "Press Ctrl+C to stop the server."

    # Wait for the background process. This will block until the process exits.
    Wait-Process -Id $proc.Id
}
finally {
    Write-Host "`n[STOP] Stopping server..."
    if ($proc -and -not $proc.HasExited) {
        try { Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue } catch { }
    }
    if ($consentProc -and -not $consentProc.HasExited) {
        try {
            # Stop child processes first (the consent-server binary started by start.ps1)
            Get-CimInstance Win32_Process -Filter "ParentProcessId = $($consentProc.Id)" -ErrorAction SilentlyContinue |
                ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
            Stop-Process -Id $consentProc.Id -Force -ErrorAction SilentlyContinue
        } catch { }
    }
}
