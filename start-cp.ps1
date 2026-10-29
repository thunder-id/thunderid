#!/usr/bin/env pwsh
# Copyright 2025-2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

$PRODUCT_NAME = "ThunderID Control Plane"
$BINARY_NAME = "thunderid-cp"

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

$BACKEND_PORT = if ($env:BACKEND_PORT) { [int]$env:BACKEND_PORT } else { 8095 }
$DEBUG_PORT = if ($env:DEBUG_PORT) { [int]$env:DEBUG_PORT } else { 2345 }
$DEBUG_MODE = $false
$RESOURCES_FILE = ""
$ENV_FILE = ""
$BOOTSTRAP_MODE = $false
$BOOTSTRAP_AND_SERVE = $false
$BOOTSTRAP_EXTRA_ARGS = @()
$ADMIN_USERNAME_FLAG_SET = $false
$ADMIN_PASSWORD_FLAG_SET = $false
$script:bootstrapExitCode = 0

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
        '--bootstrap' {
            # Run the in-process bootstrap one-shot (create default resources) instead
            # of starting the long-running server, then exit.
            $BOOTSTRAP_MODE = $true
            $i++
            break
        }
        '--bootstrap-and-serve' {
            # Run the in-process bootstrap one-shot first, then start the long-running
            # server (convenient for local/dev: seed once, then serve).
            $BOOTSTRAP_AND_SERVE = $true
            $i++
            break
        }
        '--console-redirect-uris' {
            $i++
            if ($i -lt $args.Count) {
                $BOOTSTRAP_EXTRA_ARGS += @('--console-redirect-uris', $args[$i])
                $i++
            }
            else {
                Write-Host "Missing value for --console-redirect-uris" -ForegroundColor Red
                exit 1
            }
            break
        }
        '--admin-username' {
            $i++
            if ($i -lt $args.Count) {
                $BOOTSTRAP_EXTRA_ARGS += @('--admin-username', $args[$i])
                $ADMIN_USERNAME_FLAG_SET = $true
                $i++
            }
            else {
                Write-Host "Missing value for --admin-username" -ForegroundColor Red
                exit 1
            }
            break
        }
        '--admin-password' {
            $i++
            if ($i -lt $args.Count) {
                $BOOTSTRAP_EXTRA_ARGS += @('--admin-password', $args[$i])
                $ADMIN_PASSWORD_FLAG_SET = $true
                $i++
            }
            else {
                Write-Host "Missing value for --admin-password" -ForegroundColor Red
                exit 1
            }
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
            Write-Host "$PRODUCT_NAME Startup Script"
            Write-Host ""
            Write-Host "Usage: .\start-cp.ps1 [resources_file] [options]"
            Write-Host ""
            Write-Host "Arguments:"
            Write-Host "  resources_file       Path to exported resources YAML file (optional)"
            Write-Host ""
            Write-Host "Options:"
            Write-Host "  --env FILE           Path to env file with KEY=VALUE variables"
            Write-Host "  --debug              Enable debug mode with remote debugging"
            Write-Host "  --port PORT          Set application port (default: 8095)"
            Write-Host "  --debug-port PORT    Set debug port (default: 2345)"
            Write-Host "  --bootstrap          Create default resources in-process, then exit (used by setup-cp.ps1)"
            Write-Host "  --bootstrap-and-serve Create default resources in-process, then start the server"
            Write-Host "  --admin-username VALUE  Username for the default admin user (--bootstrap-and-serve only)"
            Write-Host "                          Optional; falls back to ADMIN_USERNAME env var, then defaults to 'admin'"
            Write-Host "  --admin-password VALUE  Password for the default admin user (--bootstrap-and-serve only)"
            Write-Host "                          Falls back to ADMIN_PASSWORD env var; bootstrap fails if neither is set"
            Write-Host "  --help               Show this help message"
            Write-Host ""
            Write-Host "First-Time Setup:"
            Write-Host "  For initial setup, use the setup script:"
            Write-Host "    .\setup-cp.ps1"
            Write-Host ""
            Write-Host "  Then start the Control Plane normally:"
            Write-Host "    .\start-cp.ps1"
            Write-Host ""
            Write-Host "Examples:"
            Write-Host "  .\start-cp.ps1                                Start Control Plane normally"
            Write-Host "  .\start-cp.ps1 --debug                        Start in debug mode"
            Write-Host "  .\start-cp.ps1 --port 9095                    Start on custom port"
            Write-Host "  .\start-cp.ps1 cloud.yml --env my.env         Start with exported resources and env"
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

# --admin-username/--admin-password only make sense when bootstrapping happens in this
# invocation, and specifically only for --bootstrap-and-serve — --bootstrap is the
# seed-only mode setup-cp.ps1 drives internally via environment variables, not flags.
if (($ADMIN_USERNAME_FLAG_SET -or $ADMIN_PASSWORD_FLAG_SET) -and -not $BOOTSTRAP_AND_SERVE) {
    Write-Host "Error: --admin-username/--admin-password are only valid together with --bootstrap-and-serve" -ForegroundColor Red
    exit 1
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

# Load and export env vars from the env file.
function Load-EnvFile {
    if ($ENV_FILE -eq "") { return }
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
                    Write-Error "Failed to parse JSON array for key '$key': $_"
                    exit 1
                }
            } else {
                [System.Environment]::SetEnvironmentVariable($key, $value, 'Process')
            }
        }
    }
}

# Load env vars before starting the binary so substitution works in resource files.
if ($ENV_FILE -ne "") {
    Load-EnvFile
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

# Kill ports before binding. The bootstrap one-shot does not bind the server port,
# so the check is skipped in that mode.
if (-not $BOOTSTRAP_MODE) {
    Stop-PortListener -port $BACKEND_PORT
    if ($DEBUG_MODE) { Stop-PortListener -port $DEBUG_PORT }
    Start-Sleep -Seconds 1
}

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

# Resolve the Control Plane executable path
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
    # Bootstrap: create the default resources in-process. Runs for both --bootstrap
    # (seed-only) and --bootstrap-and-serve.
    $runServer = (-not $BOOTSTRAP_MODE)
    if ($BOOTSTRAP_MODE -or $BOOTSTRAP_AND_SERVE) {
        Write-Host "[INFO] Running $PRODUCT_NAME bootstrap ..."
        $env:BACKEND_PORT = $BACKEND_PORT
        $bootstrapArgs = @('bootstrap') + $BOOTSTRAP_EXTRA_ARGS
        $bp = Start-Process -FilePath $serverExecPath -ArgumentList $bootstrapArgs -WorkingDirectory $scriptDir -NoNewWindow -PassThru -Wait
        $script:bootstrapExitCode = $bp.ExitCode
        if ($bp.ExitCode -eq 0) {
            Write-Host "[INFO] Bootstrap completed."
        }
        else {
            Write-Host "[ERROR] Bootstrap failed." -ForegroundColor Red
            $runServer = $false
        }
    }

    if ($runServer) {
        $resourcesArgs = @()
        if ($RESOURCES_FILE -ne "") { $resourcesArgs = @('-resources', $RESOURCES_FILE) }

        if ($DEBUG_MODE) {
            Write-Host "[INFO] Starting $PRODUCT_NAME in DEBUG mode..."
            Write-Host "[INFO] Application will run on: https://localhost:$BACKEND_PORT"
            Write-Host "[INFO] Remote debugger will listen on: localhost:$DEBUG_PORT"
            Write-Host ""
            Write-Host "[INFO] Connect using remote debugging configuration:" -ForegroundColor Gray
            Write-Host "   Host: 127.0.0.1, Port: $DEBUG_PORT" -ForegroundColor Gray
            Write-Host ""

            # Export BACKEND_PORT for the child process (mirrors the non-debug branch)
            $env:BACKEND_PORT = $BACKEND_PORT

            # Start Delve in headless mode
            $dlvArgs = @(
                'exec'
                "--listen=:$DEBUG_PORT"
                '--headless=true'
                '--api-version=2'
                '--accept-multiclient'
                '--continue'
                $serverExecPath
                '--'
            ) + $resourcesArgs
            $proc = Start-Process -FilePath dlv -ArgumentList $dlvArgs -WorkingDirectory $scriptDir -NoNewWindow -PassThru
        }
        else {
            Write-Host "[INFO] Starting $PRODUCT_NAME ..."

            # Export BACKEND_PORT for the child process
            $env:BACKEND_PORT = $BACKEND_PORT
            $proc = Start-Process -FilePath $serverExecPath -ArgumentList $resourcesArgs -WorkingDirectory $scriptDir -NoNewWindow -PassThru
        }

        Write-Host ""
        Write-Host "[INFO] Control Plane running. PID: $($proc.Id)"
        Write-Host ""
        Write-Host "[INFO] Console (System Management): https://localhost:$BACKEND_PORT/console"
        Write-Host ""

        Write-Host "Press Ctrl+C to stop."

        # Wait for the background process. This will block until the process exits.
        Wait-Process -Id $proc.Id
    }
}
finally {
    Write-Host "`n[STOP] Stopping Control Plane..."
    if ($proc -and -not $proc.HasExited) {
        try { Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue } catch { }
    }
}

# Propagate a failed bootstrap (seed-only or bootstrap-and-serve), then exit cleanly
# after a seed-only run.
if ($script:bootstrapExitCode -ne 0) { exit $script:bootstrapExitCode }
if ($BOOTSTRAP_MODE) { exit 0 }
