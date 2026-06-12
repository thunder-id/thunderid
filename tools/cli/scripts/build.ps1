# ----------------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

# =============================================================================
# CLI Build Script (Windows)
#
# Cross-compiles the CLI for all supported platforms.
# Output binaries are written to dist/ inside the cli directory.
# Mirrors build.sh for Windows developers.
# =============================================================================

$ErrorActionPreference = "Stop"

$PRODUCT_NAME           = "ThunderID"
$PRODUCT_NAME_LOWERCASE = $PRODUCT_NAME.ToLower()
$CLI_DIR                = Join-Path $PSScriptRoot ".."
$DIST_DIR               = Join-Path $CLI_DIR "dist"

New-Item -ItemType Directory -Force -Path $DIST_DIR | Out-Null

$TARGETS = @(
    @{ GOOS = "darwin";  GOARCH = "amd64"; OUT = "${PRODUCT_NAME_LOWERCASE}-darwin-x64"    },
    @{ GOOS = "darwin";  GOARCH = "arm64"; OUT = "${PRODUCT_NAME_LOWERCASE}-darwin-arm64"  },
    @{ GOOS = "linux";   GOARCH = "amd64"; OUT = "${PRODUCT_NAME_LOWERCASE}-linux-x64"     },
    @{ GOOS = "linux";   GOARCH = "arm64"; OUT = "${PRODUCT_NAME_LOWERCASE}-linux-arm64"   },
    @{ GOOS = "windows"; GOARCH = "amd64"; OUT = "${PRODUCT_NAME_LOWERCASE}-win-x64.exe"   }
)

Push-Location $CLI_DIR
try {
    foreach ($t in $TARGETS) {
        $outPath = Join-Path $DIST_DIR $t.OUT
        Write-Host "Building $($t.GOOS)/$($t.GOARCH) -> $($t.OUT)"
        $env:GOOS        = $t.GOOS
        $env:GOARCH      = $t.GOARCH
        $env:CGO_ENABLED = "0"
        go build -ldflags="-s -w" -o $outPath "./cmd/${PRODUCT_NAME_LOWERCASE}/"
    }
} finally {
    "GOOS", "GOARCH", "CGO_ENABLED" | ForEach-Object { Remove-Item "Env:\$_" -ErrorAction SilentlyContinue }
    Pop-Location
}

Write-Host "Done. Binaries written to cli/dist/"
