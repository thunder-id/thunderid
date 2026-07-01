#!/usr/bin/env bash
# apply-branch-protection.sh
# Applies governance/branch-protection.json to the protected branch.
# Requires: gh (authenticated as an admin), jq.
# Usage: REPO=org/thunderid BRANCH=main ./apply-branch-protection.sh
set -euo pipefail
REPO="${REPO:?set REPO=org/repo}"
BRANCH="${BRANCH:-main}"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

gh api -X PUT "repos/${REPO}/branches/${BRANCH}/protection" \
  --input "${DIR}/governance/branch-protection.json"

echo "\u2713 Applied branch protection to ${REPO}@${BRANCH}."
