#!/bin/bash
# Source 2 of 3: Codecov's public compare API, used when the codecov/patch check
# run produced no verdict.
#
# This is Codecov's data but not Codecov's verdict: the API's patch totals cover
# base_commit..head, and base_commit is the newest ancestor Codecov holds a
# report for rather than where the branch was cut, so the range can be wider
# than the pull request. When that is the case the difference is reported rather
# than hidden, because this source only runs when the check run gave nothing and
# the alternative is a purely local number.
#
# Environment:
#   PR_NUMBER, HEAD_SHA, BASE_REF      - provided by the composite action
#   FAIL_UNDER                         - from read-codecov-config.sh
#   GITHUB_REPOSITORY, GITHUB_OUTPUT, GITHUB_STEP_SUMMARY - provided by the runner
#
# Sets resolved=true when Codecov's number was authoritative (pass or fail).
# Exits 0 with resolved=false when it cannot be trusted in time, so the caller
# computes patch coverage locally. A fetch or parse failure never passes.

set -euo pipefail

SOURCE="Codecov compare API"
SOURCE_LABEL="Codecov's compare API (fallback 1 of 2; the codecov/patch check run gave no verdict)"

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

if [ -z "${PR_NUMBER:-}" ]; then
  echo "⚠️ No pull request to compare through; computing patch coverage locally." | tee -a "$GITHUB_STEP_SUMMARY"
  exit 0
fi

for attempt in $(seq 1 "$POLL_ATTEMPTS"); do
  PROCESSED=$(curl -sf --max-time "$CURL_MAX_TIME" "${API}/commits/${HEAD_SHA}" \
    | jq -r --argjson n "$REQUIRED_SESSIONS" \
        'select((.totals.sessions // 0) >= $n) | .totals.coverage // empty' || true)
  if [ -n "$PROCESSED" ]; then
    COMPARE=$(curl -sf --max-time "$CURL_MAX_TIME" "${API}/compare/?pullid=${PR_NUMBER}") || COMPARE=""
    if [ -z "$COMPARE" ]; then
      echo "⚠️ Could not fetch the Codecov comparison; computing patch coverage locally." | tee -a "$GITHUB_STEP_SUMMARY"
      exit 0
    fi
    # totals.patch is also null when the head report has not been incorporated
    # yet, and an error payload has no totals at all. Only a comparison whose
    # head totals are present can be read as "no coverable lines".
    HEAD_INCLUDED=$(printf '%s' "$COMPARE" | jq -r 'if (.totals != null) and (.totals.head != null) then "yes" else "no" end' 2>/dev/null) || HEAD_INCLUDED="no"
    if [ "$HEAD_INCLUDED" != "yes" ]; then
      echo "⚠️ Incomplete Codecov comparison response; computing patch coverage locally." | tee -a "$GITHUB_STEP_SUMMARY"
      exit 0
    fi

    # Report, but do not hide, a comparison range wider than this branch.
    CODECOV_BASE=$(printf '%s' "$COMPARE" | jq -r '.base_commit // empty' 2>/dev/null) || CODECOV_BASE=""
    MERGE_BASE=$(git merge-base "$BASE_REF" "$HEAD_SHA" 2>/dev/null) || MERGE_BASE=""
    if [ -n "$CODECOV_BASE" ] && [ -n "$MERGE_BASE" ] && [ "$CODECOV_BASE" != "$MERGE_BASE" ]; then
      echo "ℹ️ Codecov compared against ${CODECOV_BASE:0:8} while this branch was cut at ${MERGE_BASE:0:8}, so its total covers a wider range of commits." | tee -a "$GITHUB_STEP_SUMMARY"
    fi

    PATCH=$(printf '%s' "$COMPARE" | jq -r '.totals.patch.coverage' 2>/dev/null) || PATCH=""
    PATCH_LINES=$(printf '%s' "$COMPARE" | jq -r '.totals.patch.lines // 0' 2>/dev/null) || PATCH_LINES="0"
    # A diff with no coverable lines has nothing to gate. Codecov signals this
    # either as coverage null or as coverage 0 with zero patch lines.
    if [ "$PATCH" = "null" ] || [ "$PATCH_LINES" = "0" ]; then
      echo "resolved=true" >> "$GITHUB_OUTPUT"
      echo "✅ Patch coverage passed. Source: ${SOURCE_LABEL}. No coverable lines in this diff." | tee -a "$GITHUB_STEP_SUMMARY"
      exit 0
    fi
    if ! printf '%s' "$PATCH" | grep -qE '^[0-9]+(\.[0-9]+)?$'; then
      echo "⚠️ Unexpected Codecov comparison response; computing patch coverage locally." | tee -a "$GITHUB_STEP_SUMMARY"
      exit 0
    fi

    echo "resolved=true" >> "$GITHUB_OUTPUT"
    if awk -v patch="$PATCH" -v threshold="$FAIL_UNDER" 'BEGIN{exit !(patch + 0 >= threshold + 0)}'; then
      echo "✅ Patch coverage passed. Source: ${SOURCE_LABEL}. ${PATCH}% of ${PATCH_LINES} lines (required: ≥ ${FAIL_UNDER}%)" | tee -a "$GITHUB_STEP_SUMMARY"
      exit 0
    fi
    echo "❌ Patch coverage failed. Source: ${SOURCE_LABEL}. ${PATCH}% of ${PATCH_LINES} lines (required: ≥ ${FAIL_UNDER}%)" | tee -a "$GITHUB_STEP_SUMMARY"
    exit 1
  fi
  echo "${SOURCE} has not fully processed the report yet (attempt ${attempt}/${POLL_ATTEMPTS}); retrying in ${POLL_INTERVAL_SECONDS}s..."
  sleep "$POLL_INTERVAL_SECONDS"
done

echo "⚠️ ${SOURCE} did not process the report after ${POLL_ATTEMPTS} attempts; computing patch coverage locally." | tee -a "$GITHUB_STEP_SUMMARY"
