#!/usr/bin/env bash
# selftest.sh — proves the ruleset behaves, so you don't have to trust it blind.
# 1. The bad sample MUST trip the custom rules.
# 2. The good sample MUST pass clean.
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

EXPECT=(
  collection-get-has-pagination
  collection-get-has-filter
  operation-has-standard-errors
  errors-use-problem-json
  write-has-idempotency-key
)

echo "== Linting the intentionally-bad sample =="
bad_out="$(npx spectral lint examples/openapi.bad.yaml -r .spectral.yaml --format text 2>&1)"
echo "$bad_out"

missing=0
for rule in "${EXPECT[@]}"; do
  if grep -q "$rule" <<<"$bad_out"; then
    echo "  ok: $rule fired"
  else
    echo "  FAIL: $rule did NOT fire"; missing=1
  fi
done

echo
echo "== Linting the compliant sample (must be clean) =="
if npx spectral lint examples/openapi.good.yaml -r .spectral.yaml --fail-severity=warn; then
  echo "  ok: compliant sample passed"
else
  echo "  FAIL: compliant sample produced findings"; missing=1
fi

if (( missing )); then echo; echo "SELFTEST FAILED"; exit 1; fi
echo; echo "SELFTEST PASSED"
