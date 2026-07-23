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
#   BASE_REF, FAIL_UNDER  - provided by the composite action
#   GITHUB_STEP_SUMMARY   - provided by the runner
#
# Expects the coverage artifacts to be downloaded under coverage-artifacts/.

set -euo pipefail

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
for app in console gate; do
  src=$(find "coverage-artifacts/${app}-coverage" -type f -name 'lcov.info' 2>/dev/null | head -1)
  [ -n "$src" ] || continue
  dst="${app}-lcov.info"
  sed -E "s|^SF:(.*/)?frontend/apps/${app}/|SF:frontend/apps/${app}/|; t; s|^SF:|SF:frontend/apps/${app}/|" "$src" > "$dst"
  FILES+=("$dst")
done

if [ "${#FILES[@]}" -eq 0 ]; then
  # Fail rather than pass silently when coverable code changed but no
  # coverage data reached the gate; passing would defeat its purpose.
  COVERABLE=$(git diff --name-only "${BASE_REF}...HEAD" \
    | { grep -E '^(backend/.*\.go$|frontend/apps/(console|gate)/src/.*\.(ts|tsx)$)' || true; } \
    | { grep -vE '(^backend/tests/|_test\.go$|\.test\.(ts|tsx)$|__tests__/|__mocks__/)' || true; })
  if [ -n "$COVERABLE" ]; then
    echo "❌ Coverable files changed but no coverage artifacts were found." | tee -a "$GITHUB_STEP_SUMMARY"
    exit 1
  fi
  echo "✅ No coverage artifacts and no coverable changes; nothing to gate." | tee -a "$GITHUB_STEP_SUMMARY"
  exit 0
fi

STATUS=0
diff-cover "${FILES[@]}" \
  --compare-branch "$BASE_REF" \
  --fail-under "$FAIL_UNDER" \
  --exclude 'backend/tests/**' '**/*_test.go' 'tests/**' 'samples/**' 'docs/**' 'api/**' \
            '**/*.test.ts' '**/*.test.tsx' '**/__tests__/**' '**/__mocks__/**' \
  --format markdown:patch-coverage.md || STATUS=$?
cat patch-coverage.md >> "$GITHUB_STEP_SUMMARY" 2>/dev/null || true
exit "$STATUS"
