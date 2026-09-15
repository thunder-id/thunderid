#!/bin/bash
# Computes backend patch coverage from the integration test runs alone and
# enforces the threshold.
#
# This is deliberately not the combined 🛡️ Patch Coverage Gate. That gate feeds
# backend unit, backend integration, and both frontend reports into one
# diff-cover invocation, so a changed backend line covered only by a unit test
# satisfies it. This check loads the three integration profiles and nothing else,
# so the number answers one question: did the integration suite actually execute
# the backend code this PR changed?
#
# Coverage from the SQLite, PostgreSQL, and Redis runs is a union. diff-cover
# takes all three reports in one invocation, and a line counts as covered when
# any report records a hit for it, which is the intended semantics: the databases
# exercise different branches and no single run is expected to cover everything.
#
# The metric is changed coverable *lines*, not statements. Go's profile carries a
# statement count per block, but the LCOV format and diff-cover are line-based,
# and uncovered line numbers are what a developer can act on. Weighting by
# statements would change the denominator without making the result more
# actionable.
#
# Thresholds are per package rather than global, resolved from THRESHOLD_CONFIG by
# evaluate-thresholds.py. Any non-exempt package below its own threshold fails the
# check.
#
# Environment:
#   BASE_REF            - diff base, e.g. origin/main or a merge-group base sha
#   THRESHOLD_CONFIG    - path to the per-package threshold config
#   PR_LABELS           - JSON array or comma-separated list of the PR's labels
#   GITHUB_STEP_SUMMARY - provided by the runner
#
# Expects the integration coverage artifacts under coverage-artifacts/.

set -euo pipefail

DIFF_COVER_VERSION="10.4.0"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Reused rather than copied: one converter, one set of path-mapping rules.
AWK_CONVERTER="${SCRIPT_DIR}/../patch-coverage-gate/go-cover-to-lcov.awk"

: "${BASE_REF:?BASE_REF is required}"
: "${THRESHOLD_CONFIG:=.github/backend-coverage-thresholds.yml}"
: "${PR_LABELS:=}"
SUMMARY="${GITHUB_STEP_SUMMARY:-/dev/null}"

if [ ! -f "$AWK_CONVERTER" ]; then
  echo "❌ Coverage converter not found at ${AWK_CONVERTER}." | tee -a "$SUMMARY"
  exit 1
fi

if [ ! -f "$THRESHOLD_CONFIG" ]; then
  echo "❌ Threshold config not found at ${THRESHOLD_CONFIG}." | tee -a "$SUMMARY"
  exit 1
fi

# ---------------------------------------------------------------------------
# 1. Is there anything in scope to measure?
# ---------------------------------------------------------------------------

bash "${SCRIPT_DIR}/list-changed-backend-go.sh" > changed-backend-go.txt

if [ ! -s changed-backend-go.txt ]; then
  {
    echo "### 🛡️ Backend Integration Patch Coverage"
    echo
    echo "**N/A — no backend production Go changes in this diff.**"
    echo
    echo "Nothing outside \`backend/\` production Go affects this check, so there is nothing to measure."
  } | tee -a "$SUMMARY"
  exit 0
fi

CHANGED_COUNT=$(wc -l < changed-backend-go.txt | tr -d ' ')

# ---------------------------------------------------------------------------
# 2. Collect the integration profiles only
# ---------------------------------------------------------------------------

LCOV_FILES=()
PROFILE_LABELS=()

while IFS= read -r profile; do
  # coverage-artifacts/integration-coverage-<database>/coverage_integration.out
  database=$(basename "$(dirname "$profile")" | sed 's/^integration-coverage-//')
  lcov="integration-${database}-lcov.info"
  awk -f "$AWK_CONVERTER" "$profile" > "$lcov"
  LCOV_FILES+=("$lcov")
  PROFILE_LABELS+=("$database")
done < <(find coverage-artifacts -type f -name 'coverage_integration.out' 2>/dev/null | sort)

if [ "${#LCOV_FILES[@]}" -eq 0 ]; then
  # The diff changes backend production Go, so this is missing measurement
  # rather than nothing to measure. Passing here would defeat the check.
  {
    echo "### 🛡️ Backend Integration Patch Coverage"
    echo
    echo "**Failed — the integration coverage artifacts did not reach this job.**"
    echo
    echo "This PR changes ${CHANGED_COUNT} backend production Go file(s), but no"
    echo "\`coverage_integration.out\` was found under \`coverage-artifacts/\`."
  } | tee -a "$SUMMARY"
  exit 1
fi

# ---------------------------------------------------------------------------
# 3. Refuse to silently drop an uninstrumented changed file
# ---------------------------------------------------------------------------

# Guarded: grep exits 1 on no match, which under `set -e` would kill the job with
# no diagnostic. An artifact that arrived empty or unparseable must say so.
grep -h '^SF:' "${LCOV_FILES[@]}" 2>/dev/null | sed 's/^SF://' | sort -u > instrumented-files.txt || true

if [ ! -s instrumented-files.txt ]; then
  {
    echo "### 🛡️ Backend Integration Patch Coverage"
    echo
    echo "**Failed — the integration coverage profiles contained no usable source records.**"
    echo
    echo "Converted ${#LCOV_FILES[@]} profile(s) from: ${PROFILE_LABELS[*]}"
    echo "Every one produced zero \`SF:\` entries, so the profiles are empty or not in Go"
    echo "cover format. Measuring against them would report a misleading percentage."
  } | tee -a "$SUMMARY"
  exit 1
fi

bash "${SCRIPT_DIR}/check-instrumentation.sh" changed-backend-go.txt instrumented-files.txt

# ---------------------------------------------------------------------------
# 4. Measure
# ---------------------------------------------------------------------------

# CI installs the pinned version. DIFF_COVER_BIN lets the test suite point at an
# already-installed binary instead, so the policy logic is verifiable locally
# rather than only observable through trial pushes to Actions.
if [ -z "${DIFF_COVER_BIN:-}" ]; then
  pipx install "diff_cover==${DIFF_COVER_VERSION}" >/dev/null
  DIFF_COVER_BIN="diff-cover"
fi

# Everything outside backend/ production Go is excluded twice over: it is absent
# from the integration profiles, and named here so the intent is explicit in the
# job log rather than an emergent property of the reports.
EXCLUDE_ARGS=(--exclude 'backend/tests/**' --exclude '**/*_test.go' --exclude 'tests/**'
  --exclude 'frontend/**' --exclude 'docs/**' --exclude 'samples/**' --exclude 'api/**')

# No --fail-under: a single global threshold cannot express per-package bars. The
# JSON report carries per-file covered and uncovered line numbers, so the verdict
# is derived per package by evaluate-thresholds.py. The markdown report is still
# produced, for the uncovered-line detail appended to the summary.
"$DIFF_COVER_BIN" "${LCOV_FILES[@]}" \
  --compare-branch "$BASE_REF" \
  "${EXCLUDE_ARGS[@]}" \
  --format "json:integration-patch-coverage.json,markdown:integration-patch-coverage.md" >/dev/null

STATUS=0
python3 "${SCRIPT_DIR}/evaluate-thresholds.py" \
  integration-patch-coverage.json \
  "$THRESHOLD_CONFIG" \
  "$PR_LABELS" \
  package-verdicts.md || STATUS=$?

# ---------------------------------------------------------------------------
# 5. Report
# ---------------------------------------------------------------------------

# The per-package verdict, thresholds and provenance are already formatted by
# evaluate-thresholds.py. Only the measurement provenance is added here.
{
  cat package-verdicts.md
  echo "- Measured from integration runs only: ${PROFILE_LABELS[*]} (union)"
  echo "- Backend unit and frontend coverage are not loaded and cannot change this result."
  echo "- Changed backend production Go files: ${CHANGED_COUNT}"
  echo
  if [ "$STATUS" -ne 0 ]; then
    cat integration-patch-coverage.md 2>/dev/null || true
  fi
} | tee -a "$SUMMARY"

exit "$STATUS"
