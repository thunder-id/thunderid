#!/bin/bash
# verify-sample-artifacts.sh
# Verifies that all expected sample app artifacts were created.
#
# Usage: ./scripts/verify-sample-artifacts.sh
#
# Exit codes:
#   0 - All sample artifacts found
#   1 - One or more artifacts missing

set -e

echo "📦 Verifying sample artifacts..."

# Define expected sample apps
SAMPLE_APPS=("react-vanilla" "react-sdk" "react-api-based" "wayfinder")

# Track any missing artifacts
MISSING_COUNT=0

for app in "${SAMPLE_APPS[@]}"; do
  EXPECTED_PATTERN="target/dist/sample-app-${app}-*.zip"

  # Expand glob once into an array (nullglob ensures empty array if no match)
  shopt -s nullglob
  # shellcheck disable=SC2206
  MATCHED_FILES=($EXPECTED_PATTERN)
  shopt -u nullglob

  if [ ${#MATCHED_FILES[@]} -eq 0 ]; then
    echo "❌ Sample artifact not found: $EXPECTED_PATTERN"
    MISSING_COUNT=$((MISSING_COUNT + 1))
  else
    echo "✅ Found sample artifact: $(basename "${MATCHED_FILES[0]}")"
  fi
done

if [ $MISSING_COUNT -gt 0 ]; then
  echo ""
  echo "❌ $MISSING_COUNT sample artifact(s) missing!"
  exit 1
fi

echo ""
echo "✅ All sample packages verified"
exit 0
