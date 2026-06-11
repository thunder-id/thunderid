# verify-sample-artifacts.ps1
# Verifies that all expected sample app artifacts were created.
#
# Usage: .\scripts\verify-sample-artifacts.ps1
#
# Exit codes:
#   0 - All sample artifacts found
#   1 - One or more artifacts missing

$ErrorActionPreference = "Stop"

Write-Host "📦 Verifying sample artifacts..."

$SAMPLE_APPS = @("react-vanilla", "react-sdk", "react-api-based", "wayfinder")
$MISSING_COUNT = 0

foreach ($app in $SAMPLE_APPS) {
    $EXPECTED_PATTERN = "target/dist/sample-app-${app}-*.zip"
    $MATCHED_FILES = Get-ChildItem -Path "target/dist" -Filter "sample-app-${app}-*.zip" -ErrorAction SilentlyContinue

    if ($MATCHED_FILES.Count -eq 0) {
        Write-Host "❌ Sample artifact not found: $EXPECTED_PATTERN"
        $MISSING_COUNT++
    }
    else {
        Write-Host "✅ Found sample artifact: $($MATCHED_FILES[0].Name)"
    }
}

if ($MISSING_COUNT -gt 0) {
    Write-Host ""
    Write-Host "❌ $MISSING_COUNT sample artifact(s) missing!"
    exit 1
}

Write-Host ""
Write-Host "✅ All sample packages verified"
exit 0
