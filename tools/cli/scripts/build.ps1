# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

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

# Version reported by `--version`. Supplied by the npx build wrapper from
# package.json; falls back to the unreleased placeholder for direct local builds.
$VERSION = if ($env:VERSION) { $env:VERSION } else { "0.0.0-semantically-released" }

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
        go build -ldflags="-s -w -X main.version=$VERSION" -o $outPath "./cmd/${PRODUCT_NAME_LOWERCASE}/"
    }
} finally {
    "GOOS", "GOARCH", "CGO_ENABLED" | ForEach-Object { Remove-Item "Env:\$_" -ErrorAction SilentlyContinue }
    Pop-Location
}

Write-Host "Done. Binaries written to cli/dist/"
