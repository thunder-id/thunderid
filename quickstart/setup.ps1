#!/usr/bin/env pwsh
# Copyright 2025-2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

# Quickstart setup: the one-time setup for three deployments that run together, already wired to
# each other: a Control Plane, the Data Plane it manages (pushes applications/connections/roles/
# flows to), and a second, otherwise-ordinary Data Plane (auth-dp) dedicated to the Control
# Plane's own sign-in. Keeping that sign-in on its own deployment means logging into the Control
# Plane never depends on the one deployment it also manages: auth-dp's only job is to
# authenticate Control Plane operators, not to be managed itself.
#
# Run this once, then use start.ps1 to start all three. It creates the certs, key material and
# default resources for all three, and registers the managed Data Plane as a gateway on the
# Control Plane. Running it again is safe (it skips what already exists and upserts what doesn't
# need to change), but it also resets any setting changed since, so start.ps1 does not run it.
#
# auth-dp, the managed Data Plane, and the Control Plane share one generated certificate rather
# than each minting its own, which is what lets the Control Plane's bootstrap gateway resource
# name a fixed caCertificate: it is the one certificate this script generates, not a guess at what
# any of the three would have generated on their own. Sharing the certificate with the managed
# Data Plane too (rather than only with auth-dp, which is all correctness actually requires) keeps
# this to one self-signed certificate for an operator's browser to trust instead of two.
#
# The PowerShell counterpart of setup.sh.

$ErrorActionPreference = "Stop"

if ($PSVersionTable.PSVersion.Major -lt 7) {
    Write-Host "[ERROR] The quickstart requires PowerShell 7 (Core) or later." -ForegroundColor Red
    Write-Host "        Install it from https://github.com/PowerShell/PowerShell" -ForegroundColor Yellow
    exit 1
}

$SCRIPT_DIR = $PSScriptRoot
Set-Location $SCRIPT_DIR

foreach ($plane in @("auth-dp", "dp", "cp")) {
    if (-not (Test-Path -LiteralPath $plane -PathType Container)) {
        Write-Host "[ERROR] Expected an auth-dp/, a dp/ and a cp/ directory next to this script (this is meant to" -ForegroundColor Red
        Write-Host "        be run from inside the unzipped quickstart distribution)." -ForegroundColor Red
        exit 1
    }
}

. (Join-Path $SCRIPT_DIR "quickstart-bootstrap.ps1")

# Defensive: a prior run that didn't exit cleanly (e.g. the terminal was closed) can leave any of
# the three binaries still running. Clear all three ports before setup starts, so this run doesn't
# fail on a port conflict the operator never caused themselves.
Stop-PortListener -Port $CP_PORT
Stop-PortListener -Port $DP_PORT
Stop-PortListener -Port $AUTH_DP_PORT

# A marker from an earlier run must not let start.ps1 proceed if this run fails part-way.
Remove-Item -LiteralPath ".quickstart-setup-done" -Force -ErrorAction SilentlyContinue

Write-Host "========================================="
Write-Host "   ThunderID Quickstart Setup"
Write-Host "========================================="
Write-Host ""

# ============================================================================
# Account credentials: requested once, here, for two separate accounts.
#
#   Admin: the operator who signs into the Control Plane console. Created on auth-dp, which
#   authenticates it (ADMIN_USERNAME / ADMIN_PASSWORD, the names auth-dp's setup already reads).
#
#   Service Desk Admin: authored on the Control Plane and synced to the managed Data Plane, where it
#   signs into the Admin Console. Both planes get the same account, so the Data Plane has it from
#   the start and the Control Plane's copy arrives on the first sync unchanged
#   (SERVICE_DESK_USERNAME / SERVICE_DESK_PASSWORD).
#
# Left to each setup script, an operator would be prompted three times. This mirrors setup.ps1's
# own prompt (a generated password offered as the default, interactive only when stdin is a
# terminal) and exports the result so none of the nested scripts prompts again or generates a
# password of its own.
# ============================================================================

# New-RandomPassword returns a 12-character password mixing letters, digits, and special
# characters, the same way setup.ps1 does: the value flows through environment variables and a
# YAML bootstrap template, so the special set stays limited to shell- and YAML-safe punctuation.
function New-RandomPassword {
    $charset = ([char[]]'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789@#%+=_.?-')
    $special = [char[]]'@#%+=_.?-'
    while ($true) {
        $bytes = New-Object 'System.Byte[]' 12
        [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
        $candidate = -join ($bytes | ForEach-Object { $charset[$_ % $charset.Length] })
        if ($candidate -match '[0-9]' -and ($candidate.IndexOfAny($special) -ge 0)) { return $candidate }
    }
}

# Get-AccountCredentials fills an account's username and password from the environment, the
# prompt, or a default, in that order. Non-interactive with nothing supplied (e.g. CI): falls back
# to a generated password rather than leaving it empty, the same as setup.ps1 does on its own.
function Get-AccountCredentials {
    param([string]$Label, [string]$DefaultUser, [string]$UserVar, [string]$PassVar)

    $username = [Environment]::GetEnvironmentVariable($UserVar)
    $password = [Environment]::GetEnvironmentVariable($PassVar)
    $generated = $false
    $interactive = ([Console]::In -and -not [Console]::IsInputRedirected)

    if ($interactive -and (-not $username -or -not $password)) {
        Write-Host "Configure the $Label (press Enter to accept defaults):"
        Write-Host ""
        if (-not $username) {
            $answer = Read-Host "  Username [$DefaultUser]"
            $username = if ($answer) { $answer } else { $DefaultUser }
        }
        if (-not $password) {
            # Generated up front so it can be shown as the prompt default; a typed value overrides it.
            $password = New-RandomPassword
            $generated = $true
            $secure = Read-Host "  Password [$password]" -AsSecureString
            $typed = [System.Net.NetworkCredential]::new("", $secure).Password
            if ($typed) {
                $password = $typed
                $generated = $false
            }
        }
        Write-Host ""
    }
    if (-not $username) { $username = $DefaultUser }
    if (-not $password) {
        $password = New-RandomPassword
        $generated = $true
    }
    [Environment]::SetEnvironmentVariable($UserVar, $username)
    [Environment]::SetEnvironmentVariable($PassVar, $password)
    return $generated
}

$adminPasswordGenerated = Get-AccountCredentials -Label "admin who signs into the Control Plane console (created on auth-dp)" `
    -DefaultUser "admin" -UserVar "ADMIN_USERNAME" -PassVar "ADMIN_PASSWORD"
$serviceDeskPasswordGenerated = Get-AccountCredentials -Label "Service Desk Admin (created on the Control Plane, synced to the Data Plane)" `
    -DefaultUser "servicedeskadmin" -UserVar "SERVICE_DESK_USERNAME" -PassVar "SERVICE_DESK_PASSWORD"
$ADMIN_USERNAME = $env:ADMIN_USERNAME
$ADMIN_PASSWORD = $env:ADMIN_PASSWORD
$SERVICE_DESK_USERNAME = $env:SERVICE_DESK_USERNAME
$SERVICE_DESK_PASSWORD = $env:SERVICE_DESK_PASSWORD

Set-ConsoleUrls

# Invoke-PlaneSetup runs one plane's own setup script from that plane's directory with the given
# account. Each of those scripts removes ADMIN_USERNAME and ADMIN_PASSWORD from the environment
# when it finishes, so they are set again before every call rather than once above.
function Invoke-PlaneSetup {
    param([string]$Directory, [string]$Script, [string]$Username, [string]$Password)

    $env:ADMIN_USERNAME = $Username
    $env:ADMIN_PASSWORD = $Password
    Push-Location (Join-Path $SCRIPT_DIR $Directory)
    try {
        & (Join-Path (Get-Location) $Script)
        if ($LASTEXITCODE -ne 0) { throw "$Directory/$Script failed with exit code $LASTEXITCODE" }
    }
    finally {
        Pop-Location
    }
}

Write-Host "Setting up auth-dp (backs the Control Plane's own sign-in)..."
# auth-dp carries one application, the Control Plane console's own client (CP_CONSOLE); it has no
# console UI of its own.
Invoke-PlaneSetup -Directory "auth-dp" -Script "setup.ps1" -Username $ADMIN_USERNAME -Password $ADMIN_PASSWORD

# Share auth-dp's key material with the managed Data Plane and the Control Plane before their
# setups run, so the certificate step there finds everything already present and generates
# nothing of its own: all three end up presenting the identical TLS certificate, and, just as
# important for the Control Plane, signing with the identical JWT signing keys. That second part
# is what lets a token auth-dp issued (the Control Plane mints none of its own, and the managed
# Data Plane issues its own, independent tokens for its own Gate/Admin Console) verify on the
# Control Plane too, once its deployment.yaml is patched (at package time) to claim auth-dp's
# issuer as its own.
Write-Host ""
Write-Host "Sharing auth-dp's key material with the Data Plane and the Control Plane..."
foreach ($target in @("dp", "cp")) {
    $targetCerts = Join-Path $target "config/certs"
    New-Item -ItemType Directory -Force -Path $targetCerts | Out-Null
    foreach ($file in @("server.cert", "server.key", "signing.cert", "signing.key",
            "ecdsa-signing.cert", "ecdsa-signing.key", "crypto.key")) {
        Copy-Item -LiteralPath (Join-Path "auth-dp/config/certs" $file) -Destination $targetCerts -Force
    }
}

# The Control Plane's bootstrap gateway resource trusts the managed Data Plane by its certificate.
Set-DpCaCertificate

# The Control Plane registers the managed Data Plane as a gateway with a key generated here, and the
# Data Plane trusts that key's digest, so the link needs no key committed to the repository.
New-GatewayKey
Set-GatewayKeyTrust

Write-Host ""
Write-Host "Setting up the managed Data Plane..."
Invoke-PlaneSetup -Directory "dp" -Script "setup.ps1" -Username $SERVICE_DESK_USERNAME -Password $SERVICE_DESK_PASSWORD

Write-Host ""
Write-Host "Setting up the Control Plane, and registering the managed Data Plane as its gateway..."
# The Data Plane's own console is its own application (ADMIN), bootstrapped from the same
# file on the Data Plane and here with the same fixed ID, so the copy this Control Plane syncs
# replaces the Data Plane's rather than duplicating it.
Invoke-PlaneSetup -Directory "cp" -Script "setup-cp.ps1" -Username $SERVICE_DESK_USERNAME -Password $SERVICE_DESK_PASSWORD

Write-Host ""
Write-Host "Seeding the bootstrap values auto-apply needs on the managed Data Plane..."
# Needs the Data Plane running, so it is started just long enough to hold these values, which
# persist in its own stores.
$dpProcess = Start-Plane -Directory "dp" -Script "start.ps1"
try {
    Set-DpValues
}
finally {
    Stop-PortListener -Port $DP_PORT
    if ($dpProcess -and -not $dpProcess.HasExited) { Stop-Process -Id $dpProcess.Id -Force -ErrorAction SilentlyContinue }
}
New-Item -ItemType File -Force -Path ".quickstart-setup-done" | Out-Null

Write-Host ""
Write-Host "[OK] Setup complete. Run .\start.ps1 to start the quickstart." -ForegroundColor Green
Write-Host ""
Write-Host "  Accounts:"
Write-Host "    admin (signs into the Control Plane console, created on auth-dp)"
Write-Host "      Username: $ADMIN_USERNAME"
if ($adminPasswordGenerated) { Write-Host "      Password: $ADMIN_PASSWORD (generated)" }
Write-Host "    Service Desk Admin (created on the Control Plane, synced to the Data Plane)"
Write-Host "      Username: $SERVICE_DESK_USERNAME"
if ($serviceDeskPasswordGenerated) { Write-Host "      Password: $SERVICE_DESK_PASSWORD (generated)" }
