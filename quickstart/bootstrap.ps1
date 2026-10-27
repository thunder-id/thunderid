# Copyright 2025-2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

# Dot-sourced by setup.ps1 and start.ps1. Holds what the quickstart pack's bootstrap resources (the
# *.yaml files under quickstart/bootstrap/, layered onto each plane by build.ps1's
# Package-Quickstart) need from the environment, and the helpers both scripts share, so neither
# carries them. The PowerShell counterpart of bootstrap.sh.

$AUTH_DP_PORT = 8091
$DP_PORT = 8090
$CP_PORT = 8095

# New-GatewayKey generates the management key the managed Data Plane trusts and the Control Plane's
# gateway resource (quickstart/bootstrap/cp/00-quickstart-gateway.yaml) registers it with, so no key
# is committed to the repository. It sets $QUICKSTART_GATEWAY_KEY and its SHA-256 hex digest
# $QUICKSTART_GATEWAY_KEY_HASH, and exports the key for the bootstrap template.
function New-GatewayKey {
    $bytes = New-Object 'System.Byte[]' 32
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    $script:QUICKSTART_GATEWAY_KEY = -join ($bytes | ForEach-Object { $_.ToString('x2') })
    $digest = [System.Security.Cryptography.SHA256]::Create().ComputeHash(
        [System.Text.Encoding]::UTF8.GetBytes($script:QUICKSTART_GATEWAY_KEY))
    $script:QUICKSTART_GATEWAY_KEY_HASH = -join ($digest | ForEach-Object { $_.ToString('x2') })
    $env:QUICKSTART_GATEWAY_KEY = $script:QUICKSTART_GATEWAY_KEY
}

# Set-GatewayKeyTrust makes the managed Data Plane trust the generated key, by writing its digest
# into dp/deployment.yaml as server.security.management_api_key_hash right after the "  security:"
# line. A digest from an earlier setup is replaced, so running setup again re-keys the link.
function Set-GatewayKeyTrust {
    $path = (Resolve-Path -LiteralPath (Join-Path "dp" "deployment.yaml")).Path
    $lines = ([System.IO.File]::ReadAllText($path) -replace "`r`n", "`n") -split "`n"
    $out = foreach ($line in $lines) {
        if ($line -match '^    management_api_key_hash:') { continue }
        $line
        if ($line -match '^  security:') { "    management_api_key_hash: `"$script:QUICKSTART_GATEWAY_KEY_HASH`"" }
    }
    [System.IO.File]::WriteAllText($path, ($out -join "`n"), (New-Object System.Text.UTF8Encoding($false)))
    if (-not ([System.IO.File]::ReadAllText($path)).Contains("management_api_key_hash: `"$script:QUICKSTART_GATEWAY_KEY_HASH`"")) {
        throw "Could not set the Data Plane's management key in dp/deployment.yaml."
    }
}

# Get-UserPasswordSecretName returns the name a user's password is held under in a gateway's
# secret store, which the Control Plane's export derives from the username the same way:
# USER_<USERNAME>_PASSWORD, with a word boundary wherever the case changes (ServiceDesk becomes
# SERVICE_DESK) and every character that is not a letter or digit as an underscore. See
# DeriveVariableName in backend/internal/system/varname.
function Get-UserPasswordSecretName {
    param([string]$Username)

    $out = ""
    $n = $Username.Length
    for ($i = 0; $i -lt $n; $i++) {
        $c = $Username[$i]
        if ($i -gt 0 -and $c -cmatch '[A-Z]') {
            $p = $Username[$i - 1]
            if ($p -cmatch '[a-z]') {
                $out += "_"
            }
            elseif ($p -ne '_' -and ($i + 1) -lt $n -and $Username[$i + 1] -cmatch '[a-z]') {
                $out += "_"
            }
        }
        $out += $c
    }
    $name = "USER_$($out.ToUpper())_PASSWORD"
    $name = $name -replace '[^A-Za-z0-9]', '_'
    $name = $name -replace '__+', '_'
    return $name.Trim('_')
}

# Set-ConsoleUrls exports the console redirect URIs the bootstrap resources are templated with:
# CP_CONSOLE_URL for the CP_CONSOLE application on auth-dp, and ADMIN_CONSOLE_URL for the
# ADMIN_CONSOLE application on the Data Plane and the Control Plane. Call it before the first
# plane's setup runs, since auth-dp's bootstrap already needs CP_CONSOLE_URL.
function Set-ConsoleUrls {
    $env:CP_CONSOLE_URL = "https://localhost:$CP_PORT/console"
    $env:ADMIN_CONSOLE_URL = "https://localhost:$DP_PORT/console"
}

# Set-DpCaCertificate exports DP_CA_CERTIFICATE, the managed Data Plane's certificate, which the
# Control Plane's gateway resource trusts it by. Real newlines are escaped to a literal
# two-character "\n" so the multi-line PEM survives as one line of bootstrap-template text; the
# YAML parser turns them back into real newlines once the file is templated and decoded (a plain
# double-quoted YAML scalar interprets \n as a line break). Needs dp/config/certs/server.cert, so
# call it once the key material has been shared.
function Set-DpCaCertificate {
    $dpCert = (Get-Content -Raw -LiteralPath (Join-Path "dp" "config/certs/server.cert")).TrimEnd()
    $env:DP_CA_CERTIFICATE = $dpCert -replace "`r?`n", '\n'
}

# Invoke-DpPut writes one value on the managed Data Plane's own stores with its management key.
# A failure throws, since a value that was not seeded makes every later auto-apply refuse.
function Invoke-DpPut {
    param([string]$Path, [string]$Value)

    $body = @{ value = $Value } | ConvertTo-Json -Compress
    try {
        Invoke-WebRequest -Method Put -Uri "https://localhost:$DP_PORT$Path" -SkipCertificateCheck `
            -Headers @{ "API-Key" = $QUICKSTART_GATEWAY_KEY } -ContentType "application/json" -Body $body `
            -ErrorAction Stop | Out-Null
    }
    catch {
        throw "Failed to seed $Path on the Data Plane: $($_.Exception.Message)"
    }
}

# Set-DpValues sets the values auto-apply needs on the managed Data Plane before any write has
# captured them.
#
# Auto-apply (and a manual Capture/Apply, for that matter) refuses every sweep with "the gateway
# does not hold every value this version refers to" until each value a version names exists on the
# gateway. Two come from the Control Plane's own bootstrap, which runs while the Data Plane is not
# up to capture them to:
#
#   - the ClientID and redirect URIs of the ADMIN_CONSOLE application
#     (quickstart/bootstrap/dp-cp/04-quickstart-admin-console.yaml). The redirect URIs are a list,
#     so the variable holds a JSON array;
#   - the identifier of the System resource server, which a gateway holds for itself and is
#     the Data Plane's own MCP URL;
#   - the Service Desk Admin's password. A version carries that user to the Data Plane like any
#     other, and the Data Plane fills its password in from this secret.
#
# All are set directly on the gateway's own stores. Needs SERVICE_DESK_USERNAME and
# SERVICE_DESK_PASSWORD to be set.
function Set-DpValues {
    $ready = $false
    for ($i = 0; $i -lt 60; $i++) {
        try {
            Invoke-WebRequest -Uri "https://localhost:$DP_PORT/health/readiness" -SkipCertificateCheck `
                -ErrorAction Stop | Out-Null
            $ready = $true
            break
        }
        catch { Start-Sleep -Milliseconds 500 }
    }
    if (-not $ready) { throw "The Data Plane did not become ready, so its bootstrap values were not seeded." }
    Invoke-DpPut -Path "/variables/APPLICATION_ADMIN_CONSOLE_CLIENT_ID" -Value "ADMIN_CONSOLE"
    Invoke-DpPut -Path "/variables/APPLICATION_ADMIN_CONSOLE_REDIRECT_URIS" `
        -Value (ConvertTo-Json -Compress -InputObject @($env:ADMIN_CONSOLE_URL))
    Invoke-DpPut -Path "/variables/RESOURCE_SERVER_SYSTEM_IDENTIFIER" -Value "https://localhost:$DP_PORT/mcp"
    Invoke-DpPut -Path "/secrets/$(Get-UserPasswordSecretName $env:SERVICE_DESK_USERNAME)" `
        -Value $env:SERVICE_DESK_PASSWORD
}

# Stop-PortListener kills whatever is bound to the port, not the PowerShell process that launched
# it: start.ps1 starts the actual server binary itself and waits on it, so stopping only the outer
# process would orphan that binary, which keeps holding the port. Killing the listener is correct
# regardless of how many process layers are in between.
function Stop-PortListener {
    param([int]$Port)

    try {
        $pids = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction Stop |
            Select-Object -ExpandProperty OwningProcess -Unique
    }
    catch {
        $pids = @()
        $netstat = & netstat -ano 2>$null | Select-String ":$Port\s.*LISTENING"
        foreach ($line in $netstat) {
            $procId = (($line -split '\s+') | Where-Object { $_ -ne '' })[-1]
            if ([int]::TryParse($procId, [ref]$null)) { $pids += [int]$procId }
        }
    }

    foreach ($procId in ($pids | Where-Object { $_ -and ($_ -ne 0) } | Select-Object -Unique)) {
        Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
    }
}

# Start-Plane starts one plane's start script in the background, from that plane's own directory,
# and returns the process.
function Start-Plane {
    param([string]$Directory, [string]$Script)

    $shell = (Get-Process -Id $PID).Path
    return Start-Process -FilePath $shell -ArgumentList @("-NoProfile", "-File", $Script) `
        -WorkingDirectory (Join-Path $PSScriptRoot $Directory) -NoNewWindow -PassThru
}
