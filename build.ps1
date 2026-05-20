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


[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$Command,
    
    [Parameter(Position = 1)]
    [string]$GO_OS,
    
    [Parameter(Position = 2)]
    [string]$GO_ARCH,
    
    [Parameter(Position = 3)]
    [string]$TestRun,
    
    [Parameter(Position = 4)]
    [string]$TestPackage,

    [switch]$WithoutConsent
)

# Accept --without-consent anywhere in positional arguments.
$positionalArgs = @($Command, $GO_OS, $GO_ARCH, $TestRun, $TestPackage)
$withoutConsentFromArgs = $false

for ($i = 0; $i -lt $positionalArgs.Count; $i++) {
    if ($positionalArgs[$i] -ceq "--without-consent") {
        $withoutConsentFromArgs = $true
        $positionalArgs[$i] = $null
    }
}

$Command = $positionalArgs[0]
$GO_OS = $positionalArgs[1]
$GO_ARCH = $positionalArgs[2]
$TestRun = $positionalArgs[3]
$TestPackage = $positionalArgs[4]

$skipConsent = $WithoutConsent.IsPresent -or $withoutConsentFromArgs -or ($env:WITHOUT_CONSENT -eq "true")

$PRODUCT_NAME = "ThunderID"
$PRODUCT_NAME_LOWERCASE = $PRODUCT_NAME.ToLower()

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

$ErrorActionPreference = "Stop"

$SCRIPT_DIR = $PSScriptRoot

# Script-level variables for process management
$script:BACKEND_PID = $null
$script:FRONTEND_PID = $null

# --- Set Default OS and the architecture --- 
# Auto-detect GO OS
if ([string]::IsNullOrEmpty($GO_OS)) {
    try {
        $DEFAULT_OS = & go env GOOS
        if ([string]::IsNullOrEmpty($DEFAULT_OS)) {
            throw "Go environment not found"
        }
    }
    catch {
        $DEFAULT_OS = "windows"
    }
    $GO_OS = $DEFAULT_OS
}

# Auto-detect GO ARCH
if ([string]::IsNullOrEmpty($GO_ARCH)) {
    try {
        $DEFAULT_ARCH = & go env GOARCH
        if ([string]::IsNullOrEmpty($DEFAULT_ARCH)) {
            throw "Go environment not found"
        }
    }
    catch {
        # Use PowerShell to detect architecture
        if ([Environment]::Is64BitOperatingSystem) {
            $DEFAULT_ARCH = "amd64"
        }
        else {
            throw "Unsupported architecture"
        }
    }
    $GO_ARCH = $DEFAULT_ARCH
}

Write-Host "Using GO OS: $GO_OS and ARCH: $GO_ARCH"

$SAMPLE_DIST_NODE_VERSION = "node18"
$SAMPLE_DIST_OS = $GO_OS
$SAMPLE_DIST_ARCH = $GO_ARCH

# Transform OS for node packaging executor
if ($SAMPLE_DIST_OS -eq "darwin") {
    $SAMPLE_DIST_OS = "macos"
}
elseif ($SAMPLE_DIST_OS -eq "windows") {
    $SAMPLE_DIST_OS = "win"
}

if ($SAMPLE_DIST_ARCH -eq "amd64") {
    $SAMPLE_DIST_ARCH = "x64"
}

# --- Package Distribution details ---
$GO_PACKAGE_OS = $GO_OS
$GO_PACKAGE_ARCH = $GO_ARCH

# Normalize OS name for distribution packaging
if ($GO_OS -eq "darwin") {
    $GO_PACKAGE_OS = "macos"
}
elseif ($GO_OS -eq "windows") {
    $GO_PACKAGE_OS = "win"
}

if ($GO_ARCH -eq "amd64") {
    $GO_PACKAGE_ARCH = "x64"
}

$VERSION_FILE = "version.txt"
$VERSION = Get-Content $VERSION_FILE -Raw
$VERSION = $VERSION.Trim()
$PRODUCT_VERSION = $VERSION
if ($PRODUCT_VERSION.StartsWith("v")) {
    $PRODUCT_VERSION = $PRODUCT_VERSION.Substring(1)
}
$BINARY_NAME = $PRODUCT_NAME_LOWERCASE
$PRODUCT_FOLDER = "${BINARY_NAME}-${PRODUCT_VERSION}-${GO_PACKAGE_OS}-${GO_PACKAGE_ARCH}"

# --- Sample App Distribution details ---
$SAMPLE_PACKAGE_OS = $SAMPLE_DIST_OS
$SAMPLE_PACKAGE_ARCH = $SAMPLE_DIST_ARCH

# React Vanilla Sample
$VANILLA_SAMPLE_APP_SERVER_BINARY_NAME = "server"
$vanillaPackageJson = Get-Content "samples/apps/react-vanilla-sample/package.json" -Raw | ConvertFrom-Json
$VANILLA_SAMPLE_APP_VERSION = $vanillaPackageJson.version
$VANILLA_SAMPLE_APP_FOLDER = "sample-app-react-vanilla-${VANILLA_SAMPLE_APP_VERSION}-${SAMPLE_PACKAGE_OS}-${SAMPLE_PACKAGE_ARCH}"

# React SDK Sample
$reactSdkPackageJson = Get-Content "samples/apps/react-sdk-sample/package.json" -Raw | ConvertFrom-Json
$REACT_SDK_SAMPLE_APP_VERSION = $reactSdkPackageJson.version
$REACT_SDK_SAMPLE_APP_FOLDER = "sample-app-react-sdk-${REACT_SDK_SAMPLE_APP_VERSION}-${SAMPLE_PACKAGE_OS}-${SAMPLE_PACKAGE_ARCH}"

# React API-based Sample
$reactApiPackageJson = Get-Content "samples/apps/react-api-based-sample/package.json" -Raw | ConvertFrom-Json
$REACT_API_SAMPLE_APP_VERSION = $reactApiPackageJson.version
$REACT_API_SAMPLE_APP_FOLDER = "sample-app-react-api-based-${REACT_API_SAMPLE_APP_VERSION}-${SAMPLE_PACKAGE_OS}-${SAMPLE_PACKAGE_ARCH}"

# Directories
$TARGET_DIR = Join-Path $SCRIPT_DIR "target"
$OUTPUT_DIR = Join-Path $TARGET_DIR "out"
$DIST_DIR = Join-Path $TARGET_DIR "dist"
$BUILD_DIR = Join-Path $OUTPUT_DIR ".build"
$LOCAL_CERT_DIR = Join-Path $OUTPUT_DIR ".cert"
$BACKEND_BASE_DIR = "backend"
$BACKEND_DIR = Join-Path $BACKEND_BASE_DIR "cmd/server"
$REPOSITORY_DIR = Join-Path $BACKEND_BASE_DIR "cmd/server/repository"
$REPOSITORY_DB_DIR = Join-Path $REPOSITORY_DIR "database"
$SERVER_SCRIPTS_DIR = Join-Path $BACKEND_BASE_DIR "scripts"
$SERVER_DB_SCRIPTS_DIR = Join-Path $BACKEND_BASE_DIR "dbscripts"
$SECURITY_DIR = "repository/resources/security"
$FRONTEND_BASE_DIR = "frontend"
$GATE_APP_DIST_DIR = "apps/gate"
$CONSOLE_APP_DIST_DIR = "apps/console"
$FRONTEND_GATE_APP_SOURCE_DIR = Join-Path $FRONTEND_BASE_DIR "apps/gate"
$FRONTEND_CONSOLE_APP_SOURCE_DIR = Join-Path $FRONTEND_BASE_DIR "apps/console"
$SAMPLE_BASE_DIR = "samples"
$VANILLA_SAMPLE_APP_DIR = Join-Path $SAMPLE_BASE_DIR "apps/react-vanilla-sample"
$VANILLA_SAMPLE_APP_SERVER_DIR = Join-Path $VANILLA_SAMPLE_APP_DIR "server"
$REACT_SDK_SAMPLE_APP_DIR = Join-Path $SAMPLE_BASE_DIR "apps/react-sdk-sample"
$REACT_API_SAMPLE_APP_DIR = Join-Path $SAMPLE_BASE_DIR "apps/react-api-based-sample"

# Default ports
$GATE_APP_DEFAULT_PORT = 5190
$CONSOLE_APP_DEFAULT_PORT = 5191
$DOCS_DEFAULT_PORT = 3000

# ============================================================================
# Read Configuration from deployment.yaml
# ============================================================================

$CONFIG_FILE = "./backend/cmd/server/repository/conf/deployment.yaml"

# Function to read config with fallback
function Read-Config {
    if (-not (Test-Path $CONFIG_FILE)) {
        # Use defaults if config file not found
        $script:HOSTNAME = "localhost"
        $script:PORT = 8090
        $script:HTTP_ONLY = "false"
        $script:PUBLIC_HOSTNAME = ""
        $script:CONSENT_ENABLED = $true
    }
    else {
        # Try yq first (YAML parser)
        if (Get-Command yq -ErrorAction SilentlyContinue) {
            $script:HOSTNAME = & yq eval '.server.hostname // "localhost"' $CONFIG_FILE 2>$null
            $script:PORT = & yq eval '.server.port // 8090' $CONFIG_FILE 2>$null
            $script:HTTP_ONLY = & yq eval '.server.http_only // false' $CONFIG_FILE 2>$null
            $script:PUBLIC_HOSTNAME = & yq eval '.server.public_hostname // ""' $CONFIG_FILE 2>$null
            $consentEnabled = & yq eval '.consent.enabled // true' $CONFIG_FILE 2>$null
            $script:CONSENT_ENABLED = ($consentEnabled -eq "true")
            $script:SYSTEM_RS_HANDLE = & yq eval '.resource.system_resource_server.handle // ""' $CONFIG_FILE 2>$null
            $script:SYSTEM_RS_IDENTIFIER = & yq eval '.resource.system_resource_server.identifier // ""' $CONFIG_FILE 2>$null
        }
        else {
            # Fallback: basic parsing with regex
            $content = Get-Content $CONFIG_FILE -Raw
            
            # Try to extract hostname
            if ($content -match 'hostname:\s*["'']?([^"''\n]+)["'']?') {
                $script:HOSTNAME = $matches[1].Trim()
            }
            else {
                $script:HOSTNAME = "localhost"
            }
            
            # Try to extract port
            if ($content -match 'port:\s*(\d+)') {
                $script:PORT = [int]$matches[1]
            }
            else {
                $script:PORT = 8090
            }
            
            # Try to extract http_only
            if ($content -match 'http_only:\s*true') {
                $script:HTTP_ONLY = "true"
            }
            else {
                $script:HTTP_ONLY = "false"
            }
            
            # Try to extract public_hostname
            if ($content -match 'public_hostname:\s*["'']?([^"''\n]+)["'']?') {
                $script:PUBLIC_HOSTNAME = $matches[1].Trim()
            }
            else {
                $script:PUBLIC_HOSTNAME = ""
            }

            # Try to extract consent.enabled
            if ($content -match 'consent:[\s\S]*?enabled:\s*(true|false)') {
                $script:CONSENT_ENABLED = ($matches[1] -eq "true")
            }
            else {
                $script:CONSENT_ENABLED = $true
            }

            $uncommentedContent = ($content -split "`n" | Where-Object { $_ -notmatch '^\s*#' }) -join "`n"
            # Try to extract system resource server handle
            if ($uncommentedContent -match '(?ms)system_resource_server:.*?handle:\s*[''"]([^''"]*)[''"]') {
                $script:SYSTEM_RS_HANDLE = $matches[1]
            }
            elseif ($uncommentedContent -match '(?ms)system_resource_server:.*?handle:\s*([^\s#]+)') {
                $script:SYSTEM_RS_HANDLE = $matches[1]
            }
            else {
                $script:SYSTEM_RS_HANDLE = ""
            }

            # Try to extract system resource server identifier
            if ($uncommentedContent -match '(?ms)system_resource_server:.*?identifier:\s*[''"]([^''"]*)[''"]') {
                $script:SYSTEM_RS_IDENTIFIER = $matches[1]
            }
            elseif ($uncommentedContent -match '(?ms)system_resource_server:.*?identifier:\s*([^\s#]+)') {
                $script:SYSTEM_RS_IDENTIFIER = $matches[1]
            }
            else {
                $script:SYSTEM_RS_IDENTIFIER = ""
            }
        }
    }
    
    # Determine protocol
    if ($script:HTTP_ONLY -eq "true") {
        $script:PROTOCOL = "http"
    }
    else {
        $script:PROTOCOL = "https"
    }
}

# Read configuration
Read-Config

# Construct base URL (internal API endpoint)
$BASE_URL = "${PROTOCOL}://${HOSTNAME}:${PORT}"

# Construct public URL (external/redirect URLs)
if ($PUBLIC_HOSTNAME) {
    $PUBLIC_URL = $PUBLIC_HOSTNAME
}
else {
    $PUBLIC_URL = $BASE_URL
}

function Get-CoverageExclusionPattern {
    # Read exclusion patterns (full package paths) from .excludecoverage file
    # This function can be called from any directory
    
    $coverage_exclude_file = $null
    
    # Check if we're already in the backend directory or need to use relative path
    if (Test-Path ".excludecoverage") {
        $coverage_exclude_file = ".excludecoverage"
    }
    elseif (Test-Path (Join-Path $SCRIPT_DIR $BACKEND_BASE_DIR ".excludecoverage")) {
        $coverage_exclude_file = Join-Path $SCRIPT_DIR $BACKEND_BASE_DIR ".excludecoverage"
    }
    else {
        return ""
    }
    
    # Read non-comment, non-empty lines and join with '|' for regex (exact package path matching)
    $patterns = Get-Content $coverage_exclude_file | Where-Object { 
        $_ -notmatch '^\s*#' -and $_ -notmatch '^\s*$' 
    }
    
    if ($patterns) {
        return ($patterns -join '|')
    }
    
    return ""
}

function Clean {
    Write-Host "================================================================"
    Write-Host "Cleaning build artifacts..."
    if (Test-Path $TARGET_DIR) {
        Remove-Item -Path $TARGET_DIR -Recurse -Force -ErrorAction SilentlyContinue
    }

    Write-Host "Removing certificates in $BACKEND_DIR/$SECURITY_DIR"
    if (Test-Path (Join-Path $BACKEND_DIR $SECURITY_DIR)) {
        Remove-Item -Path (Join-Path $BACKEND_DIR $SECURITY_DIR) -Recurse -Force -ErrorAction SilentlyContinue
    }

    Write-Host "Removing certificates in $VANILLA_SAMPLE_APP_DIR"
    Remove-Item -Path (Join-Path $VANILLA_SAMPLE_APP_DIR "server.cert") -Force -ErrorAction SilentlyContinue
    Remove-Item -Path (Join-Path $VANILLA_SAMPLE_APP_DIR "server.key") -Force -ErrorAction SilentlyContinue

    Write-Host "Removing certificates in $VANILLA_SAMPLE_APP_SERVER_DIR"
    Remove-Item -Path (Join-Path $VANILLA_SAMPLE_APP_SERVER_DIR "server.cert") -Force -ErrorAction SilentlyContinue
    Remove-Item -Path (Join-Path $VANILLA_SAMPLE_APP_SERVER_DIR "server.key") -Force -ErrorAction SilentlyContinue

    Write-Host "Removing certificates in $REACT_SDK_SAMPLE_APP_DIR"
    Remove-Item -Path (Join-Path $REACT_SDK_SAMPLE_APP_DIR "server.cert") -Force -ErrorAction SilentlyContinue
    Remove-Item -Path (Join-Path $REACT_SDK_SAMPLE_APP_DIR "server.key") -Force -ErrorAction SilentlyContinue

    Write-Host "Removing certificates in $REACT_API_SAMPLE_APP_DIR"
    Remove-Item -Path (Join-Path $REACT_API_SAMPLE_APP_DIR "server.cert") -Force -ErrorAction SilentlyContinue
    Remove-Item -Path (Join-Path $REACT_API_SAMPLE_APP_DIR "server.key") -Force -ErrorAction SilentlyContinue
    Write-Host "================================================================"
}

function Build-Backend {
    Write-Host "================================================================"
    Write-Host "Building Go backend..."
    New-Item -Path $BUILD_DIR -ItemType Directory -Force | Out-Null

    # Set binary name with .exe extension for Windows
    $output_binary = $BINARY_NAME
    if ($GO_OS -eq "windows") {
        $output_binary = "${BINARY_NAME}.exe"
    }

    # Prepare build date without spaces to avoid ldflags splitting
    $buildDate = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

    $env:GOOS = $GO_OS
    $env:GOARCH = $GO_ARCH
    $env:CGO_ENABLED = "0"

    # Check if coverage build is requested via ENABLE_COVERAGE environment variable
    $buildArgs = @('build', '-x')
    if ($env:ENABLE_COVERAGE -eq "true") {
        Write-Host "Building with coverage instrumentation enabled..."
        
        # Build coverage package list, excluding patterns from .excludecoverage
        Push-Location $BACKEND_BASE_DIR
        try {
            $exclude_pattern = Get-CoverageExclusionPattern
            $coverpkg = ""
            
            if ($exclude_pattern) {
                Write-Host "Excluding coverage for patterns: $exclude_pattern"
                $packages = & go list ./...
                $filtered_packages = $packages | Where-Object { $_ -notmatch $exclude_pattern }
                $coverpkg = $filtered_packages -join ','
            }
            else {
                $packages = & go list ./...
                $coverpkg = $packages -join ','
            }
        }
        finally {
            Pop-Location
        }
        
        $buildArgs += @('-cover', "-coverpkg=$coverpkg")
    }

    # Construct ldflags safely and pass as an argument array to avoid PowerShell splitting
    $ldflags = "-X main.version=$VERSION -X main.buildDate=$buildDate"
    $outputPath = Join-Path $BUILD_DIR $output_binary
    $buildArgs += @('-ldflags', $ldflags, '-o', $outputPath, './cmd/server')

    Write-Host "Executing: go $($buildArgs -join ' ')"

    Push-Location $BACKEND_BASE_DIR
    try {
        & go @buildArgs
        if ($LASTEXITCODE -ne 0) {
            throw "Go build failed with exit code $LASTEXITCODE"
        }
    }
    finally {
        Pop-Location
    }

    Write-Host "Initializing databases..."
    Initialize-Databases -override $true
    Write-Host "================================================================"
}

function Ensure-Pnpm {
    if (-not (Get-Command pnpm -ErrorAction SilentlyContinue)) {
        Write-Host "pnpm not found, installing..."
        & npm install -g pnpm
    }
}

function Build-Frontend {
    Write-Host "================================================================"
    Write-Host "Building frontend apps..."
    Ensure-Pnpm
    
    # Install dependencies
    try {
        Write-Host "Installing frontend dependencies..."
        & pnpm install --frozen-lockfile
        
        Write-Host "Building frontend applications & packages..."
        & pnpm build:frontend
    }
    finally {
        Pop-Location
    }
    
    Write-Host "================================================================"
}

function Build-Docs {
    Write-Host "================================================================"
    Write-Host "Building documentation..."
    Ensure-Pnpm
    
    try {
        Write-Host "Installing frontend dependencies (required for docs build)..."
        & pnpm install --frozen-lockfile
        
        Write-Host "Building documentation..."
        & pnpm run build:docs
    }
    finally {
        Pop-Location
    }
    
    Write-Host "================================================================"
}

function Build-Sdks-Js {
    Ensure-Pnpm
    
    Write-Host "Installing SDK dependencies..."
    & pnpm install --frozen-lockfile
    
    Write-Host "Building JavaScript ecosystem SDK packages..."
    & pnpm --filter './sdks/**' build
}

function Test-Sdks-Js {
    Ensure-Pnpm
    
    Write-Host "Installing SDK dependencies..."
    & pnpm install --frozen-lockfile
    
    Write-Host "Running JavaScript ecosystem SDK tests..."
    & pnpm --filter './sdks/**' test
}

function Lint-Sdks-Js {
    Ensure-Pnpm
    
    Write-Host "Installing SDK dependencies..."
    & pnpm install --frozen-lockfile
    
    Write-Host "Linting JavaScript ecosystem SDK packages..."
    & pnpm --filter './sdks/**' lint
}

function Build-Sdks {
    Write-Host "================================================================"
    Write-Host "Building SDKs..."
    Build-Sdks-Js
    Write-Host "================================================================"
}

function Test-Sdks {
    Write-Host "================================================================"
    Write-Host "Running SDK tests..."
    Test-Sdks-Js
    Write-Host "================================================================"
}

function Lint-Sdks {
    Write-Host "================================================================"
    Write-Host "Linting SDKs..."
    Lint-Sdks-Js
    Write-Host "================================================================"
}

function Initialize-Databases {
    param(
        [bool]$override = $false
    )
    
    Write-Host "================================================================"
    Write-Host "Initializing SQLite databases..."

    # Check for sqlite3 CLI availability
    $sqliteCmd = Get-Command sqlite3 -ErrorAction SilentlyContinue
    if (-not $sqliteCmd) {
        Write-Host ""
        Write-Host "ERROR: 'sqlite3' CLI not found on PATH. The build script uses the sqlite3 command to initialize local SQLite databases."
        Write-Host "On Windows you can install sqlite3 using one of the following methods:"
        Write-Host "  1) Chocolatey (requires admin PowerShell):"
        Write-Host "       choco install sqlite" 
        Write-Host "  2) Scoop (recommended for user installs):"
        Write-Host "       scoop install sqlite" 
        Write-Host "  3) Download prebuilt binaries from https://www.sqlite.org/download.html and add the folder to your PATH."
        Write-Host ""
        Write-Host "Alternatively, skip database initialization and create the DB files manually under '$REPOSITORY_DB_DIR'."
        throw "sqlite3 CLI not found. Install sqlite3 and re-run the build."
    }

    New-Item -Path $REPOSITORY_DB_DIR -ItemType Directory -Force | Out-Null

    $db_files = @("configdb.db", "runtimedb.db", "userdb.db")
    $script_paths = @("configdb/sqlite.sql", "runtimedb/sqlite.sql", "userdb/sqlite.sql")

    for ($i = 0; $i -lt $db_files.Length; $i++) {
        $db_file = $db_files[$i]
        $script_rel_path = $script_paths[$i]
        $db_path = Join-Path $REPOSITORY_DB_DIR $db_file
        $script_path = Join-Path $SERVER_DB_SCRIPTS_DIR $script_rel_path

        if (Test-Path $script_path) {
            if (Test-Path $db_path) {
                if ($override) {
                    Write-Host " - Removing existing $db_file as override is true"
                    Remove-Item $db_path -Force
                }
                else {
                    Write-Host " ! Skipping $db_file : DB already exists. Delete the existing and re-run to recreate."
                    continue
                }
            }

            Write-Host " - Creating $db_file using $script_path"
            # Use sqlite3 command line tool
            & sqlite3 $db_path ".read $script_path"
            if ($LASTEXITCODE -ne 0) {
                throw "SQLite operation failed with exit code $LASTEXITCODE"
            }
            Write-Host " - Enabling WAL mode for $db_file"
            & sqlite3 $db_path "PRAGMA journal_mode=WAL;"
            if ($LASTEXITCODE -ne 0) {
                throw "Failed to enable WAL mode with exit code $LASTEXITCODE"
            }
        }
        else {
            Write-Host " ! Skipping $db_file : SQL script not found at $script_path"
        }
    }

    Write-Host "SQLite database initialization complete."
    Write-Host "================================================================"
}

function Prepare-Backend-For-Packaging {
    Write-Host "================================================================"
    Write-Host "Copying backend artifacts..."

    # Use appropriate binary name based on OS
    $binary_name = $BINARY_NAME
    if ($GO_OS -eq "windows") {
        $binary_name = "${BINARY_NAME}.exe"
    }

    $package_folder = Join-Path $DIST_DIR $PRODUCT_FOLDER
    Copy-Item -Path (Join-Path $BUILD_DIR $binary_name) -Destination $package_folder -Force
    Copy-Item -Path $REPOSITORY_DIR -Destination $package_folder -Recurse -Force
    Copy-Item -Path $VERSION_FILE -Destination $package_folder -Force
    Copy-Item -Path $SERVER_SCRIPTS_DIR -Destination $package_folder -Recurse -Force
    Copy-Item -Path $SERVER_DB_SCRIPTS_DIR -Destination $package_folder -Recurse -Force
    
    $security_dir = Join-Path $package_folder $SECURITY_DIR
    New-Item -Path $security_dir -ItemType Directory -Force | Out-Null

    # Copy bootstrap directory
    Write-Host "Copying bootstrap scripts..."
    Copy-Item -Path (Join-Path $BACKEND_DIR "bootstrap") -Destination $package_folder -Recurse -Force

    Write-Host "=== Ensuring server certificates exist in the distribution ==="
    Ensure-Certificates -cert_dir $security_dir -cert_name_prefix "server"
    Ensure-Certificates -cert_dir $security_dir -cert_name_prefix "signing"
    Write-Host "================================================================"

    Write-Host "=== Ensuring crypto file exists in the distribution ==="
    Ensure-Crypto-File -conf_dir (Join-Path $package_folder "repository/conf")
    Write-Host "================================================================"
}

function Prepare-Frontend-For-Packaging {
    Write-Host "================================================================"
    Write-Host "Copying frontend artifacts..."

    $package_folder = Join-Path $DIST_DIR $PRODUCT_FOLDER
    New-Item -Path (Join-Path $package_folder $GATE_APP_DIST_DIR) -ItemType Directory -Force | Out-Null
    New-Item -Path (Join-Path $package_folder $CONSOLE_APP_DIST_DIR) -ItemType Directory -Force | Out-Null

    # Copy gate app build output
    if (Test-Path (Join-Path $FRONTEND_GATE_APP_SOURCE_DIR "dist")) {
        Write-Host "Copying Gate app build output..."
        Copy-Item -Path (Join-Path $FRONTEND_GATE_APP_SOURCE_DIR "dist\*") -Destination (Join-Path $package_folder $GATE_APP_DIST_DIR) -Recurse -Force
    }
    else {
        Write-Host "Warning: Gate app build output not found at $((Join-Path $FRONTEND_GATE_APP_SOURCE_DIR "dist"))"
    }
    
    # Copy console app build output
    if (Test-Path (Join-Path $FRONTEND_CONSOLE_APP_SOURCE_DIR "dist")) {
        Write-Host "Copying Console app build output..."
        Copy-Item -Path (Join-Path $FRONTEND_CONSOLE_APP_SOURCE_DIR "dist\*") -Destination (Join-Path $package_folder $CONSOLE_APP_DIST_DIR) -Recurse -Force
    }
    else {
        Write-Host "Warning: Console app build output not found at $((Join-Path $FRONTEND_CONSOLE_APP_SOURCE_DIR "dist"))"
    }

    Write-Host "================================================================"
}

function Package {
    Write-Host "================================================================"
    Write-Host "Packaging backend & frontend artifacts..."

    $package_folder = Join-Path $DIST_DIR $PRODUCT_FOLDER
    New-Item -Path $package_folder -ItemType Directory -Force | Out-Null

    Prepare-Frontend-For-Packaging
    Prepare-Backend-For-Packaging

    # Copy the appropriate startup and setup scripts based on the target OS
    if ($GO_OS -eq "windows") {
        Write-Host "Including Windows scripts (start.ps1, setup.ps1)..."
        Copy-Item -Path "start.ps1" -Destination $package_folder -Force
        Copy-Item -Path "setup.ps1" -Destination $package_folder -Force
    }
    else {
        Write-Host "Including Unix scripts (start.sh, setup.sh)..."
        Copy-Item -Path "start.sh" -Destination $package_folder -Force
        Copy-Item -Path "setup.sh" -Destination $package_folder -Force
    }

    if (-not $skipConsent) {
        Write-Host "Packaging consent server..."
        $packageFolderAbs = (Resolve-Path -Path $package_folder).Path
        & (Join-Path $SCRIPT_DIR "scripts/package-consent-server.ps1") `
            -GoOS $GO_OS -GoArch $GO_ARCH -DistOutputPath $packageFolderAbs
        if ($LASTEXITCODE -ne 0) {
            throw "Consent server packaging failed with exit code $LASTEXITCODE"
        }
    } else {
        Write-Host "Skipping consent server packaging (--without-consent)..."
        $targetYaml = Join-Path $package_folder "repository/conf/deployment.yaml"
        $yqPatched = $false
        if (Get-Command yq -ErrorAction SilentlyContinue) {
            & yq eval '.consent.enabled = false' -i $targetYaml
            if ($LASTEXITCODE -eq 0) {
                $yqPatched = $true
            }
        }
        if (-not $yqPatched) {
            $content = Get-Content $targetYaml
            $inConsent = $false
            for ($i = 0; $i -lt $content.Length; $i++) {
                if ($content[$i] -match '^consent:') {
                    $inConsent = $true
                } elseif ($inConsent -and $content[$i] -match '^\s*enabled:\s*true') {
                    $content[$i] = $content[$i] -replace 'enabled:\s*true', 'enabled: false'
                    $inConsent = $false
                } elseif ($inConsent -and $content[$i] -match '^\S') {
                    $inConsent = $false
                }
            }
            $content | Set-Content $targetYaml
        }
        $consentDisabled = $false
        $inConsentBlock = $false
        foreach ($line in (Get-Content $targetYaml)) {
            if ($line -match '^consent:') {
                $inConsentBlock = $true
            } elseif ($inConsentBlock -and $line -match '^\s+enabled:\s*false') {
                $consentDisabled = $true
                break
            } elseif ($inConsentBlock -and $line -match '^\S') {
                break
            }
        }
        if (-not $consentDisabled) {
            throw "Failed to disable consent in '$targetYaml' — packaging cannot continue with consent still enabled."
        }
    }

    Write-Host "Creating zip file..."
    $zipFile = Join-Path $DIST_DIR "$PRODUCT_FOLDER.zip"
    if (Test-Path $zipFile) {
        Remove-Item $zipFile -Force
    }
    
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory($package_folder, $zipFile)
    
    Remove-Item -Path $package_folder -Recurse -Force
    if (Test-Path $BUILD_DIR) {
        Remove-Item -Path $BUILD_DIR -Recurse -Force
    }
    Write-Host "================================================================"
}

function Build-Sample-App {
    Write-Host "================================================================"
    Write-Host "Building sample apps..."

    # Build React Vanilla sample
    Write-Host "=== Building React Vanilla sample app ==="
    Write-Host "=== Ensuring React Vanilla sample app certificates exist ==="
    Ensure-Certificates -cert_dir $VANILLA_SAMPLE_APP_DIR -cert_name_prefix "server"

    Push-Location $VANILLA_SAMPLE_APP_DIR
    try {
        Write-Host "Installing React Vanilla sample dependencies..."
        & npm ci
        if ($LASTEXITCODE -ne 0) {
            throw "npm ci failed with exit code $LASTEXITCODE"
        }

        Write-Host "Building React Vanilla sample app (TypeScript + Vite)..."

        Write-Host " - Running TypeScript build (tsc -b)..."
        & npx tsc -b
        if ($LASTEXITCODE -ne 0) {
            throw "tsc build failed with exit code $LASTEXITCODE"
        }

        Write-Host " - Running Vite build..."
        & npx vite build
        if ($LASTEXITCODE -ne 0) {
            throw "vite build failed with exit code $LASTEXITCODE"
        }

        # Replicate npm script: copy dist to server/app and copy certs
        $serverDir = Join-Path $VANILLA_SAMPLE_APP_DIR "server"
        $serverAppDir = Join-Path $serverDir "app"
        if (Test-Path $serverAppDir) {
            Remove-Item -Path $serverAppDir -Recurse -Force
        }
        New-Item -Path $serverAppDir -ItemType Directory -Force | Out-Null

        $distFull = Resolve-Path -Path "dist" | Select-Object -ExpandProperty Path
        Copy-Item -Path (Join-Path $distFull "*") -Destination $serverAppDir -Recurse -Force

        # Copy server certs into server directory
        if (Test-Path (Join-Path $VANILLA_SAMPLE_APP_DIR "server.key")) {
            Copy-Item -Path (Join-Path $VANILLA_SAMPLE_APP_DIR "server.key") -Destination $serverDir -Force
        }
        if (Test-Path (Join-Path $VANILLA_SAMPLE_APP_DIR "server.cert")) {
            Copy-Item -Path (Join-Path $VANILLA_SAMPLE_APP_DIR "server.cert") -Destination $serverDir -Force
        }

        # Install server dependencies
        Push-Location $serverDir
        try {
            Write-Host " - Installing server dependencies..."
            & npm ci
            if ($LASTEXITCODE -ne 0) {
                throw "npm ci (server) failed with exit code $LASTEXITCODE"
            }
        }
        finally {
            Pop-Location
        }
    }
    finally {
        Pop-Location
    }

    Write-Host "✅ React Vanilla sample app built successfully."

    # Build React SDK sample
    Write-Host "=== Building React SDK sample app ==="

    # Ensure certificates exist for React SDK sample
    Write-Host "=== Ensuring React SDK sample app certificates exist ==="
    Ensure-Certificates -cert_dir $REACT_SDK_SAMPLE_APP_DIR -cert_name_prefix "server"

    Push-Location $REACT_SDK_SAMPLE_APP_DIR
    try {
        Write-Host "Installing React SDK sample dependencies..."
        & npm ci
        if ($LASTEXITCODE -ne 0) {
            throw "npm ci failed with exit code $LASTEXITCODE"
        }

        Write-Host "Building React SDK sample app..."
        & npm run build
        if ($LASTEXITCODE -ne 0) {
            throw "npm run build failed with exit code $LASTEXITCODE"
        }
    }
    finally {
        Pop-Location
    }

    Write-Host "✅ React SDK sample app built successfully."

    # Build React API-based sample
    Write-Host "=== Building React API-based sample app ==="

    # Ensure certificates exist for React API-based sample
    Write-Host "=== Ensuring React API-based sample app certificates exist ==="
    Ensure-Certificates -cert_dir $REACT_API_SAMPLE_APP_DIR

    Push-Location $REACT_API_SAMPLE_APP_DIR
    try {
        Write-Host "Installing React API-based sample dependencies..."
        & npm ci
        if ($LASTEXITCODE -ne 0) {
            throw "npm ci failed with exit code $LASTEXITCODE"
        }

        Write-Host "Building React API-based sample app..."
        & npm run build
        if ($LASTEXITCODE -ne 0) {
            throw "npm run build failed with exit code $LASTEXITCODE"
        }
    }
    finally {
        Pop-Location
    }

    Write-Host "✅ React API-based sample app built successfully."
    Write-Host "================================================================"
}

function Package-Sample-App {
    Write-Host "================================================================"
    Write-Host "Packaging sample apps..."

    # Package React Vanilla sample
    Write-Host "=== Packaging React Vanilla sample app ==="
    Package-Vanilla-Sample

    # Package React SDK sample
    Write-Host "=== Packaging React SDK sample app ==="
    Package-React-SDK-Sample

    # Package React API-based sample
    Write-Host "=== Packaging React API-based sample app ==="
    Package-React-API-Based-Sample

    Write-Host "================================================================"
}

function Package-Vanilla-Sample {
    # Use appropriate binary name based on OS
    $binary_name = $VANILLA_SAMPLE_APP_SERVER_BINARY_NAME
    $executable_name = "$VANILLA_SAMPLE_APP_SERVER_BINARY_NAME-$SAMPLE_DIST_OS-$SAMPLE_DIST_ARCH"

    if ($SAMPLE_DIST_OS -eq "win") {
        $binary_name = "${VANILLA_SAMPLE_APP_SERVER_BINARY_NAME}.exe"
        $executable_name = "${VANILLA_SAMPLE_APP_SERVER_BINARY_NAME}-${SAMPLE_DIST_OS}-${SAMPLE_DIST_ARCH}.exe"
    }

    $vanilla_sample_app_folder = Join-Path $DIST_DIR $VANILLA_SAMPLE_APP_FOLDER
    New-Item -Path $vanilla_sample_app_folder -ItemType Directory -Force | Out-Null
    $vanilla_sample_app_folder = (Resolve-Path -Path $vanilla_sample_app_folder).Path

    # Copy the built app files
    $serverAppSource = Join-Path $VANILLA_SAMPLE_APP_SERVER_DIR "app"
    if (-not (Test-Path $serverAppSource)) {
        Write-Host "Server app folder '$serverAppSource' not found; falling back to copying from '$VANILLA_SAMPLE_APP_DIR/dist'..."
        New-Item -Path $VANILLA_SAMPLE_APP_SERVER_DIR -ItemType Directory -Force | Out-Null
        New-Item -Path $serverAppSource -ItemType Directory -Force | Out-Null

        $distFull = Resolve-Path -Path (Join-Path $VANILLA_SAMPLE_APP_DIR "dist") | Select-Object -ExpandProperty Path
        Copy-Item -Path (Join-Path $distFull "*") -Destination $serverAppSource -Recurse -Force
    }

    Copy-Item -Path $serverAppSource -Destination $vanilla_sample_app_folder -Recurse -Force

    Push-Location $VANILLA_SAMPLE_APP_SERVER_DIR
    try {
        New-Item -Path "executables" -ItemType Directory -Force | Out-Null

        # Install dependencies to ensure pkg is available
        & npm ci
        if ($LASTEXITCODE -ne 0) {
            throw "npm ci failed with exit code $LASTEXITCODE"
        }

        & npx pkg . -t $SAMPLE_DIST_NODE_VERSION-$SAMPLE_DIST_OS-$SAMPLE_DIST_ARCH -o executables/$VANILLA_SAMPLE_APP_SERVER_BINARY_NAME-$SAMPLE_DIST_OS-$SAMPLE_DIST_ARCH
        if ($LASTEXITCODE -ne 0) {
            throw "npx pkg failed with exit code $LASTEXITCODE"
        }
    }
    finally {
        Pop-Location
    }

    # Copy the server binary
    Copy-Item -Path (Join-Path $VANILLA_SAMPLE_APP_SERVER_DIR "executables/$executable_name") -Destination (Join-Path $vanilla_sample_app_folder $binary_name) -Force

    # Copy README and other necessary files
    if (Test-Path (Join-Path $VANILLA_SAMPLE_APP_DIR "README.md")) {
        Copy-Item -Path (Join-Path $VANILLA_SAMPLE_APP_DIR "README.md") -Destination $vanilla_sample_app_folder -Force
    }

    # Ensure the certificates exist in the sample app directory
    Write-Host "=== Ensuring certificates exist in the React Vanilla sample distribution ==="
    Ensure-Certificates -cert_dir $vanilla_sample_app_folder -cert_name_prefix "server"

    # Copy the appropriate startup script based on the target OS
    if ($SAMPLE_DIST_OS -eq "win") {
        Write-Host "Including Windows start script (start.ps1)..."
        Copy-Item -Path (Join-Path $VANILLA_SAMPLE_APP_SERVER_DIR "start.ps1") -Destination $vanilla_sample_app_folder -Force
    }
    else {
        Write-Host "Including Unix start script (start.sh)..."
        Copy-Item -Path (Join-Path $VANILLA_SAMPLE_APP_SERVER_DIR "start.sh") -Destination $vanilla_sample_app_folder -Force
    }

    Write-Host "Creating React Vanilla sample zip file..."
    $distAbs = (Resolve-Path -Path $DIST_DIR).Path
    $zipFile = [System.IO.Path]::Combine($distAbs, "$VANILLA_SAMPLE_APP_FOLDER.zip")
    if (Test-Path $zipFile) {
        Remove-Item $zipFile -Force
    }

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory($vanilla_sample_app_folder, $zipFile)

    Remove-Item -Path $vanilla_sample_app_folder -Recurse -Force

    Write-Host "✅ React Vanilla sample app packaged successfully as $zipFile"
}

function Package-React-SDK-Sample {
    $react_sdk_sample_app_folder_t = Join-Path $DIST_DIR $REACT_SDK_SAMPLE_APP_FOLDER
    New-Item -Path $react_sdk_sample_app_folder_t -ItemType Directory -Force | Out-Null

    # Copy the built React app (dist folder)
    if (Test-Path (Join-Path $REACT_SDK_SAMPLE_APP_DIR "dist")) {
        Write-Host "Copying React SDK sample build output..."
        Copy-Item -Path (Join-Path $REACT_SDK_SAMPLE_APP_DIR "dist") -Destination $react_sdk_sample_app_folder_t -Recurse -Force
    }
    else {
        Write-Host "Warning: React SDK sample build output not found at $((Join-Path $REACT_SDK_SAMPLE_APP_DIR 'dist'))"
        throw "React SDK sample build output not found"
    }

    # Copy README and other necessary files
    if (Test-Path (Join-Path $REACT_SDK_SAMPLE_APP_DIR "README.md")) {
        Copy-Item -Path (Join-Path $REACT_SDK_SAMPLE_APP_DIR "README.md") -Destination $react_sdk_sample_app_folder_t -Force
    }

    if (Test-Path (Join-Path $REACT_SDK_SAMPLE_APP_DIR ".env.example")) {
        Copy-Item -Path (Join-Path $REACT_SDK_SAMPLE_APP_DIR ".env.example") -Destination $react_sdk_sample_app_folder_t -Force
    }

    # Copy the appropriate startup script based on the target OS
    if ($SAMPLE_DIST_OS -eq "win") {
        Write-Host "Including Windows start script (start.ps1)..."
        Copy-Item -Path (Join-Path $REACT_SDK_SAMPLE_APP_DIR "start.ps1") -Destination $react_sdk_sample_app_folder_t -Force
    }
    else {
        Write-Host "Including Unix start script (start.sh)..."
        Copy-Item -Path (Join-Path $REACT_SDK_SAMPLE_APP_DIR "start.sh") -Destination $react_sdk_sample_app_folder_t -Force
    }

    Write-Host "Creating React SDK sample zip file..."
    $distAbs = (Resolve-Path -Path $DIST_DIR).Path
    $zipFile = [System.IO.Path]::Combine($distAbs, "$REACT_SDK_SAMPLE_APP_FOLDER.zip")
    if (Test-Path $zipFile) {
        Remove-Item $zipFile -Force
    }

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory($react_sdk_sample_app_folder_t, $zipFile)

    Remove-Item -Path $react_sdk_sample_app_folder_t -Recurse -Force

    Write-Host "✅ React SDK sample app packaged successfully as $zipFile"
}

function Package-React-API-Based-Sample {
    $react_api_sample_app_folder_t = Join-Path $DIST_DIR $REACT_API_SAMPLE_APP_FOLDER
    New-Item -Path $react_api_sample_app_folder_t -ItemType Directory -Force | Out-Null

    # Copy the built React app (dist folder)
    if (Test-Path (Join-Path $REACT_API_SAMPLE_APP_DIR "dist")) {
        Write-Host "Copying React API-based sample build output..."
        Copy-Item -Path (Join-Path $REACT_API_SAMPLE_APP_DIR "dist") -Destination $react_api_sample_app_folder_t -Recurse -Force
    }
    else {
        Write-Host "Warning: React API-based sample build output not found at $((Join-Path $REACT_API_SAMPLE_APP_DIR 'dist'))"
        throw "React API-based sample build output not found"
    }

    # Copy README if it exists
    if (Test-Path (Join-Path $REACT_API_SAMPLE_APP_DIR "README.md")) {
        Copy-Item -Path (Join-Path $REACT_API_SAMPLE_APP_DIR "README.md") -Destination $react_api_sample_app_folder_t -Force
    }

    # Ensure the certificates exist in the sample app dist directory
    Write-Host "=== Ensuring certificates exist in the React API-based sample distribution ==="
    Ensure-Certificates -cert_dir (Join-Path $react_api_sample_app_folder_t "dist")

    # Copy the appropriate startup script based on the target OS
    if ($SAMPLE_DIST_OS -eq "win") {
        Write-Host "Including Windows start script (start.ps1)..."
        Copy-Item -Path (Join-Path $REACT_API_SAMPLE_APP_DIR "start.ps1") -Destination $react_api_sample_app_folder_t -Force
    }
    else {
        Write-Host "Including Unix start script (start.sh)..."
        Copy-Item -Path (Join-Path $REACT_API_SAMPLE_APP_DIR "start.sh") -Destination $react_api_sample_app_folder_t -Force
    }

    Write-Host "Creating React API-based sample zip file..."
    $distAbs = (Resolve-Path -Path $DIST_DIR).Path
    $zipFile = [System.IO.Path]::Combine($distAbs, "$REACT_API_SAMPLE_APP_FOLDER.zip")
    if (Test-Path $zipFile) {
        Remove-Item $zipFile -Force
    }

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory($react_api_sample_app_folder_t, $zipFile)

    Remove-Item -Path $react_api_sample_app_folder_t -Recurse -Force

    Write-Host "✅ React API-based sample app packaged successfully as $zipFile"
}

function Test-Unit {
    Write-Host "================================================================"
    Write-Host "Running unit tests with coverage..."
    
    Push-Location $BACKEND_BASE_DIR
    try {
        # Build coverage package list
        $exclude_pattern = Get-CoverageExclusionPattern
        $coverpkg = ""
        
        if ($exclude_pattern) {
            Write-Host "Excluding coverage for patterns: $exclude_pattern"
            $packages = & go list ./...
            $filtered_packages = $packages | Where-Object { $_ -notmatch $exclude_pattern }
            $coverpkg = $filtered_packages -join ','
        }
        else {
            Write-Host "No exclusion patterns found, including all packages"
            $packages = & go list ./...
            $coverpkg = $packages -join ','
        }
        
        # Check if gotestsum is available
        $gotestsum = Get-Command gotestsum -ErrorAction SilentlyContinue
        
        if ($gotestsum) {
            Write-Host "Running unit tests with coverage using gotestsum..."
            & gotestsum -- -v -count=1 "-coverprofile=coverage_unit.out" "-covermode=atomic" "-coverpkg=$coverpkg" ./...
            if ($LASTEXITCODE -ne 0) {
                Write-Host "There are unit test failures."
                exit 1
            }
        }
        else {
            Write-Host "Running unit tests with coverage using go test..."
            & go test -v -count=1 "-coverprofile=coverage_unit.out" "-covermode=atomic" "-coverpkg=$coverpkg" ./...
            if ($LASTEXITCODE -ne 0) {
                Write-Host "There are unit test failures."
                exit 1
            }
        }
        
        Write-Host "Unit test coverage profile generated in: backend/coverage_unit.out"
        
        # Generate HTML coverage report for unit tests
        & go tool cover "-html=coverage_unit.out" "-o=coverage_unit.html"
        Write-Host "Unit test coverage HTML report generated in: backend/coverage_unit.html"
        
        # Display unit test coverage summary
        Write-Host ""
        Write-Host "================================================================"
        Write-Host "Unit Test Coverage Summary:"
        & go tool cover "-func=coverage_unit.out" | Select-Object -Last 1
        Write-Host "================================================================"
        Write-Host ""
    }
    finally {
        Pop-Location
    }
    
    Write-Host "================================================================"
}

function Test-Integration {
    Write-Host "================================================================"
    Write-Host "Running integration tests..."
    
    # Build extra args for test filtering
    $extra_args = @()
    
    if ($TestRun) {
        $extra_args += "-run"
        $extra_args += $TestRun
        Write-Host "Test filter: -run $TestRun"
    }
    
    if ($TestPackage) {
        $extra_args += "-package"
        $extra_args += $TestPackage
        Write-Host "Test package: $TestPackage"
    }
    
    Push-Location $SCRIPT_DIR
    try {
        # Set up coverage directory for integration tests
        $coverage_dir = Join-Path (Get-Location) "$OUTPUT_DIR\.test\integration"
        New-Item -Path $coverage_dir -ItemType Directory -Force | Out-Null
        
        # Export coverage directory for the server binary to use
        $env:GOCOVERDIR = $coverage_dir
        
        Write-Host "Coverage data will be collected in: $coverage_dir"
        if ($extra_args.Count -gt 0) {
            & go run -C ./tests/integration ./main.go @extra_args
        } else {
            & go run -C ./tests/integration ./main.go
        }
        $test_exit_code = $LASTEXITCODE
        
        # Process coverage data if tests passed or failed
        if ((Test-Path $coverage_dir) -and ((Get-ChildItem $coverage_dir -ErrorAction SilentlyContinue).Count -gt 0)) {
            Write-Host "================================================================"
            Write-Host "Processing integration test coverage..."
            
            # Convert binary coverage data to text format
            Push-Location $BACKEND_BASE_DIR
            try {
                & go tool covdata textfmt -i="$coverage_dir" -o="../$TARGET_DIR/coverage_integration.out"
                Write-Host "Integration test coverage report generated in: $TARGET_DIR/coverage_integration.out"
                
                # Generate HTML coverage report
                & go tool cover -html="../$TARGET_DIR/coverage_integration.out" -o="../$TARGET_DIR/coverage_integration.html"
                Write-Host "Integration test coverage HTML report generated in: $TARGET_DIR/coverage_integration.html"
                
                # Display coverage summary
                Write-Host ""
                Write-Host "================================================================"
                Write-Host "Coverage Summary:"
                & go tool cover -func="../$TARGET_DIR/coverage_integration.out" | Select-Object -Last 1
                Write-Host "================================================================"
                Write-Host ""
            }
            finally {
                Pop-Location
            }
        }
        else {
            Write-Host "================================================================"
            Write-Host "No coverage data collected"
        }
        
        # Exit with the test exit code
        if ($test_exit_code -ne 0) {
            Write-Host "================================================================"
            Write-Host "Integration tests failed with exit code: $test_exit_code"
            exit $test_exit_code
        }
    }
    finally {
        Pop-Location
    }
    
    Write-Host "================================================================"
}

function Merge-Coverage {
    Write-Host "================================================================"
    Write-Host "Merging coverage reports..."
    
    Push-Location $SCRIPT_DIR
    try {
        $unit_coverage = Join-Path $BACKEND_BASE_DIR "coverage_unit.out"
        $integration_coverage = Join-Path $TARGET_DIR "coverage_integration.out"
        $combined_coverage = Join-Path $TARGET_DIR "coverage_combined.out"
        
        # Check if both coverage files exist
        if (-not (Test-Path $unit_coverage)) {
            Write-Host "Warning: Unit test coverage file not found at $unit_coverage"
            Write-Host "Skipping coverage merge."
            return
        }
        
        if (-not (Test-Path $integration_coverage)) {
            Write-Host "Warning: Integration test coverage file not found at $integration_coverage"
            Write-Host "Skipping coverage merge."
            return
        }
        
        Write-Host "Merging unit and integration test coverage..."
        
        # Get the mode from the first file and write to combined coverage
        $mode_line = Get-Content $unit_coverage -First 1
        $mode_line | Set-Content $combined_coverage
        
        # Read both files (skip mode lines) and merge overlapping coverage
        $unit_lines = Get-Content $unit_coverage | Select-Object -Skip 1
        $integration_lines = Get-Content $integration_coverage | Select-Object -Skip 1
        
        # Combine and process coverage data
        $coverage_map = @{}
        
        foreach ($line in ($unit_lines + $integration_lines)) {
            $parts = $line -split '\s+'
            if ($parts.Count -ge 3) {
                $key = "$($parts[0]) $($parts[1])"
                $count = [int]$parts[2]
                
                if ($coverage_map.ContainsKey($key)) {
                    # For duplicate entries, take the maximum count
                    if ($count -gt $coverage_map[$key]) {
                        $coverage_map[$key] = $count
                    }
                }
                else {
                    $coverage_map[$key] = $count
                }
            }
        }
        
        # Sort and write to combined coverage file
        $sorted_lines = $coverage_map.GetEnumerator() | Sort-Object Key | ForEach-Object {
            "$($_.Key) $($_.Value)"
        }
        
        $sorted_lines | Add-Content $combined_coverage
        
        Write-Host "Combined coverage report generated in: $combined_coverage"
        
        # Generate HTML coverage report for combined coverage
        Push-Location $BACKEND_BASE_DIR
        try {
            & go tool cover -html="../$combined_coverage" -o="../$TARGET_DIR/coverage_combined.html"
            Write-Host "Combined coverage HTML report generated in: $TARGET_DIR/coverage_combined.html"
            
            # Display combined coverage summary
            Write-Host ""
            Write-Host "================================================================"
            Write-Host "Combined Test Coverage Summary:"
            & go tool cover -func="../$combined_coverage" | Select-Object -Last 1
            Write-Host "================================================================"
            Write-Host ""
        }
        finally {
            Pop-Location
        }
    }
    finally {
        Pop-Location
    }
    
    Write-Host "================================================================"
}

function Export-CertificateAndKeyToPem {
    param(
        [System.Security.Cryptography.X509Certificates.X509Certificate2]$cert,
        [string]$certPath,
        [string]$keyPath,
        [System.Security.Cryptography.RSA]$privateRSA = $null
    )
    # Export cert to PEM
    $rawCert = $cert.Export([System.Security.Cryptography.X509Certificates.X509ContentType]::Cert)
    $certBase64 = [System.Convert]::ToBase64String($rawCert)
    $certLines = $certBase64 -split '(.{64})' | Where-Object { $_ -ne '' }
    $certPem = "-----BEGIN CERTIFICATE-----`n" + ($certLines -join "`n") + "`n-----END CERTIFICATE-----`n"
    Set-Content -Path $certPath -Value $certPem -Encoding ascii

    # Obtain RSA private key. If a privateRSA instance was provided by the caller use it
    # (this avoids relying on PFX export/import semantics which can vary across runtimes).
    $rsa = $null
    $reloadCert = $null
    try {
        if ($null -ne $privateRSA) {
            $rsa = $privateRSA
        }
        else {
            # Export as PFX and reload with Exportable flag so we can export the private key
            $pfxBytes = $cert.Export([System.Security.Cryptography.X509Certificates.X509ContentType]::Pfx, '')
            $reloadCert = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new($pfxBytes, '', [System.Security.Cryptography.X509Certificates.X509KeyStorageFlags]::Exportable)

            # Try the modern API first
            try { $rsa = $reloadCert.GetRSAPrivateKey() } catch { $rsa = $null }

            # Fallback: some runtimes expose PrivateKey which can export parameters
            if (-not $rsa -and $null -ne $reloadCert.PrivateKey) {
                try {
                    $privateKey = $reloadCert.PrivateKey
                    $rsaFallback = [System.Security.Cryptography.RSA]::Create()
                    $rsaFallback.ImportParameters($privateKey.ExportParameters($true))
                    $rsa = $rsaFallback
                }
                catch {
                    if ($rsaFallback -is [System.IDisposable]) { $rsaFallback.Dispose() }
                    $rsa = $null
                }
            }
        }

        if (-not $rsa) { throw "Certificate does not contain an RSA private key" }

        # Export private key to PEM (PKCS#8)
        $pkcs8 = $rsa.ExportPkcs8PrivateKey()
        $keyBase64 = [System.Convert]::ToBase64String($pkcs8)
        $pkcs8Lines = $keyBase64 -split '(.{64})' | Where-Object { $_ -ne '' }
        $keyPem = "-----BEGIN PRIVATE KEY-----`n" + ($pkcs8Lines -join "`n") + "`n-----END PRIVATE KEY-----`n"
        Set-Content -Path $keyPath -Value $keyPem -Encoding ascii
    }
    finally {
        # Only dispose RSA if we created it locally (i.e., privateRSA was not passed in)
        if ($null -eq $privateRSA) {
            if ($rsa -is [System.IDisposable]) { $rsa.Dispose() }
            if ($reloadCert -is [System.IDisposable]) { $reloadCert.Dispose() }
        }
    }
}

function Ensure-Certificates {
    param(
        [string]$cert_dir,
        [string]$cert_name_prefix = "server"  # Default to "server" if not specified
    )
    
    $cert_file_name = "${cert_name_prefix}.cert"
    $key_file_name = "${cert_name_prefix}.key"

    # Generate certificate and key file if they don't exist in the cert directory
    $local_cert_file = Join-Path $LOCAL_CERT_DIR $cert_file_name
    $local_key_file = Join-Path $LOCAL_CERT_DIR $key_file_name
    
    if (-not (Test-Path $local_cert_file) -or -not (Test-Path $local_key_file)) {
        New-Item -Path $LOCAL_CERT_DIR -ItemType Directory -Force | Out-Null
        
        Write-Host "Generating certificates ($cert_name_prefix) in $LOCAL_CERT_DIR..."
        
        try {
            $openssl = Get-Command openssl -ErrorAction SilentlyContinue
            if ($openssl) {
                & openssl req -x509 -nodes -days 365 -newkey rsa:2048 `
                    -keyout $local_key_file `
                    -out $local_cert_file `
                    -subj "/O=WSO2/OU=$PRODUCT_NAME/CN=localhost" 2>$null
                if ($LASTEXITCODE -ne 0) {
                    throw "Error generating certificates: OpenSSL failed with exit code $LASTEXITCODE"
                }
                Write-Host "Certificates generated successfully in $LOCAL_CERT_DIR using OpenSSL."
            }
            else {
                Write-Host "OpenSSL not found - generating certificates using .NET CertificateRequest (no UI)."
                # Use .NET CertificateRequest to avoid CertEnroll / smartcard enrollment UI issues.
                try {
                    $rsa = [System.Security.Cryptography.RSA]::Create(2048)

                    $subjectName = New-Object System.Security.Cryptography.X509Certificates.X500DistinguishedName("CN=localhost, O=WSO2, OU=$PRODUCT_NAME")
                    $certReq = New-Object System.Security.Cryptography.X509Certificates.CertificateRequest($subjectName, $rsa, [System.Security.Cryptography.HashAlgorithmName]::SHA256, [System.Security.Cryptography.RSASignaturePadding]::Pkcs1)

                    # Add standard server usages
                    $basicConstraints = New-Object System.Security.Cryptography.X509Certificates.X509BasicConstraintsExtension($false, $false, 0, $false)
                    $ku1 = [int][System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::DigitalSignature
                    $ku2 = [int][System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]::KeyEncipherment
                    $kuFlags = $ku1 -bor $ku2
                    $keyUsage = New-Object System.Security.Cryptography.X509Certificates.X509KeyUsageExtension([System.Security.Cryptography.X509Certificates.X509KeyUsageFlags]$kuFlags, $true)
                    $ekuCollection = New-Object System.Security.Cryptography.OidCollection
                    $serverAuthOid = New-Object System.Security.Cryptography.Oid("1.3.6.1.5.5.7.3.1")
                    [void]$ekuCollection.Add($serverAuthOid)
                    $eku = New-Object System.Security.Cryptography.X509Certificates.X509EnhancedKeyUsageExtension($ekuCollection, $false)

                    $certReq.CertificateExtensions.Add($basicConstraints)
                    $certReq.CertificateExtensions.Add($keyUsage)
                    $certReq.CertificateExtensions.Add($eku)

                    $notBefore = (Get-Date).AddDays(-1)
                    $notAfter = (Get-Date).AddYears(1)

                    $cert = $certReq.CreateSelfSigned($notBefore, $notAfter)

                    # Ensure the generated certificate has the private key associated. Use CopyWithPrivateKey
                    # so that when we export the PFX it includes the private key and can be reloaded as exportable.
                    # Use the RSA extension helper to avoid overload resolution issues in PowerShell.
                    try {
                        $certWithKey = [System.Security.Cryptography.X509Certificates.RSACertificateExtensions]::CopyWithPrivateKey($cert, $rsa)
                    }
                    catch {
                        try {
                            $certWithKey = [System.Security.Cryptography.X509Certificates.RSACertificateExtensions]::CopyWithPrivateKey(([System.Security.Cryptography.X509Certificates.X509Certificate2]::new($cert.RawData)), $rsa)
                        }
                        catch {
                            throw "Failed to associate private key with certificate: $_"
                        }
                    }

                    # Export and reload as exportable so we can extract the private key bytes
                    $pfxBytes = $certWithKey.Export([System.Security.Cryptography.X509Certificates.X509ContentType]::Pfx, '')
                    $exportableCert = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new($pfxBytes, '', [System.Security.Cryptography.X509Certificates.X509KeyStorageFlags]::Exportable)

                    # Pass the RSA instance used to sign the certificate to the exporter so it
                    # can directly export the private key (avoids re-import issues on some runtimes).
                    Export-CertificateAndKeyToPem -cert $exportableCert -certPath $local_cert_file -keyPath $local_key_file -privateRSA $rsa

                    if ($exportableCert -is [System.IDisposable]) { $exportableCert.Dispose() }
                    if ($certWithKey -is [System.IDisposable]) { $certWithKey.Dispose() }
                    if ($cert -is [System.IDisposable]) { $cert.Dispose() }
                    if ($rsa -is [System.IDisposable]) { $rsa.Dispose() }

                    Write-Host "Certificates generated successfully in $LOCAL_CERT_DIR using .NET CertificateRequest." 
                }
                catch {
                    throw "Error creating certificates using .NET APIs: $_"
                }
            }
        }
        catch {
            Write-Error "Error generating certificates: $_"
            exit 1
        }
    }
    else {
        Write-Host "Certificates ($cert_name_prefix) already exist in $LOCAL_CERT_DIR."
    }

    # Copy the generated certificates to the specified directory
    $cert_file = Join-Path $cert_dir $cert_file_name
    $key_file = Join-Path $cert_dir $key_file_name

    if (-not (Test-Path $cert_file) -or -not (Test-Path $key_file)) {
        New-Item -Path $cert_dir -ItemType Directory -Force | Out-Null
        
        Write-Host "Copying certificates ($cert_name_prefix) to $cert_dir..."
        Copy-Item -Path $local_cert_file -Destination $cert_file -Force
        Copy-Item -Path $local_key_file -Destination $key_file -Force
        Write-Host "Certificates copied successfully to $cert_dir."
    }
    else {
        Write-Host "Certificates ($cert_name_prefix) already exist in $cert_dir."
    }
}

function Ensure-Crypto-File {
    param(
        [string]$conf_dir
    )

    # Resolve the .. path segment to get a clean key directory path
    $KEY_DIR_Temp = Join-Path $conf_dir ".." "resources/security"
    $KEY_DIR = (Resolve-Path -Path $KEY_DIR_Temp).Path
    $KEY_FILE = Join-Path $KEY_DIR "crypto.key"

    Write-Host "================================================================"
    Write-Host "Ensuring crypto key file exists..."

    # Check Whether the key file exists
    if (Test-Path $KEY_FILE) {
        Write-Host "Default crypto key file already present in $KEY_FILE. Skipping generation."
    }
    else {
        Write-Host "Default crypto key file not found. Generating new key at $KEY_FILE..."
        $NEW_KEY = $null
        
        # Try generating key using OpenSSL first
        $openssl = Get-Command openssl -ErrorAction SilentlyContinue
        if ($openssl) {
            try {
                Write-Host " - Using OpenSSL to generate key..."
                # openssl rand -hex 32 returns a 64-char string.
                $NEW_KEY = (openssl rand -hex 32 | Out-String).Trim()
                
                if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrEmpty($NEW_KEY) -or $NEW_KEY.Length -ne 64) {
                    throw "OpenSSL rand command failed or returned empty/incorrect length."
                }
            }
            catch {
                Write-Host " - OpenSSL failed: $_. Falling back to POSIX tools/DOTNET."
                $NEW_KEY = $null
            }
        }
        else {
            Write-Host " - OpenSSL not found. Falling back to POSIX tools/DOTNET."
        }

        # Try POSIX tools as first fallback option
        if ([string]::IsNullOrEmpty($NEW_KEY)) {
            $bash = Get-Command bash -ErrorAction SilentlyContinue
            if ($bash -and (Test-Path /dev/urandom)) {
                try {
                    Write-Host " - Using POSIX tools (/dev/urandom) to generate key..."
                    # Command: head -c 32 /dev/urandom | xxd -p -c 256
                    # Generates 32 random bytes, converts to a single line of hex (64 chars)
                    # The ToLower() ensures consistency with the openssl/dotnet output.
                    $POS_KEY_RAW = (& bash -c 'head -c 32 /dev/urandom | xxd -p -c 256' | Out-String).Trim()
                    $NEW_KEY = $POS_KEY_RAW.ToLower()
                    
                    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrEmpty($NEW_KEY) -or $NEW_KEY.Length -ne 64) {
                         throw "POSIX key generation command failed or returned invalid length."
                    }
                }
                catch {
                    Write-Host " - POSIX tool failed: $_. Falling back to .NET cryptography."
                    $NEW_KEY = $null
                }
            }
            else {
                Write-Host " - POSIX tools not found or not suitable. Falling back to .NET cryptography."
            }
        }

        # try .NET cryptography as final fallback
        if ([string]::IsNullOrEmpty($NEW_KEY)) {
            try {
                Write-Host " - Using .NET cryptography to generate key..."
                $bytes = New-Object byte[] 32
                # Note: System.Security.Cryptography.RandomNumberGenerator is available in both .NET Framework and .NET (Core)
                $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
                $rng.GetBytes($bytes)
                $rng.Dispose()
                # Convert bytes to lowercase hex string (64 chars)
                $NEW_KEY = ([System.BitConverter]::ToString($bytes) -replace '-', '').ToLower()
            }
            catch {
                 throw "Failed to generate crypto key using .NET: $_"
            }
        }
        # --- END: .NET cryptography fallback ---
        
        # Ensure the target directory exists
        New-Item -Path $KEY_DIR -ItemType Directory -Force | Out-Null

        # Write the key to the new file (NoNewline matches 'echo -n')
        Set-Content -Path $KEY_FILE -Value $NEW_KEY -NoNewline -Encoding Ascii
        
        Write-Host "Successfully generated and added new crypto key to $KEY_FILE."
    }

    Write-Host "================================================================"
}

function Run {
    function Cleanup-Servers {
        Write-Host ""
        Write-Host "🛑 Shutting down servers..."
        if ($script:FRONTEND_PID) { 
            Stop-Process -Id $script:FRONTEND_PID -Force -ErrorAction SilentlyContinue
        }
        Get-Process -Name "*pnpm*" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
        Get-Process -Name "node" -ErrorAction SilentlyContinue | Where-Object { $_.ProcessName -like "*vite*" } | Stop-Process -Force -ErrorAction SilentlyContinue
        if ($script:BACKEND_PID) { 
            Stop-Process -Id $script:BACKEND_PID -Force -ErrorAction SilentlyContinue
        }
        if ($script:CONSENT_PROCESS -and -not $script:CONSENT_PROCESS.HasExited) {
            Stop-Process -Id $script:CONSENT_PROCESS.Id -Force -ErrorAction SilentlyContinue
        }
        Start-Sleep -Seconds 1
        Write-Host "✅ All servers stopped successfully."
    }

    Write-Host "Running frontend apps..."
    Run-Frontend

    if ($script:CONSENT_ENABLED -and -not $skipConsent) {
        Write-Host "Running consent server..."
        Run-Consent
    }

    # Save original skip security value and temporarily set to true
    $script:ORIGINAL_SKIP_SECURITY = $env:SKIP_SECURITY
    $env:SKIP_SECURITY = "true"
    Run-Backend -ShowFinalOutput $false

    # Run initial data setup
    Write-Host "⚙️  Running initial data setup..."
    Write-Host ""
    
    # Wait for server to be ready
    $MAX_RETRIES = 30
    $RETRY_INTERVAL = 2
    $retries = 0
    
    # Configure TLS to use modern protocols (required for HTTPS requests on Windows)
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13
    } catch {
        # Fallback to TLS 1.2 if TLS 1.3 is not available
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    }
    
    Write-Host "[INFO] Waiting for $PRODUCT_NAME server to be ready..."
    while ($retries -lt $MAX_RETRIES) {
        try {
            $response = Invoke-WebRequest -Uri "$BASE_URL/health/readiness" -UseBasicParsing -SkipCertificateCheck -ErrorAction Stop
            if ($response.StatusCode -eq 200) {
                Write-Host "✓ Server is ready!"
                break
            }
        }
        catch {
            # Server not ready yet
        }
        
        $retries++
        if ($retries -ge $MAX_RETRIES) {
            Write-Host "❌ Server did not become ready after $MAX_RETRIES attempts"
            Write-Host "💡 Please ensure the $PRODUCT_NAME server is running at $BASE_URL"
            exit 1
        }
        
        Write-Host "[WAITING] Attempt $retries/$MAX_RETRIES - Server not ready yet, retrying in ${RETRY_INTERVAL}s..."
        Start-Sleep -Seconds $RETRY_INTERVAL
    }
    
    Write-Host ""
    
    # Run the bootstrap script directly with environment variable and arguments
    $env:API_BASE = $BASE_URL
    $env:SYSTEM_RS_HANDLE = if ($script:SYSTEM_RS_HANDLE) { $script:SYSTEM_RS_HANDLE } else { "" }
    $env:SYSTEM_RS_IDENTIFIER = if ($script:SYSTEM_RS_IDENTIFIER) { $script:SYSTEM_RS_IDENTIFIER } else { "" }
    $bootstrapScript = Join-Path $BACKEND_BASE_DIR "cmd/server/bootstrap/01-default-resources.ps1"
    & $bootstrapScript -ConsoleRedirectUris "https://localhost:${CONSOLE_APP_DEFAULT_PORT}/console"

    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ Initial data setup failed"
        Write-Host "💡 Check the logs above for more details"
        exit 1
    }

    Write-Host "🔒 Restoring security setting and restarting backend..."
    # Restore original skip security value
    if (![string]::IsNullOrEmpty($script:ORIGINAL_SKIP_SECURITY)) {
        $env:SKIP_SECURITY = $script:ORIGINAL_SKIP_SECURITY
    }
    else {
        Remove-Item Env:\SKIP_SECURITY -ErrorAction SilentlyContinue
    }
    # Start backend with initial output but without final output/wait
    Start-Backend -ShowFinalOutput $false

    Write-Host ""
    Write-Host "🚀 Servers running:"
    Write-Host "  👉 Backend : $BASE_URL"
    Write-Host "  📱 Frontend :"
    Write-Host "      🚪 Gate (Login/Register): https://localhost:${GATE_APP_DEFAULT_PORT}/gate"
    Write-Host "      🛠️  Console (System Management): https://localhost:${CONSOLE_APP_DEFAULT_PORT}/console"
    Write-Host ""

    Write-Host "Press Ctrl+C to stop."

    # Set up Ctrl+C handler
    [Console]::TreatControlCAsInput = $false
    
    # Wait for user to press Ctrl+C
    try {
        while ($true) {
            Start-Sleep -Seconds 1
        }
    }
    catch [System.Management.Automation.PipelineStoppedException] {
        Cleanup-Servers
        exit 0
    }

    Wait-Process $script:BACKEND_PID -ErrorAction SilentlyContinue
}

function Run-Backend {
    param(
        [bool]$ShowFinalOutput = $true
    )

    Write-Host "=== Ensuring server certificates exist ==="
    Ensure-Certificates -cert_dir (Join-Path $BACKEND_DIR $SECURITY_DIR) -cert_name_prefix "server"
    Ensure-Certificates -cert_dir (Join-Path $BACKEND_DIR $SECURITY_DIR) -cert_name_prefix "signing"

    Write-Host "=== Ensuring React Vanilla sample app certificates exist ==="
    Ensure-Certificates -cert_dir $VANILLA_SAMPLE_APP_DIR -cert_name_prefix "server"

    Write-Host "=== Ensuring crypto file exists for run ==="
    Ensure-Crypto-File -conf_dir (Join-Path $BACKEND_DIR "repository/conf")

    Write-Host "Initializing databases..."
    Initialize-Databases

    if ($script:CONSENT_ENABLED -and -not $skipConsent -and -not $script:CONSENT_PROCESS) {
        Write-Host "Running consent server..."
        Run-Consent
    }

    Start-Backend -ShowFinalOutput $ShowFinalOutput
}

function Start-Backend {
    param(
        [bool]$ShowFinalOutput = $true
    )

    # Kill processes on known ports
    function Kill-Port {
        param([int]$port)
        
        $processes = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess
        foreach ($process in $processes) {
            Stop-Process -Id $process -Force -ErrorAction SilentlyContinue
        }
    }

    Kill-Port $PORT

    Write-Host "=== Starting backend on $BASE_URL ==="
    
    Push-Location $BACKEND_DIR
    try {
        $backendProcess = Start-Process -FilePath "go" -ArgumentList "run", "." -PassThru -NoNewWindow
        $script:BACKEND_PID = $backendProcess.Id
    }
    finally {
        Pop-Location
    }

    if ($ShowFinalOutput) {
        Write-Host ""
        Write-Host "🚀 Servers running:"
        Write-Host "👉 Backend : $BASE_URL"
        Write-Host "Press Ctrl+C to stop."

        try {
            while ($true) {
                Start-Sleep -Seconds 1
            }
        }
        catch [System.Management.Automation.PipelineStoppedException] {
            Write-Host ""
            Write-Host "🛑 Shutting down servers..."
            if ($script:BACKEND_PID) { 
                Stop-Process -Id $script:BACKEND_PID -Force -ErrorAction SilentlyContinue
            }
            if ($script:CONSENT_PROCESS -and -not $script:CONSENT_PROCESS.HasExited) {
                Stop-Process -Id $script:CONSENT_PROCESS.Id -Force -ErrorAction SilentlyContinue
            }
            Write-Host "✅ Servers stopped successfully."
            exit 0
        }

        Wait-Process $backendProcess -ErrorAction SilentlyContinue
    }
}

function Run-Frontend {
    Write-Host "================================================================"
    Write-Host "Running frontend apps..."
    Ensure-Pnpm
    
    # Install dependencies
    try {
        Write-Host "Installing frontend dependencies..."
        & pnpm install --frozen-lockfile
        
        Write-Host "Building frontend applications & packages..."
        & pnpm build:frontend
        
        Write-Host "Starting frontend applications in the background..."
        # Start frontend processes in background
        $frontendProcess = Start-Process -FilePath "cmd.exe" -ArgumentList "/c", "pnpm", "-r", "--parallel", "--filter", "@thunderid/console", "--filter", "@thunderid/gate", "dev" -PassThru -NoNewWindow
        $script:FRONTEND_PID = $frontendProcess.Id
    }
    finally {
        Pop-Location
    }
    
    Write-Host "================================================================"
}

function Run-Docs {
    Write-Host "================================================================"
    Write-Host "Starting documentation development server..."
    Ensure-Pnpm
    
    # Install dependencies
    try {
        Write-Host "Installing frontend dependencies (required for docs)..."
        & pnpm install --frozen-lockfile
    }
    finally {
        Pop-Location
    }
    
    # Navigate to docs directory
    Push-Location (Join-Path $SCRIPT_DIR "docs")
    try {
        Write-Host "Starting documentation server with live reload..."
        Write-Host "📚 Documentation will be available at http://localhost:$DOCS_DEFAULT_PORT"
        Write-Host "Press Ctrl+C to stop."
        & pnpm dev
    }
    finally {
        Pop-Location
    }
    
    Write-Host "================================================================"
}

function Run-Consent {
    $consentDir = Join-Path $TARGET_DIR "consent"
    $consentBinary = Join-Path $consentDir "consent-server.exe"
    if ($GO_OS -ne "windows") {
        $consentBinary = Join-Path $consentDir "consent-server"
    }

    if (-not (Test-Path $consentBinary)) {
        Write-Host "=== Downloading consent server ==="
        & .\scripts\package-consent-server.ps1 -GoOS $GO_OS -GoArch $GO_ARCH -DistOutputPath $TARGET_DIR
        if ($LASTEXITCODE -ne 0) { throw "Failed to package consent server" }
    }

    if (-not (Test-Path $consentBinary)) {
        Write-Host "Error: Consent server binary not found at $consentBinary"
        exit 1
    }

    Write-Host "=== Starting consent server ==="
    $consentPort = if ($env:CONSENT_SERVER_PORT) { $env:CONSENT_SERVER_PORT } else { "9090" }
    $script:CONSENT_PROCESS = Start-Process -FilePath $consentBinary -WorkingDirectory $consentDir -PassThru -NoNewWindow
    $consentTimeout = 30
    $consentElapsed = 0
    while ($consentElapsed -lt $consentTimeout) {
        if ($script:CONSENT_PROCESS.HasExited) {
            Write-Host "Error: Consent server process exited unexpectedly"
            exit 1
        }
        try {
            $resp = Invoke-WebRequest -Uri "http://localhost:${consentPort}/health/readiness" -UseBasicParsing -ErrorAction Stop
            if ($resp.StatusCode -eq 200) {
                Write-Host "Consent server is ready"
                break
            }
        } catch { }
        Start-Sleep -Seconds 1
        $consentElapsed++
    }
    if ($consentElapsed -ge $consentTimeout) {
        Write-Host "Error: Consent server failed to become ready within ${consentTimeout}s"
        exit 1
    }

    Write-Host "Consent server started (PID: $($script:CONSENT_PROCESS.Id))"
}

# Main script logic
switch ($Command) {
    'clean' {
        Clean
    }
    'build' {
        Build-Backend
        Build-Frontend
        Package
        Build-Sample-App
        Package-Sample-App
    }
    'build_backend' {
        Build-Backend
        Package
    }
    'build_frontend' {
        Build-Frontend
    }
    'build_docs' {
        Build-Docs
    }
    'build_sdks' {
        Build-Sdks
    }
    'test_sdks' {
        Test-Sdks
    }
    'lint_sdks' {
        Lint-Sdks
    }
    'build_samples' {
        Build-Sample-App
    }
    'package_samples' {
        Package-Sample-App
    }
    'test_unit' {
        Test-Unit
    }
    'test_integration' {
        Test-Integration
    }
    'merge_coverage' {
        Merge-Coverage
    }
    'run' {
        Run
    }
    'run_backend' {
        Run-Backend
    }
    'run_frontend' {
        Run-Frontend
    }
    'run_docs' {
        Run-Docs
    }
    'test' {
        Test-Unit
        Test-Integration
    }
    default {
        Write-Host "Usage: $($MyInvocation.MyCommand.Name) {clean|build|build_backend|build_frontend|build_docs|build_samples|package_samples|test_unit|test_integration|merge_coverage|run|run_backend|run_frontend|run_docs|test}"
        exit 1
    }
}
