#!/usr/bin/env pwsh
# ----------------------------------------------------------------------------
# Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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
    [string]$TestPackage
)

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

# React Vanilla Sample
$vanillaPackageJson = Get-Content "samples/apps/react-vanilla-sample/package.json" -Raw | ConvertFrom-Json
$VANILLA_SAMPLE_APP_VERSION = $vanillaPackageJson.version
$VANILLA_SAMPLE_APP_FOLDER = "sample-app-react-vanilla-${VANILLA_SAMPLE_APP_VERSION}"

# React SDK Sample
$reactSdkPackageJson = Get-Content "samples/apps/react-sdk-sample/package.json" -Raw | ConvertFrom-Json
$REACT_SDK_SAMPLE_APP_VERSION = $reactSdkPackageJson.version
$REACT_SDK_SAMPLE_APP_FOLDER = "sample-app-react-sdk-${REACT_SDK_SAMPLE_APP_VERSION}"

# React API-based Sample
$reactApiPackageJson = Get-Content "samples/apps/react-api-based-sample/package.json" -Raw | ConvertFrom-Json
$REACT_API_SAMPLE_APP_VERSION = $reactApiPackageJson.version
$REACT_API_SAMPLE_APP_FOLDER = "sample-app-react-api-based-${REACT_API_SAMPLE_APP_VERSION}"

# Wayfinder Sample
$agentIdPackageJson = Get-Content "samples/apps/wayfinder-sample/package.json" -Raw | ConvertFrom-Json
$WAYFINDER_SAMPLE_APP_VERSION = $agentIdPackageJson.version
$WAYFINDER_SAMPLE_APP_FOLDER = "sample-app-wayfinder-${WAYFINDER_SAMPLE_APP_VERSION}"

# PNPM version to use for frontend builds and docs build
$rootPackageJson = Get-Content "package.json" -Raw | ConvertFrom-Json
$PNPM_VERSION = $rootPackageJson.devEngines.packageManager.version

# Directories
$TARGET_DIR = Join-Path $SCRIPT_DIR "target"
$OUTPUT_DIR = Join-Path $TARGET_DIR "out"
$DIST_DIR = Join-Path $TARGET_DIR "dist"
$BUILD_DIR = Join-Path $OUTPUT_DIR ".build"
$LOCAL_CERT_DIR = Join-Path $OUTPUT_DIR ".cert"
$BACKEND_BASE_DIR = "backend"
$BACKEND_DIR = Join-Path $BACKEND_BASE_DIR "cmd/server"
$REPOSITORY_DB_DIR = Join-Path $BACKEND_DIR "database"
$SERVER_SCRIPTS_DIR = Join-Path $BACKEND_BASE_DIR "scripts"
$SERVER_DB_SCRIPTS_DIR = Join-Path $BACKEND_BASE_DIR "dbscripts"
$SECURITY_DIR = "config/certs"
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
$WAYFINDER_SAMPLE_APP_DIR = Join-Path $SAMPLE_BASE_DIR "apps/wayfinder-sample"

# Quick start declarative bundles staged into the console's welcome feature so they're inlined
# into the console JS bundle at build time.
# Add a new bundle as a single line. Name may include "/" for grouping.
$QUICKSTART_SAMPLE_BUNDLES = @(
    @{ Name = "wayfinder"; Source = (Join-Path $WAYFINDER_SAMPLE_APP_DIR "thunderid-config") }
)
$QUICKSTART_BUNDLE_STAGE_DIR = Join-Path $FRONTEND_CONSOLE_APP_SOURCE_DIR "src/features/welcome/data/sample-bundles"

# Default ports
$GATE_APP_DEFAULT_PORT = 5190
$CONSOLE_APP_DEFAULT_PORT = 5191
$DOCS_DEFAULT_PORT = 3000

# ============================================================================
# Read Configuration from deployment.yaml
# ============================================================================

$CONFIG_FILE = "./backend/cmd/server/deployment.yaml"

# Function to read config with fallback
function Read-Config {
    if (-not (Test-Path $CONFIG_FILE)) {
        # Use defaults if config file not found
        $script:HOSTNAME = "localhost"
        $script:PORT = 8090
        $script:HTTP_ONLY = "false"
        $script:PUBLIC_HOSTNAME = ""
    }
    else {
        # Try yq first (YAML parser)
        if (Get-Command yq -ErrorAction SilentlyContinue) {
            $script:HOSTNAME = & yq eval '.server.hostname // "localhost"' $CONFIG_FILE 2>$null
            $script:PORT = & yq eval '.server.port // 8090' $CONFIG_FILE 2>$null
            $script:HTTP_ONLY = & yq eval '.server.http_only // false' $CONFIG_FILE 2>$null
            $script:PUBLIC_HOSTNAME = & yq eval '.server.public_hostname // ""' $CONFIG_FILE 2>$null
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

    Write-Host "Removing runtime secrets in $BACKEND_DIR/config/secrets"
    if (Test-Path (Join-Path $BACKEND_DIR "config/secrets")) {
        Remove-Item -Path (Join-Path $BACKEND_DIR "config/secrets") -Recurse -Force -ErrorAction SilentlyContinue
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
        & npm install -g "pnpm@$PNPM_VERSION"
    }
}

function Build-Frontend {
    Write-Host "================================================================"
    Write-Host "Building frontend apps..."
    Ensure-Pnpm

    Sync-QuickstartBundles

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

function Build-CLI {
    Write-Host "Building CLI tool..."
    & bash "$PSScriptRoot/tools/cli/scripts/build.sh"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

function Test-CLI {
    Write-Host "Running CLI tool tests..."
    Push-Location "$PSScriptRoot/tools/cli"
    try {
        & go test -v -race -count=1 ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally {
        Pop-Location
    }
}

function Build-I18n-Extractor {
    $toolBin = Join-Path $PSScriptRoot "backend/bin/tools"
    New-Item -ItemType Directory -Force -Path $toolBin | Out-Null
    Write-Host "Building i18n-extractor..."
    Push-Location "$PSScriptRoot/tools/i18n-extractor"
    try {
        & go build -o "$toolBin/i18n-extractor.exe" .
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally {
        Pop-Location
    }
}

function Test-I18n-Extractor {
    Write-Host "Running i18n-extractor tests..."
    Push-Location "$PSScriptRoot/tools/i18n-extractor"
    try {
        & go test -v .
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally {
        Pop-Location
    }
}

function Lint-CLI {
    $golangciLint = Join-Path $PSScriptRoot "backend/bin/tools/golangci-lint.exe"
    Write-Host "Linting CLI tool..."
    Push-Location "$PSScriptRoot/tools/cli"
    try {
        & $golangciLint run ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally {
        Pop-Location
    }
}

function Lint-I18n-Extractor {
    $golangciLint = Join-Path $PSScriptRoot "backend/bin/tools/golangci-lint.exe"
    Write-Host "Linting i18n-extractor..."
    Push-Location "$PSScriptRoot/tools/i18n-extractor"
    try {
        & $golangciLint run ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally {
        Pop-Location
    }
}

function Lint-Tools {
    Write-Host "================================================================"
    Write-Host "Linting tools..."
    Lint-CLI
    Lint-I18n-Extractor
    Write-Host "================================================================"
}

function Build-Npm-Tools {
    Ensure-Pnpm
    Write-Host "Installing tools dependencies..."
    & pnpm install --frozen-lockfile
    Write-Host "Building npm-based tools..."
    & pnpm --filter './tools/**' build
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

function Build-Tools {
    Write-Host "================================================================"
    Write-Host "Building tools..."
    Build-CLI
    Build-I18n-Extractor
    Build-Npm-Tools
    Write-Host "================================================================"
}

function Test-Tools {
    Write-Host "================================================================"
    Write-Host "Running tool tests..."
    Test-CLI
    Test-I18n-Extractor
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

    $db_files = @("configdb.db", "runtime_transient.db", "entitydb.db", "runtime_persistent.db")
    $script_paths = @("configdb/sqlite.sql", "runtime_transient/sqlite.sql", "entitydb/sqlite.sql", "runtime_persistent/sqlite.sql")

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
    Copy-Item -Path (Join-Path $BACKEND_DIR "deployment.yaml") -Destination $package_folder -Force
    Copy-Item -Path (Join-Path $BACKEND_DIR "config") -Destination $package_folder -Recurse -Force
    if (Test-Path $REPOSITORY_DB_DIR) {
        Copy-Item -Path $REPOSITORY_DB_DIR -Destination $package_folder -Recurse -Force
    }
    Copy-Item -Path $VERSION_FILE -Destination $package_folder -Force
    Copy-Item -Path $SERVER_SCRIPTS_DIR -Destination $package_folder -Recurse -Force
    Copy-Item -Path $SERVER_DB_SCRIPTS_DIR -Destination $package_folder -Recurse -Force
    
    $security_dir = Join-Path $package_folder $SECURITY_DIR
    New-Item -Path $security_dir -ItemType Directory -Force | Out-Null
    # Never ship key material: strip any dev certs/keys that copying config above may have brought in.
    Remove-Item -Path (Join-Path $security_dir "*.cert") -Force -ErrorAction SilentlyContinue
    Remove-Item -Path (Join-Path $security_dir "*.key") -Force -ErrorAction SilentlyContinue
    # Never ship runtime secrets: strip the dev Direct Auth Secret that copying config may have brought in.
    # setup.ps1 generates a fresh per-deployment secret.
    Remove-Item -Path (Join-Path $package_folder "config/secrets") -Recurse -Force -ErrorAction SilentlyContinue

    # Copy bootstrap directory
    Write-Host "Copying bootstrap scripts..."
    Copy-Item -Path (Join-Path $BACKEND_DIR "bootstrap") -Destination $package_folder -Recurse -Force
    # Never ship the dev-only CORS seed that Run stages into the source bootstrap dir.
    Remove-Item -Path (Join-Path $package_folder "bootstrap/02-server-configurations.yaml") -Force -ErrorAction SilentlyContinue

    # Key material is not generated into the distribution; setup.ps1 generates it per deployment.
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

function Sync-QuickstartBundles {
    # Stage Quick start declarative bundles into the console's public dir.
    Write-Host "Syncing quick start sample bundles to console welcome data dir..."
    if (Test-Path $QUICKSTART_BUNDLE_STAGE_DIR) {
        Remove-Item -Path $QUICKSTART_BUNDLE_STAGE_DIR -Recurse -Force
    }
    foreach ($bundle in $QUICKSTART_SAMPLE_BUNDLES) {
        $dest_dir = Join-Path $QUICKSTART_BUNDLE_STAGE_DIR $bundle.Name
        if (Test-Path $bundle.Source) {
            Write-Host "  Staging '$($bundle.Name)' from $($bundle.Source)"
            New-Item -Path $dest_dir -ItemType Directory -Force | Out-Null
            Copy-Item -Path (Join-Path $bundle.Source "*") -Destination $dest_dir -Recurse -Force
        }
        else {
            Write-Host "  Warning: Quick start bundle source not found at $($bundle.Source) (dest '$($bundle.Name)')"
        }
    }
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

function Package-Sample-App {
    Write-Host "================================================================"
    Write-Host "Packaging sample apps..."
    Ensure-Pnpm

    # pnpm pack rewrites workspace: dependencies to real versions, which
    # requires the workspace to be installed.
    Write-Host "Installing workspace dependencies..."
    & pnpm install --frozen-lockfile
    if ($LASTEXITCODE -ne 0) {
        throw "pnpm install failed with exit code $LASTEXITCODE"
    }

    # Samples are packaged from source; ship certificates for the samples that
    # expect them at the package root (react-api-based ignores them via .gitignore).
    Write-Host "=== Ensuring sample app certificates exist ==="
    Ensure-Certificates -cert_dir $VANILLA_SAMPLE_APP_DIR -cert_name_prefix "server"
    Ensure-Certificates -cert_dir $REACT_SDK_SAMPLE_APP_DIR -cert_name_prefix "server"

    # Package React Vanilla sample
    Write-Host "=== Packaging React Vanilla sample app ==="
    Package-Vanilla-Sample

    # Package React SDK sample
    Write-Host "=== Packaging React SDK sample app ==="
    Package-React-SDK-Sample

    # Package React API-based sample
    Write-Host "=== Packaging React API-based sample app ==="
    Package-React-API-Based-Sample

    # Package Wayfinder sample
    Write-Host "=== Packaging Wayfinder sample app ==="
    Package-Wayfinder-Sample

    Write-Host "================================================================"
}

function Package-Vanilla-Sample {
    Push-Location $VANILLA_SAMPLE_APP_DIR
    & pnpm pack --pack-destination (Resolve-Path $DIST_DIR).Path
    Pop-Location

    $tgz = Get-ChildItem -Path $DIST_DIR -Filter "thunderid-react-vanilla-sample-*.tgz" | Select-Object -First 1
    if (-not $tgz) {
        throw "pnpm pack did not produce a tgz for react-vanilla-sample"
    }

    tar xzf $tgz.FullName -C $DIST_DIR
    Rename-Item -Path (Join-Path $DIST_DIR "package") -NewName $VANILLA_SAMPLE_APP_FOLDER

    Write-Host "Creating React Vanilla sample zip file..."
    $distAbs = (Resolve-Path -Path $DIST_DIR).Path
    $zipFile = [System.IO.Path]::Combine($distAbs, "$VANILLA_SAMPLE_APP_FOLDER.zip")
    if (Test-Path $zipFile) {
        Remove-Item $zipFile -Force
    }

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory((Join-Path $DIST_DIR $VANILLA_SAMPLE_APP_FOLDER), $zipFile)
    Remove-Item -Path (Join-Path $DIST_DIR $VANILLA_SAMPLE_APP_FOLDER) -Recurse -Force
    Remove-Item -Path $tgz.FullName -Force

    Write-Host "✅ React Vanilla sample app packaged successfully as $zipFile"
}

function Package-React-SDK-Sample {
    Push-Location $REACT_SDK_SAMPLE_APP_DIR
    & pnpm pack --pack-destination (Resolve-Path $DIST_DIR).Path
    Pop-Location

    $tgz = Get-ChildItem -Path $DIST_DIR -Filter "thunderid-react-sdk-sample-*.tgz" | Select-Object -First 1
    if (-not $tgz) {
        throw "pnpm pack did not produce a tgz for react-sdk-sample"
    }

    tar xzf $tgz.FullName -C $DIST_DIR
    Rename-Item -Path (Join-Path $DIST_DIR "package") -NewName $REACT_SDK_SAMPLE_APP_FOLDER

    Write-Host "Creating React SDK sample zip file..."
    $distAbs = (Resolve-Path -Path $DIST_DIR).Path
    $zipFile = [System.IO.Path]::Combine($distAbs, "$REACT_SDK_SAMPLE_APP_FOLDER.zip")
    if (Test-Path $zipFile) {
        Remove-Item $zipFile -Force
    }

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory((Join-Path $DIST_DIR $REACT_SDK_SAMPLE_APP_FOLDER), $zipFile)
    Remove-Item -Path (Join-Path $DIST_DIR $REACT_SDK_SAMPLE_APP_FOLDER) -Recurse -Force
    Remove-Item -Path $tgz.FullName -Force

    Write-Host "✅ React SDK sample app packaged successfully as $zipFile"
}

function Package-React-API-Based-Sample {
    Push-Location $REACT_API_SAMPLE_APP_DIR
    & pnpm pack --pack-destination (Resolve-Path $DIST_DIR).Path
    Pop-Location

    $tgz = Get-ChildItem -Path $DIST_DIR -Filter "thunderid-react-api-based-sample-*.tgz" | Select-Object -First 1
    if (-not $tgz) {
        throw "pnpm pack did not produce a tgz for react-api-based-sample"
    }

    tar xzf $tgz.FullName -C $DIST_DIR
    Rename-Item -Path (Join-Path $DIST_DIR "package") -NewName $REACT_API_SAMPLE_APP_FOLDER

    # Certs are gitignored in this sample so pnpm pack excludes them; inject them
    # into the package root where vite.config.ts expects them.
    Ensure-Certificates -cert_dir (Join-Path $DIST_DIR $REACT_API_SAMPLE_APP_FOLDER) -cert_name_prefix "server"

    Write-Host "Creating React API-based sample zip file..."
    $distAbs = (Resolve-Path -Path $DIST_DIR).Path
    $zipFile = [System.IO.Path]::Combine($distAbs, "$REACT_API_SAMPLE_APP_FOLDER.zip")
    if (Test-Path $zipFile) {
        Remove-Item $zipFile -Force
    }

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory((Join-Path $DIST_DIR $REACT_API_SAMPLE_APP_FOLDER), $zipFile)
    Remove-Item -Path (Join-Path $DIST_DIR $REACT_API_SAMPLE_APP_FOLDER) -Recurse -Force
    Remove-Item -Path $tgz.FullName -Force

    Write-Host "✅ React API-based sample app packaged successfully as $zipFile"
}

function Package-Wayfinder-Sample {
    Push-Location $WAYFINDER_SAMPLE_APP_DIR
    & pnpm pack --pack-destination (Resolve-Path $DIST_DIR).Path
    Pop-Location

    $tgz = Get-ChildItem -Path $DIST_DIR -Filter "thunderid-wayfinder-sample-*.tgz" | Select-Object -First 1
    if (-not $tgz) {
        throw "pnpm pack did not produce a tgz for wayfinder-sample"
    }

    tar xzf $tgz.FullName -C $DIST_DIR
    Rename-Item -Path (Join-Path $DIST_DIR "package") -NewName $WAYFINDER_SAMPLE_APP_FOLDER

    $dist_folder = Join-Path $DIST_DIR $WAYFINDER_SAMPLE_APP_FOLDER

    foreach ($dir in @("frontend", "backend", "smtp-server", "ai-agent")) {
        $envExample = Join-Path $dist_folder "$dir/.env.example"
        if (Test-Path $envExample) {
            Copy-Item -Path $envExample -Destination (Join-Path $dist_folder "$dir/.env") -Force
        }
    }

    Write-Host "Creating Wayfinder sample zip file..."
    $distAbs = (Resolve-Path -Path $DIST_DIR).Path
    $zipFile = [System.IO.Path]::Combine($distAbs, "$WAYFINDER_SAMPLE_APP_FOLDER.zip")
    if (Test-Path $zipFile) {
        Remove-Item $zipFile -Force
    }

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory($dist_folder, $zipFile)
    Remove-Item -Path $dist_folder -Recurse -Force
    Remove-Item -Path $tgz.FullName -Force

    Write-Host "✅ Wayfinder sample app packaged successfully as $zipFile"
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
        $coverage_dir = [System.IO.Path]::GetFullPath((Join-Path $SCRIPT_DIR "target\out\.test\integration"))
        if (Test-Path $coverage_dir) {
            Remove-Item -Path $coverage_dir -Recurse -Force -ErrorAction SilentlyContinue
        }
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

            # Formulate robust absolute target paths to keep Windows volume prefixes intact
            $output_file = [System.IO.Path]::GetFullPath((Join-Path $SCRIPT_DIR "target\coverage_integration.out"))
            $output_html = [System.IO.Path]::GetFullPath((Join-Path $SCRIPT_DIR "target\coverage_integration.html"))

            # Convert binary coverage data to text format cleanly
            Push-Location $BACKEND_BASE_DIR
            try {
                & go tool covdata textfmt -i="$coverage_dir" -o="$output_file"
                Write-Host "Integration test coverage report generated in: $output_file"

                # Generate HTML coverage report
                & go tool cover -html="$output_file" -o="$output_html"
                Write-Host "Integration test coverage HTML report generated in: $output_html"

                # Display coverage summary
                Write-Host ""
                Write-Host "================================================================"
                Write-Host "Coverage Summary:"
                & go tool cover -func="$output_file" | Select-Object -Last 1
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
        [System.Security.Cryptography.AsymmetricAlgorithm]$privateKey = $null
    )
    # Export cert to PEM
    $rawCert = $cert.Export([System.Security.Cryptography.X509Certificates.X509ContentType]::Cert)
    $certBase64 = [System.Convert]::ToBase64String($rawCert)
    $certLines = $certBase64 -split '(.{64})' | Where-Object { $_ -ne '' }
    $certPem = "-----BEGIN CERTIFICATE-----`n" + ($certLines -join "`n") + "`n-----END CERTIFICATE-----`n"
    Set-Content -Path $certPath -Value $certPem -Encoding ascii

    # Obtain private key. If a privateKey instance was provided by the caller use it
    # (this avoids relying on PFX export/import semantics which can vary across runtimes).
    $keyAlg = $null
    $reloadCert = $null
    try {
        if ($null -ne $privateKey) {
            $keyAlg = $privateKey
        }
        else {
            # Export as PFX and reload with Exportable flag so we can export the private key
            $pfxBytes = $cert.Export([System.Security.Cryptography.X509Certificates.X509ContentType]::Pfx, '')
            $reloadCert = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new($pfxBytes, '', [System.Security.Cryptography.X509Certificates.X509KeyStorageFlags]::Exportable)

            # Try modern APIs
            try { $keyAlg = $reloadCert.GetRSAPrivateKey() } catch { $keyAlg = $null }
            if (-not $keyAlg) {
                try { $keyAlg = $reloadCert.GetECDsaPrivateKey() } catch { $keyAlg = $null }
            }

            # Fallback for RSA if modern API fails
            if (-not $keyAlg -and $null -ne $reloadCert.PrivateKey) {
                try {
                    $pk = $reloadCert.PrivateKey
                    $rsaFallback = [System.Security.Cryptography.RSA]::Create()
                    $rsaFallback.ImportParameters($pk.ExportParameters($true))
                    $keyAlg = $rsaFallback
                }
                catch {
                    if ($rsaFallback -is [System.IDisposable]) { $rsaFallback.Dispose() }
                    $keyAlg = $null
                }
            }
        }

        if (-not $keyAlg) { throw "Certificate does not contain an exportable private key" }

        # Export private key to PEM (PKCS#8)
        $pkcs8 = $keyAlg.ExportPkcs8PrivateKey()
        $keyBase64 = [System.Convert]::ToBase64String($pkcs8)
        $pkcs8Lines = $keyBase64 -split '(.{64})' | Where-Object { $_ -ne '' }
        $keyPem = "-----BEGIN PRIVATE KEY-----`n" + ($pkcs8Lines -join "`n") + "`n-----END PRIVATE KEY-----`n"
        Set-Content -Path $keyPath -Value $keyPem -Encoding ascii
    }
    finally {
        # Only dispose keyAlg if we created it locally (i.e., privateKey was not passed in)
        if ($null -eq $privateKey) {
            if ($keyAlg -is [System.IDisposable]) { $keyAlg.Dispose() }
            if ($reloadCert -is [System.IDisposable]) { $reloadCert.Dispose() }
        }
    }
}

function Ensure-Certificates {
    param(
        [string]$cert_dir,
        [string]$cert_name_prefix = "server",
        [string]$Algorithm = "RSA"
    )
    
    $cert_file_name = "${cert_name_prefix}.cert"
    $key_file_name = "${cert_name_prefix}.key"

    # Generate certificate and key file if they don't exist in the cert directory
    $local_cert_file = Join-Path $LOCAL_CERT_DIR $cert_file_name
    $local_key_file = Join-Path $LOCAL_CERT_DIR $key_file_name
    
    if (-not (Test-Path $local_cert_file) -or -not (Test-Path $local_key_file)) {
        New-Item -Path $LOCAL_CERT_DIR -ItemType Directory -Force | Out-Null
        
        Write-Host "Generating certificates ($cert_name_prefix) in $LOCAL_CERT_DIR using $Algorithm..."
        
        try {
            $openssl = Get-Command openssl -ErrorAction SilentlyContinue
            if ($openssl) {
                if ($Algorithm -eq "ECDSA") {
                    & openssl ecparam -name prime256v1 -genkey -noout -param_enc named_curve -out $local_key_file 2>$null
                    if ($LASTEXITCODE -ne 0) { throw "Error generating EC key: OpenSSL failed with exit code $LASTEXITCODE" }
                    & openssl req -new -x509 -nodes -key $local_key_file -out $local_cert_file -days 3650 -subj "/O=WSO2/OU=$PRODUCT_NAME/CN=localhost" -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" 2>$null
                }
                else {
                    & openssl req -x509 -nodes -days 365 -newkey rsa:2048 `
                        -keyout $local_key_file `
                        -out $local_cert_file `
                        -subj "/O=WSO2/OU=$PRODUCT_NAME/CN=localhost" `
                        -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" 2>$null
                }
                if ($LASTEXITCODE -ne 0) {
                    throw "Error generating certificates: OpenSSL failed with exit code $LASTEXITCODE"
                }
                Write-Host "Certificates generated successfully in $LOCAL_CERT_DIR using OpenSSL."
            }
            else {
                Write-Host "OpenSSL not found - generating certificates using .NET CertificateRequest (no UI)."
                # Use .NET CertificateRequest to avoid CertEnroll / smartcard enrollment UI issues.
                try {
                    $keyAlg = $null
                    $certReq = $null
                    $subjectName = New-Object System.Security.Cryptography.X509Certificates.X500DistinguishedName("CN=localhost, O=WSO2, OU=$PRODUCT_NAME")

                    if ($Algorithm -eq "ECDSA") {
                        $keyAlg = [System.Security.Cryptography.ECDsa]::Create([System.Security.Cryptography.ECCurve+NamedCurves]::nistP256)
                        $certReq = New-Object System.Security.Cryptography.X509Certificates.CertificateRequest($subjectName, $keyAlg, [System.Security.Cryptography.HashAlgorithmName]::SHA256)
                    } else {
                        $keyAlg = [System.Security.Cryptography.RSA]::Create(2048)
                        $certReq = New-Object System.Security.Cryptography.X509Certificates.CertificateRequest($subjectName, $keyAlg, [System.Security.Cryptography.HashAlgorithmName]::SHA256, [System.Security.Cryptography.RSASignaturePadding]::Pkcs1)
                    }

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

                    $sanBuilder = New-Object System.Security.Cryptography.X509Certificates.SubjectAlternativeNameBuilder
                    $sanBuilder.AddDnsName("localhost")
                    $certReq.CertificateExtensions.Add($sanBuilder.Build())

                    $notBefore = (Get-Date).AddDays(-1)
                    $notAfter = (Get-Date).AddYears(1)

                    $cert = $certReq.CreateSelfSigned($notBefore, $notAfter)

                    # Ensure the generated certificate has the private key associated. Use CopyWithPrivateKey
                    # so that when we export the PFX it includes the private key and can be reloaded as exportable.
                    try {
                        if ($Algorithm -eq "ECDSA") {
                            $certWithKey = [System.Security.Cryptography.X509Certificates.ECDsaCertificateExtensions]::CopyWithPrivateKey($cert, $keyAlg)
                        } else {
                            $certWithKey = [System.Security.Cryptography.X509Certificates.RSACertificateExtensions]::CopyWithPrivateKey($cert, $keyAlg)
                        }
                    }
                    catch {
                        try {
                            $cert2 = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new($cert.RawData)
                            if ($Algorithm -eq "ECDSA") {
                                $certWithKey = [System.Security.Cryptography.X509Certificates.ECDsaCertificateExtensions]::CopyWithPrivateKey($cert2, $keyAlg)
                            } else {
                                $certWithKey = [System.Security.Cryptography.X509Certificates.RSACertificateExtensions]::CopyWithPrivateKey($cert2, $keyAlg)
                            }
                        }
                        catch {
                            throw "Failed to associate private key with certificate: $_"
                        }
                    }

                    # Export and reload as exportable so we can extract the private key bytes
                    $pfxBytes = $certWithKey.Export([System.Security.Cryptography.X509Certificates.X509ContentType]::Pfx, '')
                    $exportableCert = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new($pfxBytes, '', [System.Security.Cryptography.X509Certificates.X509KeyStorageFlags]::Exportable)

                    # Pass the algorithm instance used to sign the certificate to the exporter so it
                    # can directly export the private key (avoids re-import issues on some runtimes).
                    Export-CertificateAndKeyToPem -cert $exportableCert -certPath $local_cert_file -keyPath $local_key_file -privateKey $keyAlg

                    if ($exportableCert -is [System.IDisposable]) { $exportableCert.Dispose() }
                    if ($certWithKey -is [System.IDisposable]) { $certWithKey.Dispose() }
                    if ($cert -is [System.IDisposable]) { $cert.Dispose() }
                    if ($keyAlg -is [System.IDisposable]) { $keyAlg.Dispose() }

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
        [string]$key_dir
    )

    $KEY_DIR = $key_dir
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

function Ensure-DirectAuthSecret-File {
    param(
        [string]$secret_dir
    )

    # Path referenced by server.security.direct_auth_secret (file://config/secrets/direct_auth_secret)
    # in deployment.yaml. The server reads the secret from here at load time.
    $SECRET_FILE = Join-Path $secret_dir "direct_auth_secret"

    Write-Host "================================================================"
    Write-Host "Ensuring Direct Auth Secret file exists..."

    if (Test-Path $SECRET_FILE) {
        Write-Host "Direct Auth Secret file already present in $SECRET_FILE. Skipping generation."
    }
    else {
        Write-Host "Direct Auth Secret file not found. Generating new secret at $SECRET_FILE..."
        $NEW_SECRET = $null

        # Prefer OpenSSL; fall back to .NET cryptography (always available in PowerShell).
        $openssl = Get-Command openssl -ErrorAction SilentlyContinue
        if ($openssl) {
            try {
                $NEW_SECRET = (openssl rand -hex 32 | Out-String).Trim()
                if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrEmpty($NEW_SECRET) -or $NEW_SECRET.Length -ne 64) {
                    throw "OpenSSL rand command failed or returned empty/incorrect length."
                }
            }
            catch {
                Write-Host " - OpenSSL failed: $_. Falling back to .NET cryptography."
                $NEW_SECRET = $null
            }
        }

        if ([string]::IsNullOrEmpty($NEW_SECRET)) {
            $bytes = New-Object byte[] 32
            $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
            $rng.GetBytes($bytes)
            $rng.Dispose()
            $NEW_SECRET = ([System.BitConverter]::ToString($bytes) -replace '-', '').ToLower()
        }

        # Ensure the target directory exists.
        New-Item -Path $secret_dir -ItemType Directory -Force | Out-Null

        # Write the secret without a trailing newline so it is used verbatim.
        Set-Content -Path $SECRET_FILE -Value $NEW_SECRET -NoNewline -Encoding Ascii

        Write-Host "Successfully generated and added new Direct Auth Secret to $SECRET_FILE."
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
        Start-Sleep -Seconds 1
        Write-Host "✅ All servers stopped successfully."
    }

    Write-Host "Running frontend apps..."
    Run-Frontend

    # Ensure runtime prerequisites (certificates, crypto material, databases) so the
    # in-process bootstrap can create the default resources before the server starts.
    Write-Host "=== Ensuring server certificates exist ==="
    Ensure-Certificates -cert_dir (Join-Path $BACKEND_DIR $SECURITY_DIR) -cert_name_prefix "server"
    Ensure-Certificates -cert_dir (Join-Path $BACKEND_DIR $SECURITY_DIR) -cert_name_prefix "signing"
    Ensure-Certificates -cert_dir (Join-Path $BACKEND_DIR $SECURITY_DIR) -cert_name_prefix "ecdsa-signing" -Algorithm "ECDSA"
    Ensure-Certificates -cert_dir $VANILLA_SAMPLE_APP_DIR -cert_name_prefix "server"
    Ensure-Crypto-File -key_dir (Join-Path $BACKEND_DIR "config/certs")
    Ensure-DirectAuthSecret-File -secret_dir (Join-Path $BACKEND_DIR "config/secrets")
    Write-Host "Initializing databases..."
    Initialize-Databases

    # Create default resources via the in-process bootstrap one-shot (security stays
    # enabled; the bootstrap runs through the service layer under a runtime context).
    Write-Host "⚙️  Creating default resources..."
    Write-Host ""

    # Dev-only: seed CORS allowed origins for the Gate and Console apps so they can call
    # the backend without manual configuration. Regenerated on every run and picked up by
    # the bootstrap one-shot; it is git-ignored and never packaged (see Build).
    $devServerConfig = @"
resource_type: server_config
name: cors
value:
  allowedOrigins:
    - "https://localhost:$GATE_APP_DEFAULT_PORT"
    - "https://localhost:$CONSOLE_APP_DEFAULT_PORT"
"@
    Set-Content -Path (Join-Path $BACKEND_DIR "bootstrap/02-server-configurations.yaml") -Value $devServerConfig

    # Local dev only: default to admin/admin if not supplied. This path never produces a
    # shared or distributed artifact, so a fixed default here is acceptable.
    $env:PUBLIC_URL = $PUBLIC_URL
    $env:ADMIN_USERNAME = if ($env:ADMIN_USERNAME) { $env:ADMIN_USERNAME } else { "admin" }
    $env:ADMIN_PASSWORD = if ($env:ADMIN_PASSWORD) { $env:ADMIN_PASSWORD } else { "admin" }
    Push-Location $BACKEND_DIR
    try {
        & go run . bootstrap --console-redirect-uris "https://localhost:${CONSOLE_APP_DEFAULT_PORT}/console"
        $bootstrapExit = $LASTEXITCODE
    }
    finally {
        Pop-Location
    }
    if ($bootstrapExit -ne 0) {
        Write-Host "❌ Initial data setup failed"
        Write-Host "💡 Check the logs above for more details"
        exit 1
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
    Ensure-Certificates -cert_dir (Join-Path $BACKEND_DIR $SECURITY_DIR) -cert_name_prefix "ecdsa-signing" -Algorithm "ECDSA"

    Write-Host "=== Ensuring React Vanilla sample app certificates exist ==="
    Ensure-Certificates -cert_dir $VANILLA_SAMPLE_APP_DIR -cert_name_prefix "server"

    Write-Host "=== Ensuring crypto file exists for run ==="
    Ensure-Crypto-File -key_dir (Join-Path $BACKEND_DIR "config/certs")
    Ensure-DirectAuthSecret-File -secret_dir (Join-Path $BACKEND_DIR "config/secrets")

    Write-Host "Initializing databases..."
    Initialize-Databases

    Start-Backend -ShowFinalOutput $ShowFinalOutput
}

 # Kill processes on known ports
function Kill-Port {
    param([int]$port)
    
    $processes = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess
    foreach ($process in $processes) {
        Stop-Process -Id $process -Force -ErrorAction SilentlyContinue
    }
}

function Start-Backend {
    param(
        [bool]$ShowFinalOutput = $true
    )

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

    Sync-QuickstartBundles

    # Install dependencies
    try {
        Write-Host "Installing frontend dependencies..."
        & pnpm install --frozen-lockfile
        
        Write-Host "Building frontend applications & packages..."
        & pnpm build:frontend
        
        Write-Host "Starting frontend applications in the background..."
        # In dev the apps are served on their own origins, so point them at the backend via
        # THUNDERID_DEV_SERVER_URL (injected into __DEV_SERVER_URL__; applied only in dev builds).
        $env:THUNDERID_DEV_SERVER_URL = $PUBLIC_URL
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

# Main script logic
switch ($Command) {
    'clean' {
        Clean
    }
    'build' {
        Build-Backend
        Build-Frontend
        Package
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
    'build_tools' {
        Build-Tools
    }
    'test_tools' {
        Test-Tools
    }
    'lint_tools' {
        Lint-Tools
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
        Write-Host "Usage: $($MyInvocation.MyCommand.Name) {clean|build|build_backend|build_frontend|build_docs|build_tools|test_tools|lint_tools|package_samples|test_unit|test_integration|merge_coverage|run|run_backend|run_frontend|run_docs|test}"
        exit 1
    }
}
