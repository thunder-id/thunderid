#!/bin/bash
# Polls the Codecov API for the pull request's patch coverage and enforces it.
#
# Environment:
#   PR_NUMBER, HEAD_SHA, FAIL_UNDER  - provided by the composite action
#   GITHUB_REPOSITORY, GITHUB_OUTPUT, GITHUB_STEP_SUMMARY - provided by the runner
#
# Sets the step output resolved=true when Codecov's number was authoritative
# (pass or fail). Exits 0 with resolved=false when Codecov cannot be trusted
# in time, so the caller falls back to the local computation. A fetch or parse
# failure never passes the gate.

set -euo pipefail

# Number of coverage uploads in a complete report; mirrors notify.after_n_builds
# in codecov.yml. Codecov processes uploads incrementally, and a comparison
# against a partially processed head report yields a bogus patch number
# (e.g. 0% when only backend sessions are in but the diff is frontend-only).
REQUIRED_SESSIONS=6
POLL_ATTEMPTS=10
POLL_INTERVAL_SECONDS=30
CURL_MAX_TIME=15

OWNER="${GITHUB_REPOSITORY%/*}"
REPO="${GITHUB_REPOSITORY#*/}"
API="https://api.codecov.io/api/v2/github/${OWNER}/repos/${REPO}"

echo "resolved=false" >> "$GITHUB_OUTPUT"

for attempt in $(seq 1 "$POLL_ATTEMPTS"); do
  PROCESSED=$(curl -sf --max-time "$CURL_MAX_TIME" "${API}/commits/${HEAD_SHA}" \
    | jq -r --argjson n "$REQUIRED_SESSIONS" \
        'select((.totals.sessions // 0) >= $n) | .totals.coverage // empty' || true)
  if [ -n "$PROCESSED" ]; then
    # The head report is fully processed; read patch coverage from the
    # comparison. A fetch or parse failure here must NOT pass the gate:
    # leave it unresolved so the local computation runs instead.
    COMPARE=$(curl -sf --max-time "$CURL_MAX_TIME" "${API}/compare/?pullid=${PR_NUMBER}") || COMPARE=""
    if [ -z "$COMPARE" ]; then
      echo "⚠️ Could not fetch the Codecov comparison; falling back to local patch coverage." | tee -a "$GITHUB_STEP_SUMMARY"
      exit 0
    fi
    # Validate the comparison is structurally complete before interpreting
    # missing patch data: totals.patch is also null when the head report has
    # not been incorporated yet, and an error payload has no totals at all.
    # Only a comparison whose head totals are present can be trusted to mean
    # "no coverable lines"; anything else falls back to the local computation.
    HEAD_INCLUDED=$(printf '%s' "$COMPARE" | jq -r 'if (.totals != null) and (.totals.head != null) then "yes" else "no" end' 2>/dev/null) || HEAD_INCLUDED="no"
    if [ "$HEAD_INCLUDED" != "yes" ]; then
      echo "⚠️ Incomplete Codecov comparison response; falling back to local patch coverage." | tee -a "$GITHUB_STEP_SUMMARY"
      exit 0
    fi
    PATCH=$(printf '%s' "$COMPARE" | jq -r '.totals.patch.coverage' 2>/dev/null) || PATCH=""
    PATCH_LINES=$(printf '%s' "$COMPARE" | jq -r '.totals.patch.lines // 0' 2>/dev/null) || PATCH_LINES="0"
    # A diff with no coverable lines (comment/config/CI-only changes) has nothing
    # to gate. Codecov signals this either as coverage null or as coverage 0 with
    # zero patch lines.
    if [ "$PATCH" = "null" ] || [ "$PATCH_LINES" = "0" ]; then
      echo "resolved=true" >> "$GITHUB_OUTPUT"
      echo "✅ Codecov reports no coverable lines in this diff." | tee -a "$GITHUB_STEP_SUMMARY"
      exit 0
    fi
    if ! printf '%s' "$PATCH" | grep -qE '^[0-9]+(\.[0-9]+)?$'; then
      echo "⚠️ Unexpected Codecov comparison response; falling back to local patch coverage." | tee -a "$GITHUB_STEP_SUMMARY"
      exit 0
    fi
    echo "resolved=true" >> "$GITHUB_OUTPUT"
    echo "Codecov patch coverage: ${PATCH}% (required: ≥ ${FAIL_UNDER}%)" | tee -a "$GITHUB_STEP_SUMMARY"
    awk -v patch="$PATCH" -v threshold="$FAIL_UNDER" 'BEGIN{exit !(patch + 0 >= threshold + 0)}' || {
      echo "❌ Patch coverage is below the required threshold." | tee -a "$GITHUB_STEP_SUMMARY"
      exit 1
    }
    exit 0
  fi
  echo "Codecov has not fully processed the report yet (attempt ${attempt}/${POLL_ATTEMPTS}); retrying in ${POLL_INTERVAL_SECONDS}s..."
  sleep "$POLL_INTERVAL_SECONDS"
done

echo "⚠️ Codecov did not process the report in time; falling back to local patch coverage." | tee -a "$GITHUB_STEP_SUMMARY"
