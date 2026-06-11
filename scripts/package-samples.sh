#!/bin/bash
# package-samples.sh
# Builds and packages all sample apps, then verifies the artifacts.
#
# Usage: ./scripts/package-samples.sh
#
# Exit codes:
#   0 - Success: samples built, packaged, and verified
#   1 - Failure: build failed or verification failed

set -e

echo "📦 Packaging samples..."
make package_samples
echo "✅ Packaging complete"

echo ""

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Verify artifacts were created
echo "🔍 Verifying artifacts..."
"$SCRIPT_DIR/verify-sample-artifacts.sh"

echo ""
echo "✅ Sample packaging complete"
exit 0
