#!/usr/bin/env bash
# check-branch-protection.sh
#
# "Guard the guard." Asserts that branch protection on the protected branch has
# not been quietly weakened. Run on a schedule (see governance-drift.yml) and on
# demand. Exits non-zero on drift so the scheduled run fails loudly.
#
# Requires: gh (authenticated), jq.
# Usage: REPO=org/thunderid BRANCH=main REQUIRED_CHECKS="api-lint,codegen-drift,breaking-changes,contract-tests,coverage-meta,exemptions" ./check-branch-protection.sh

set -euo pipefail

REPO="${REPO:?set REPO=org/repo}"
BRANCH="${BRANCH:-main}"
REQUIRED_CHECKS="${REQUIRED_CHECKS:-api-lint,codegen-drift,breaking-changes,contract-tests,coverage-meta,exemptions}"

prot="$(gh api "repos/${REPO}/branches/${BRANCH}/protection")"
fail=0
note() { echo "  \u2717 $1"; fail=1; }

# enforce on admins (no bypass)
if [[ "$(jq -r '.enforce_admins.enabled' <<<"$prot")" != "true" ]]; then
  note "enforce_admins is not enabled (admins can bypass)."
fi

# require code-owner review
if [[ "$(jq -r '.required_pull_request_reviews.require_code_owner_reviews' <<<"$prot")" != "true" ]]; then
  note "require_code_owner_reviews is not enabled."
fi

# at least one approval
approvals="$(jq -r '.required_pull_request_reviews.required_approving_review_count // 0' <<<"$prot")"
if (( approvals < 1 )); then
  note "required_approving_review_count is ${approvals} (< 1)."
fi

# linear history (merge queue friendly)
if [[ "$(jq -r '.required_linear_history.enabled' <<<"$prot")" != "true" ]]; then
  note "required_linear_history is not enabled."
fi

# every required check is present
present="$(jq -r '.required_status_checks.contexts[]?' <<<"$prot" 2>/dev/null || true)"
IFS=',' read -ra want <<<"$REQUIRED_CHECKS"
for c in "${want[@]}"; do
  if ! grep -qxF "$c" <<<"$present"; then
    note "required status check missing: ${c}"
  fi
done

if (( fail )); then
  echo "Branch protection on ${REPO}@${BRANCH} has drifted from policy." >&2
  exit 1
fi
echo "\u2713 Branch protection on ${REPO}@${BRANCH} matches policy."
