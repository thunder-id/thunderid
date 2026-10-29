#!/usr/bin/env pwsh
# Copyright 2025-2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

$PRODUCT_NAME = "ThunderID Control Plane"

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

# Control Plane Setup Script
# Prepares a Control Plane for its first start:
# 1. Generates the TLS certificate and signing and encryption keys if they do not exist
# 2. Creates the default resources by running `start-cp.ps1 --bootstrap`, an in-process bootstrap
#    that needs no running server and leaves nothing to stop
# 3. Exits cleanly
#
# Unlike setup.ps1, this configures no Direct Auth Secret: that gates the Direct API, a
# data-plane-only concern the Control Plane's deployment.yaml declares no key for.

# Exit on any error
$ErrorActionPreference = 'Stop'

# Default settings
$VERBOSE_MODE = $false
$SILENT_MODE = $true
$ADMIN_USERNAME_PROVIDED = if ($env:ADMIN_USERNAME) { $true } else { $false }
$ADMIN_PASSWORD_PROVIDED = if ($env:ADMIN_PASSWORD) { $true } else { $false }
$ADMIN_USERNAME = if ($env:ADMIN_USERNAME) { $env:ADMIN_USERNAME } else { "admin" }
# Left empty when not supplied: Set-AdminPassword (below) generates a random password
# in that case, rather than falling back to a fixed, predictable value.
$ADMIN_PASSWORD = if ($env:ADMIN_PASSWORD) { $env:ADMIN_PASSWORD } else { "" }
$ADMIN_PASSWORD_GENERATED = $false
# Set when key material is generated this run (controls the one-time notice).
$CERTS_GENERATED = $false

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
    Write-Host "Usage: .\setup-cp.ps1 [options]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  --verbose                Enable detailed setup output"
    Write-Host "  --admin-username VALUE   Username for the default admin user (default: admin)"
    Write-Host "                           Falls back to ADMIN_USERNAME env var if flag not set"
    Write-Host "  --admin-password VALUE   Password for the default admin user"
    Write-Host "                           Falls back to ADMIN_PASSWORD env var; generated if unset"
    Write-Host "  --help                   Show this help message"
    Write-Host ""
    Write-Host "Description:"
    Write-Host "  This script performs initial setup by:"
    Write-Host "  1. Generating the TLS certificate and signing and encryption keys if they do not exist"
    Write-Host "  2. Creating the default resources with an in-process bootstrap (no server is started)"
    Write-Host ""
    Write-Host "  After setup completes, use '.\start-cp.ps1' to start $PRODUCT_NAME normally."
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
        '--admin-username' {
            $i++
            if ($i -lt $args.Count -and $args[$i] -notlike '--*') {
                $ADMIN_USERNAME = $args[$i]
                $ADMIN_USERNAME_PROVIDED = $true
                $i++
            }
            else {
                Write-Host "--admin-username requires a non-empty value" -ForegroundColor Red
                exit 1
            }
            break
        }
        '--admin-password' {
            $i++
            if ($i -lt $args.Count -and $args[$i] -notlike '--*') {
                $ADMIN_PASSWORD = $args[$i]
                $ADMIN_PASSWORD_PROVIDED = $true
                $i++
            }
            else {
                Write-Host "--admin-password requires a non-empty value" -ForegroundColor Red
                exit 1
            }
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
    $configFile = Resolve-ConfigFile

    if (-not $configFile) {
        Log-Warning "Configuration file not found, using defaults"
        return $false
    }

    Log-Debug "Reading configuration from: $configFile"

    # Try yq first (YAML parser)
    if (Get-Command yq -ErrorAction SilentlyContinue) {
        $script:HOSTNAME = & yq eval '.server.hostname // "localhost"' $configFile 2>$null
        $script:PORT = & yq eval '.server.port // 8095' $configFile 2>$null
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
            $script:PORT = 8095
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

# Resolve-ConfigFile returns the path to the deployment.yaml in use, or "" if not found.
function Resolve-ConfigFile {
    # The package directory comes first: it holds the deployment.yaml the bootstrap child loads, and
    # this script can be started from anywhere.
    foreach ($candidate in @((Join-Path $PSScriptRoot "deployment.yaml"), $CONFIG_FILE, ".\backend\cmd\cpserver\deployment.yaml")) {
        if (Test-Path $candidate) { return $candidate }
    }
    return ""
}

# Set-AdminPassword ensures the default admin account is usable out of the box while staying secure by
# default: it uses the provided value, or generates a random one. This intentionally regenerates every
# run where no value is supplied (no persisted value to check), so re-running setup-cp.ps1 with nothing
# explicit set is also how an operator resets the password.
function Set-AdminPassword {
    if ($ADMIN_PASSWORD) {
        return
    }

    # Generate a 12-character password mixing letters, digits, and special characters.
    # The special set is limited to shell- and YAML-safe punctuation, because the value
    # flows through environment variables and the bundle's YAML template before it is
    # stored. Regenerate until the result contains at least one digit and one special
    # character so it reliably looks like a password.
    $charset = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789@#%+=_.?-'.ToCharArray()
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    do {
        $bytes = New-Object 'System.Byte[]' 12
        $rng.GetBytes($bytes)
        $password = -join ($bytes | ForEach-Object { $charset[$_ % $charset.Length] })
    } while (-not ($password -match '[0-9]' -and $password -match '[@#%+=_.?-]'))

    $script:ADMIN_PASSWORD = $password
    $script:ADMIN_PASSWORD_GENERATED = $true
}

# Show-AdminCredentialsNotice prints the generated admin password once, so the operator can capture it.
# Only shown when the password was generated during this run (not for operator-supplied values).
function Show-AdminCredentialsNotice {
    if (-not $ADMIN_PASSWORD_GENERATED) { return }
    Write-Host "Admin credentials:"
    Write-Host "  Username: $ADMIN_USERNAME"
    Write-Host "  Password: $ADMIN_PASSWORD"
    Write-Host "  Sign in to the Console with these credentials."
    Write-Host ""
}

# Write DER bytes as a PEM file (64-char lines, LF endings) of the given block type.
function Write-PemFile {
    param(
        [string]$Path,
        [string]$Type,
        [byte[]]$Der
    )
    $b64 = [Convert]::ToBase64String($Der)
    $sb = New-Object System.Text.StringBuilder
    [void]$sb.Append("-----BEGIN $Type-----`n")
    for ($i = 0; $i -lt $b64.Length; $i += 64) {
        $len = [Math]::Min(64, $b64.Length - $i)
        [void]$sb.Append($b64.Substring($i, $len))
        [void]$sb.Append("`n")
    }
    [void]$sb.Append("-----END $Type-----`n")
    [System.IO.File]::WriteAllText($Path, $sb.ToString())
}

# Generate a self-signed cert/key PEM pair if absent, using .NET (no openssl needed on Windows).
function New-SelfSignedCertPair {
    param(
        [string]$CertFile,
        [string]$KeyFile,
        [string]$Algo
    )
    if ((Test-Path $CertFile) -and (Test-Path $KeyFile)) {
        return
    }

    $subject = "CN=localhost, OU=$PRODUCT_NAME, O=ThunderID"
    $san = New-Object System.Security.Cryptography.X509Certificates.SubjectAlternativeNameBuilder
    $san.AddDnsName("localhost")
    $san.AddIpAddress([System.Net.IPAddress]::Parse("127.0.0.1"))

    if ($Algo -eq 'ecdsa') {
        $key = [System.Security.Cryptography.ECDsa]::Create([System.Security.Cryptography.ECCurve+NamedCurves]::nistP256)
        $req = [System.Security.Cryptography.X509Certificates.CertificateRequest]::new(
            $subject, $key, [System.Security.Cryptography.HashAlgorithmName]::SHA256)
        $days = 3650
    }
    else {
        $key = [System.Security.Cryptography.RSA]::Create(2048)
        $req = [System.Security.Cryptography.X509Certificates.CertificateRequest]::new(
            $subject, $key, [System.Security.Cryptography.HashAlgorithmName]::SHA256,
            [System.Security.Cryptography.RSASignaturePadding]::Pkcs1)
        $days = 365
    }
    $req.CertificateExtensions.Add($san.Build())

    $notBefore = [System.DateTimeOffset]::UtcNow.AddDays(-1)
    $notAfter = [System.DateTimeOffset]::UtcNow.AddDays($days)
    $cert = $req.CreateSelfSigned($notBefore, $notAfter)

    Write-PemFile -Path $CertFile -Type "CERTIFICATE" -Der $cert.RawData
    Write-PemFile -Path $KeyFile -Type "PRIVATE KEY" -Der $key.ExportPkcs8PrivateKey()
    $cert.Dispose()
    $key.Dispose()
    $script:CERTS_GENERATED = $true
}

# Generate the server TLS, JWT signing, and AES key material if absent (reused on later runs).
function Set-Certificates {
    $configFile = Resolve-ConfigFile
    if ($configFile) {
        $certDir = Join-Path (Split-Path -Parent $configFile) "config/certs"
    }
    else {
        $certDir = "./config/certs"
    }
    New-Item -ItemType Directory -Force -Path $certDir | Out-Null
    # Absolute path so the .NET file writes below resolve correctly (they ignore the PowerShell location).
    $certDir = (Resolve-Path -LiteralPath $certDir).Path

    New-SelfSignedCertPair -CertFile (Join-Path $certDir "server.cert") -KeyFile (Join-Path $certDir "server.key") -Algo rsa
    New-SelfSignedCertPair -CertFile (Join-Path $certDir "signing.cert") -KeyFile (Join-Path $certDir "signing.key") -Algo rsa
    New-SelfSignedCertPair -CertFile (Join-Path $certDir "ecdsa-signing.cert") -KeyFile (Join-Path $certDir "ecdsa-signing.key") -Algo ecdsa

    $cryptoKey = Join-Path $certDir "crypto.key"
    if (-not (Test-Path $cryptoKey)) {
        $bytes = New-Object 'System.Byte[]' 32
        [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
        $hex = ($bytes | ForEach-Object { $_.ToString('x2') }) -join ''
        [System.IO.File]::WriteAllText($cryptoKey, $hex)
        $script:CERTS_GENERATED = $true
    }
}

# Print a one-time notice when key material was generated this run.
function Show-CertificatesNotice {
    if (-not $CERTS_GENERATED) { return }
    Write-Host "Generated missing security material in config/certs."
    Write-Host "  Preserve this directory; if these keys are lost or changed, previously issued tokens and encrypted data can no longer be validated or decrypted."
    Write-Host ""
}

# ============================================================================
# Prompt for Admin Credentials (interactive mode only)
# ============================================================================

# Prompt for any credential not supplied via CLI flags or environment variables, but only
# when stdin is a terminal.
if (([Console]::In -and -not [Console]::IsInputRedirected) -and (-not $ADMIN_USERNAME_PROVIDED -or -not $ADMIN_PASSWORD_PROVIDED)) {
    Write-Host ""
    Write-Host "Configure the default admin user (press Enter to accept defaults):"
    Write-Host ""
    if (-not $ADMIN_USERNAME_PROVIDED) {
        $inputUsername = Read-Host "  Admin username [admin]"
        $ADMIN_USERNAME = if ($inputUsername) { $inputUsername } else { "admin" }
    }
    if (-not $ADMIN_PASSWORD_PROVIDED) {
        # Generate the password up front so it can be shown as the prompt default (the value
        # used if the operator presses Enter). A typed value overrides it.
        Set-AdminPassword
        $inputPassword = Read-Host "  Admin password [$ADMIN_PASSWORD]" -AsSecureString
        $plainInputPassword = [System.Runtime.InteropServices.Marshal]::PtrToStringAuto(
            [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($inputPassword)
        )
        if ($plainInputPassword) {
            $ADMIN_PASSWORD = $plainInputPassword
            $ADMIN_PASSWORD_GENERATED = $false
        }
    }
    Write-Host ""
}

# Read configuration
# Bootstrap needs the Control Plane's deployment.yaml, so a missing one stops setup before any
# credentials or key material are generated.
if (-not (Resolve-ConfigFile)) {
    Log-Error "deployment.yaml was not found next to this script"
    exit 1
}

Read-Config | Out-Null

# Configure the admin password and key material before bootstrap so both are ready to use.
Set-AdminPassword
Set-Certificates

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
    Write-Host ""
}
Log-Debug "Platform: $($PSVersionTable.Platform)"

# ============================================================================
# Create Default Resources (in-process bootstrap)
# ============================================================================

# Resolve the script directory (used to locate start-cp.ps1).
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# Create the default resources by delegating to start-cp.ps1 -Bootstrap. The public URL and admin
# credentials are exported so the bootstrap subcommand picks them up.
$env:PUBLIC_URL = $PUBLIC_URL
$env:ADMIN_USERNAME = $ADMIN_USERNAME
$env:ADMIN_PASSWORD = $ADMIN_PASSWORD

$startScript = Join-Path $scriptDir 'start-cp.ps1'
if (-not (Test-Path $startScript)) {
    Log-Error "start-cp.ps1 is missing: $startScript"
    exit 1
}

if ($VERBOSE_MODE) {
    Write-Host "[WAIT] Creating default resources..." -ForegroundColor Blue
}

try {
    & $startScript --bootstrap
    if ($LASTEXITCODE -ne 0) {
        Log-Error "Failed to create default resources"
        exit 1
    }
}
finally {
    # Env:ADMIN_USERNAME/ADMIN_PASSWORD were set above so the nested start-cp.ps1 call could read
    # them. This script runs in the caller's own pwsh.exe process (not a forked child), so without
    # this cleanup they'd leak into the interactive session and silently suppress the prompt on
    # the next ./setup-cp.ps1 run.
    Remove-Item Env:ADMIN_USERNAME -ErrorAction SilentlyContinue
    Remove-Item Env:ADMIN_PASSWORD -ErrorAction SilentlyContinue
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
    Show-AdminCredentialsNotice
    Show-CertificatesNotice
    Write-Host "Run .\start-cp.ps1 to start ${PRODUCT_NAME}."
    Write-Host ""
}
else {
    Write-Host "========================================="
    Write-Host "[OK] Setup completed successfully!" -ForegroundColor Green
    Write-Host "========================================="
    Write-Host ""
    Show-AdminCredentialsNotice
    Show-CertificatesNotice
    Write-Host "[INFO] Next steps:"
    Write-Host "   1. Start the Control Plane: .\start-cp.ps1" -ForegroundColor Cyan
    Write-Host "   2. Access $PRODUCT_NAME at: $BASE_URL" -ForegroundColor Cyan
    Write-Host ""
}

exit 0
