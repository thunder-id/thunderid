# package-samples.ps1
# Packages all sample apps and verifies the artifacts.
#
# Usage: .\scripts\package-samples.ps1
#
# Exit codes:
#   0 - Success: samples packaged and verified
#   1 - Failure: packaging or verification failed

$ErrorActionPreference = "Stop"

$SCRIPT_DIR = Split-Path -Parent $MyInvocation.MyCommand.Path

Write-Host "📦 Packaging samples..."
& "$SCRIPT_DIR\..\build.ps1" package_samples
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "✅ Packaging complete"

Write-Host ""

Write-Host "🔍 Verifying artifacts..."
& "$SCRIPT_DIR\verify-sample-artifacts.ps1"

Write-Host ""
Write-Host "✅ Sample packaging complete"
