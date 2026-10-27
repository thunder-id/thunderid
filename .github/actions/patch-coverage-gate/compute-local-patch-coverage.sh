#!/bin/bash
# Computes patch coverage locally from the archived coverage artifacts with
# diff-cover and enforces the threshold. Mirrors Codecov's combined patch
# semantics: all backend and frontend reports feed a single diff-cover
# invocation, so one pooled percentage covers the whole diff.
#
# Go cover profiles are converted to LCOV in-house (go-cover-to-lcov.awk) and
# frontend LCOV files are used as-is, so diff-cover receives a single format
# and no third-party converters are needed.
#
# Environment:
#   BASE_REF                                - provided by the composite action
#   FAIL_UNDER, CODECOV_IGNORE_FILE         - from read-codecov-config.sh
#   GITHUB_STEP_SUMMARY                     - provided by the runner
#
# Expects the coverage artifacts to be downloaded under coverage-artifacts/.

set -euo pipefail

SOURCE_LABEL="computed here by diff-cover from the coverage artifacts (fallback 2 of 2; Codecov reported nothing)"

DIFF_COVER_VERSION="10.4.0"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

pipx install "diff_cover==${DIFF_COVER_VERSION}"

FILES=()

# Convert Go cover profiles to LCOV with repo-relative paths. Locate profiles
# with find so the gate does not depend on the artifacts' internal layout.
GO_LCOV_INDEX=0
while IFS= read -r profile; do
  GO_LCOV_INDEX=$((GO_LCOV_INDEX + 1))
  lcov="go-coverage-${GO_LCOV_INDEX}-lcov.info"
  awk -f "${SCRIPT_DIR}/go-cover-to-lcov.awk" "$profile" > "$lcov"
  FILES+=("$lcov")
done < <(find coverage-artifacts -type f \( -name 'coverage_unit.out' -o -name 'coverage_integration.out' \) 2>/dev/null | sort)

# Rewrite LCOV SF paths (absolute or app-relative) to repo-relative paths.
for app in gate; do
  src=$(find "coverage-artifacts/${app}-coverage" -type f -name 'lcov.info' 2>/dev/null | head -1)
  [ -n "$src" ] || continue
  dst="${app}-lcov.info"
  sed -E "s|^SF:(.*/)?frontend/apps/${app}/|SF:frontend/apps/${app}/|; t; s|^SF:|SF:frontend/apps/${app}/|" "$src" > "$dst"
  FILES+=("$dst")
done

if [ "${#FILES[@]}" -eq 0 ]; then
  # This source only runs when the diff was found to contain files that report
  # coverage, so missing artifacts mean the measurement went missing rather than
  # that there was nothing to measure. Passing here would defeat the gate.
  echo "❌ Patch coverage failed. Source: ${SOURCE_LABEL}. Files that report coverage changed, but no coverage artifacts reached the gate." | tee -a "$GITHUB_STEP_SUMMARY"
  exit 1
fi

# The excluded paths come from codecov.yml's ignore list, so a line that Codecov
# would count is never dropped here.
EXCLUDES=()
while IFS= read -r glob; do
  [ -n "$glob" ] || continue
  EXCLUDES+=("$glob")
done < "$CODECOV_IGNORE_FILE"
# --exclude takes all of its patterns as one flag, so build the flag only when
# there is at least one; a bare --exclude would be a usage error.
EXCLUDE_ARGS=()
if [ "${#EXCLUDES[@]}" -gt 0 ]; then
  EXCLUDE_ARGS=(--exclude "${EXCLUDES[@]}")
fi

STATUS=0
diff-cover "${FILES[@]}" \
  --compare-branch "$BASE_REF" \
  --fail-under "$FAIL_UNDER" \
  "${EXCLUDE_ARGS[@]}" \
  --format markdown:patch-coverage.md || STATUS=$?

# Named explicitly because this number is computed here rather than reported by
# Codecov, and the two are close but not identical.
if [ "$STATUS" -eq 0 ]; then
  echo "✅ Patch coverage passed. Source: ${SOURCE_LABEL} (required: ≥ ${FAIL_UNDER}%)" | tee -a "$GITHUB_STEP_SUMMARY"
else
  echo "❌ Patch coverage failed. Source: ${SOURCE_LABEL} (required: ≥ ${FAIL_UNDER}%)" | tee -a "$GITHUB_STEP_SUMMARY"
fi
cat patch-coverage.md >> "$GITHUB_STEP_SUMMARY" 2>/dev/null || true
exit "$STATUS"
