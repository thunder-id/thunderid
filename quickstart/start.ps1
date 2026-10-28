#!/usr/bin/env pwsh
# Copyright 2025-2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

# Quickstart start: starts the three deployments that setup.ps1 prepared: auth-dp (the Control
# Plane's own sign-in), the managed Data Plane, and the Control Plane. Run setup.ps1 once first.
#
# The PowerShell counterpart of start.sh.

$ErrorActionPreference = "Stop"

if ($PSVersionTable.PSVersion.Major -lt 7) {
    Write-Host "[ERROR] The quickstart requires PowerShell 7 (Core) or later." -ForegroundColor Red
    Write-Host "        Install it from https://github.com/PowerShell/PowerShell" -ForegroundColor Yellow
    exit 1
}

$SCRIPT_DIR = $PSScriptRoot
Set-Location $SCRIPT_DIR

if (-not (Test-Path -LiteralPath ".quickstart-setup-done")) {
    Write-Host "[ERROR] The quickstart has not been set up yet. Run .\setup.ps1 first." -ForegroundColor Red
    exit 1
}

. (Join-Path $SCRIPT_DIR "quickstart-bootstrap.ps1")

function Stop-Quickstart {
    Write-Host ""
    Write-Host "Stopping Control Plane, Data Plane and auth-dp..."
    Stop-PortListener -Port $CP_PORT
    Stop-PortListener -Port $DP_PORT
    Stop-PortListener -Port $AUTH_DP_PORT
    Write-Host "[OK] Stopped."
}

Stop-PortListener -Port $CP_PORT
Stop-PortListener -Port $DP_PORT
Stop-PortListener -Port $AUTH_DP_PORT

Write-Host "========================================="
Write-Host "   ThunderID Quickstart"
Write-Host "========================================="
Write-Host ""

$processes = @()
try {
    Write-Host "Starting auth-dp..."
    $processes += Start-Plane -Directory "auth-dp" -Script "start.ps1"

    Write-Host "Starting the Data Plane..."
    $processes += Start-Plane -Directory "dp" -Script "start.ps1"

    Write-Host "Starting the Control Plane..."
    $processes += Start-Plane -Directory "cp" -Script "start-cp.ps1"

    Start-Sleep -Seconds 2

    Write-Host ""
    Write-Host "Quickstart running:"
    Write-Host "  auth-dp (Control Plane sign-in):"
    Write-Host "      Gate (Login/Register): https://localhost:$AUTH_DP_PORT/gate"
    Write-Host "  Data Plane (managed):"
    Write-Host "      Gate (Login/Register): https://localhost:$DP_PORT/gate"
    Write-Host "      Admin Console (Users, Agents, Groups, Roles): https://localhost:$DP_PORT/admin"
    Write-Host "  Control Plane Console: https://localhost:$CP_PORT/console"
    Write-Host ""
    Write-Host "  The Data Plane is already registered as a gateway on the Control Plane: an application,"
    Write-Host "  connection, role, or flow created on the Control Plane is captured and applied to the"
    Write-Host "  Data Plane automatically (gateway.auto_apply, on for this pack alone). Signing into the"
    Write-Host "  Control Plane's console authenticates against auth-dp, a separate deployment, not the"
    Write-Host "  Data Plane it manages. Neither plane's own console shows applications/connections/"
    Write-Host "  roles/flows (those are authored on the Control Plane). Check the Data Plane's own API,"
    Write-Host "  or GET /configuration-versions and GET /gateways/{id}/applied-version on the Control"
    Write-Host "  Plane, to see what was synced."
    Write-Host ""
    Write-Host "Press Ctrl+C to stop all three."

    Wait-Process -Id ($processes | ForEach-Object { $_.Id })
}
finally {
    Stop-Quickstart
    foreach ($process in $processes) {
        if ($process -and -not $process.HasExited) { Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue }
    }
}
