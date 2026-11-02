#!/bin/bash
# Policy tests for the backend integration patch-coverage check.
#
# Each case builds a throwaway git repository with known Go files, a known
# integration coverage profile, and a known diff, so the expected verdict is
# arithmetic rather than a guess. The point is to assert policy outcomes — N/A,
# pass, fail, uninstrumented — not merely that the scripts execute.
#
# Requires diff-cover on PATH or in DIFF_COVER_BIN. CI installs the pinned
# version; locally, point DIFF_COVER_BIN at a virtualenv binary.
#
# Usage: DIFF_COVER_BIN=/path/to/diff-cover ./run-tests.sh

set -uo pipefail

ACTION_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPUTE="${ACTION_DIR}/compute-integration-patch-coverage.sh"
GATE_AWK="${ACTION_DIR}/../patch-coverage-gate/go-cover-to-lcov.awk"

PASSED=0
FAILED=0

if [ -z "${DIFF_COVER_BIN:-}" ] && ! command -v diff-cover >/dev/null 2>&1; then
  echo "❌ diff-cover not found. Set DIFF_COVER_BIN or install diff_cover."
  exit 1
fi
export DIFF_COVER_BIN="${DIFF_COVER_BIN:-diff-cover}"

# ---------------------------------------------------------------------------
# Fixture helpers
# ---------------------------------------------------------------------------

# new_repo <dir> — a git repo with one covered backend file committed on main.
new_repo() {
  local dir="$1"
  mkdir -p "$dir"
  cd "$dir" || exit 1
  git init -q -b main .
  git config user.email test@example.com
  git config user.name Test
  git config commit.gpgsign false

  mkdir -p backend/internal/demo
  cat > backend/internal/demo/service.go <<'GO'
package demo

func Existing() int {
	return 1
}
GO
  git add -A
  git commit -q -m base
}

# add_lines <file> <count> — appends a function whose body has <count> lines.
add_func() {
  local file="$1" name="$2" lines="$3"
  {
    echo ""
    echo "func ${name}() int {"
    echo -e "\tx := 0"
    for ((i = 1; i < lines; i++)); do
      echo -e "\tx++"
    done
    echo -e "\treturn x"
    echo "}"
  } >> "$file"
}

# profile <database> <lines...> — writes a Go cover profile marking the given
# "start end hits" triples for backend/internal/demo/service.go.
profile() {
  local database="$1"; shift
  local dir="coverage-artifacts/integration-coverage-${database}"
  mkdir -p "$dir"
  local out="${dir}/coverage_integration.out"
  echo "mode: set" > "$out"
  while [ "$#" -gt 0 ]; do
    local start="$1" end="$2" hits="$3"; shift 3
    echo "github.com/thunder-id/thunderid/internal/demo/service.go:${start}.1,${end}.2 1 ${hits}" >> "$out"
  done
}

# write_config <min_changed_lines> <default> [tags-yaml-body] [base-packages-yaml-body]
# Cases that do not call this get the default written by run_case.
write_config() {
  local min="$1" default="$2" tags="${3:-}" base="${4:-}"
  mkdir -p .github
  {
    echo "min_changed_lines: ${min}"
    echo "default: ${default}"
    if [ -n "$base" ]; then
      echo "packages:"
      printf '%s\n' "$base"
    else
      echo "packages: {}"
    fi
    if [ -n "$tags" ]; then
      echo "tags:"
      printf '%s\n' "$tags"
    else
      echo "tags: {}"
    fi
  } > .github/backend-coverage-thresholds.yml
}

run_case() {
  local name="$1" expected_status="$2" expected_text="$3" labels="${4:-}"
  local out status

  # min_changed_lines is 0 here on purpose. These fixtures change a handful of
  # lines, so any floor above zero would make every package exempt and the whole
  # suite would pass without measuring anything.
  [ -f .github/backend-coverage-thresholds.yml ] || write_config 0 70

  out=$(BASE_REF=main THRESHOLD_CONFIG=.github/backend-coverage-thresholds.yml \
    PR_LABELS="$labels" GITHUB_STEP_SUMMARY=/dev/null \
    bash "$COMPUTE" 2>&1)
  status=$?

  local ok=1
  [ "$status" -eq "$expected_status" ] || ok=0
  if [ -n "$expected_text" ] && ! grep -qF "$expected_text" <<<"$out"; then
    ok=0
  fi

  if [ "$ok" -eq 1 ]; then
    echo "  ✅ $name"
    PASSED=$((PASSED + 1))
  else
    echo "  ❌ $name (exit=$status expected=$expected_status)"
    echo "$out" | sed 's/^/       /' | head -20
    FAILED=$((FAILED + 1))
  fi
}

# No EXIT trap and no subshells around the cases: an EXIT trap fires on every
# subshell exit, which would delete the fixtures after the first case, and
# counters incremented inside a subshell never reach this shell.
TMPROOT=$(mktemp -d)

echo "Backend integration patch coverage — policy tests"

# ---------------------------------------------------------------------------
# 1. No backend production Go in the diff → N/A success
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/na"
  git checkout -q -b feature
  mkdir -p tests/integration/foo
  echo "package foo" > tests/integration/foo/foo_test.go
  git add -A && git commit -q -m "tests only"
  profile sqlite 3 5 1
  run_case "test-only diff reports N/A and passes" 0 "N/A — no backend production Go changes"

# ---------------------------------------------------------------------------
# 2. Fully covered backend change → pass
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/pass"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 6
  git add -A && git commit -q -m "add covered function"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 1
  run_case "covered backend change passes" 0 "**Passed**"

# ---------------------------------------------------------------------------
# 3. Uncovered backend change → fail
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/fail"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 12
  git add -A && git commit -q -m "add uncovered function"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  run_case "uncovered backend change fails" 1 "**Failed**"

# ---------------------------------------------------------------------------
# 4. Union across databases: covered in one run only → still covered
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/union"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 6
  git add -A && git commit -q -m "add function covered only on postgres"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  profile postgres 6 "$total" 1
  run_case "coverage from a single database counts (union)" 0 "**Passed**"

# ---------------------------------------------------------------------------
# 5. Unit coverage must not lift the result
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/unit"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 12
  git add -A && git commit -q -m "add function covered only by unit tests"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  # A unit profile that fully covers the same lines, sitting in the same tree.
  mkdir -p coverage-artifacts/unit-coverage
  echo "mode: set" > coverage-artifacts/unit-coverage/coverage_unit.out
  echo "github.com/thunder-id/thunderid/internal/demo/service.go:6.1,${total}.2 1 1" \
    >> coverage-artifacts/unit-coverage/coverage_unit.out
  run_case "unit coverage cannot lift the integration result" 1 "**Failed**"

# ---------------------------------------------------------------------------
# 6. Changed file with functions but absent from every profile → fail loudly
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/uninstrumented"
  git checkout -q -b feature
  mkdir -p backend/internal/orphan
  cat > backend/internal/orphan/orphan.go <<'GO'
package orphan

func Unlinked() int {
	return 42
}
GO
  git add -A && git commit -q -m "add unlinked package"
  profile sqlite 3 5 1
  run_case "uninstrumented changed file fails instead of being dropped" 1 "uninstrumented or unlinked changed file"

# ---------------------------------------------------------------------------
# 7. Declaration-only changed file absent from profiles → not a failure
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/declonly"
  git checkout -q -b feature
  mkdir -p backend/internal/generated
  {
    echo "package generated"
    echo ""
    echo "var messages = map[string]string{"
    echo '	"a": "b",'
    echo "}"
  } > backend/internal/generated/defaults.go
  git add -A && git commit -q -m "add declaration-only file"
  profile sqlite 3 5 1
  run_case "declaration-only file is not reported as uninstrumented" 0 ""

# ---------------------------------------------------------------------------
# 8. Backend change with no coverage artifacts at all → fail
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/noartifacts"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 6
  git add -A && git commit -q -m "add function, no artifacts produced"
  run_case "missing coverage artifacts fail rather than pass" 1 "did not reach this job"

# ---------------------------------------------------------------------------
# 9. An artifact that arrived empty or unparseable must say so, not die silently
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/emptyprofile"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 6
  git add -A && git commit -q -m "add function, profile is garbage"
  mkdir -p coverage-artifacts/integration-coverage-sqlite
  printf 'mode: set\nnot a cover profile line\n' \
    > coverage-artifacts/integration-coverage-sqlite/coverage_integration.out
  run_case "unparseable profile reports why instead of exiting silently" 1 "no usable source records"

# ---------------------------------------------------------------------------
# 10. No labels: every package falls back to the global default
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/thr-default"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 12
  git add -A && git commit -q -m "uncovered change, no labels"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  write_config 0 70
  run_case "no labels uses the global default" 1 "global default"

# ---------------------------------------------------------------------------
# 11. A tag may loosen below the global default
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/thr-loosen"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 10
  git add -A && git commit -q -m "half covered"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  # Cover the first half of the added body, leave the rest uncovered.
  half=$(( (6 + total) / 2 ))
  profile sqlite 6 "$half" 1 $((half + 1)) "$total" 0
  write_config 0 70 "  experimental:
    packages:
      backend/internal/demo: 10"
  run_case "a tag can lower the bar below the default" 0 "tag \`experimental\`" "[\"experimental\"]"

# ---------------------------------------------------------------------------
# 12. Threshold is inherited from a parent package path
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/thr-inherit"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 12
  git add -A && git commit -q -m "uncovered change"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  write_config 0 70 "  strict:
    packages:
      backend/internal: 95"
  run_case "threshold is inherited from the parent path" 1 "inherited from" "[\"strict\"]"

# ---------------------------------------------------------------------------
# 13. Multiple applicable tags: the highest threshold applies
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/thr-max"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 10
  git add -A && git commit -q -m "half covered"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  half=$(( (6 + total) / 2 ))
  profile sqlite 6 "$half" 1 $((half + 1)) "$total" 0
  # Roughly half covered. The lower tag would pass, the higher one fails, so the
  # verdict shows which one was chosen without needing an impossible threshold.
  write_config 0 70 "  lower:
    packages:
      backend/internal/demo: 40
  higher:
    packages:
      backend/internal/demo: 60"
  run_case "highest threshold wins across applicable tags" 1 "60%" "[\"lower\",\"higher\"]"

# ---------------------------------------------------------------------------
# 14. A label with no matching tag group changes nothing
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/thr-unknown-label"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 12
  git add -A && git commit -q -m "uncovered change"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  write_config 0 70 "  strict:
    packages:
      backend/internal/demo: 95"
  run_case "an unrelated label leaves the default in place" 1 "global default" "[\"unrelated\"]"

# ---------------------------------------------------------------------------
# 15. min_changed_lines exempts a small package
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/thr-minlines"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 6
  git add -A && git commit -q -m "small uncovered change"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  write_config 500 70
  run_case "a package under min_changed_lines is exempt" 0 "exempt"

# ---------------------------------------------------------------------------
# 16. A config path that does not exist fails the check
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/thr-stale"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 12
  git add -A && git commit -q -m "uncovered change"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  write_config 0 70 "  strict:
    packages:
      backend/internal/gone: 95"
  run_case "a stale config path fails rather than rotting" 1 "do not exist" "[\"strict\"]"

# ---------------------------------------------------------------------------
# 17. Base packages apply when no label is present
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/base-pkg"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 12
  git add -A && git commit -q -m "uncovered change"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  write_config 0 70 "" "  backend/internal/demo: 0"
  run_case "a base package entry applies with no labels" 0 "base entry"

# ---------------------------------------------------------------------------
# 18. A tag default overrides a base package entry
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/tagdefault-beats-base"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 12
  git add -A && git commit -q -m "uncovered change"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  # Base exempts this package at 0, but the tag sets a default and names nothing,
  # so the tag default wins and the package fails.
  write_config 0 70 "  strict:
    default: 85" "  backend/internal/demo: 0"
  run_case "a tag default overrides a base package entry" 1 "tag \`strict\` default" "[\"strict\"]"

# ---------------------------------------------------------------------------
# 19. A tag package entry overrides a base package entry, and may loosen
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/tagpkg-beats-base"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 12
  git add -A && git commit -q -m "uncovered change"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  write_config 0 70 "  experimental:
    packages:
      backend/internal/demo: 0" "  backend/internal/demo: 95"
  run_case "a tag package entry overrides base and may loosen" 0 "tag \`experimental\`" "[\"experimental\"]"

# ---------------------------------------------------------------------------
# 20. A named package beats another applicable tag's default
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/named-beats-default"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 10
  git add -A && git commit -q -m "half covered"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  half=$(( (6 + total) / 2 ))
  profile sqlite 6 "$half" 1 $((half + 1)) "$total" 0
  # Roughly half covered. The naming tag's 40 passes, the other tag's default of 60
  # would fail, so passing proves the named entry won over the default.
  write_config 0 70 "  naming:
    packages:
      backend/internal/demo: 40
  defaulting:
    default: 60"
  run_case "a named package beats another tag's default" 0 "tag \`naming\`" "[\"naming\",\"defaulting\"]"

# ---------------------------------------------------------------------------
# 21. An applicable tag that says nothing falls through to the base layer
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/tag-silent"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 12
  git add -A && git commit -q -m "uncovered change"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  # The referenced package must exist, or the stale-path guard fires first. An
  # untracked empty directory is enough and stays out of the diff.
  mkdir -p backend/internal/other
  # The tag applies but names another package and sets no default, so the base
  # entry is still what decides.
  write_config 0 70 "  narrow:
    packages:
      backend/internal/other: 95" "  backend/internal/demo: 0"
  run_case "a silent tag falls through to the base layer" 0 "base entry" "[\"narrow\"]"

# ---------------------------------------------------------------------------
# 22. Frontend-only diff is out of scope
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/frontend-only"
  git checkout -q -b feature
  mkdir -p frontend/apps/console/src
  echo "export const x = 1;" > frontend/apps/console/src/app.ts
  git add -A && git commit -q -m "frontend only"
  profile sqlite 3 5 1
  run_case "frontend-only diff reports N/A" 0 "N/A — no backend production Go changes"

# ---------------------------------------------------------------------------
# 23. Docs-only diff is out of scope
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/docs-only"
  git checkout -q -b feature
  mkdir -p docs/content
  echo "# Heading" > docs/content/page.md
  git add -A && git commit -q -m "docs only"
  profile sqlite 3 5 1
  run_case "docs-only diff reports N/A" 0 "N/A — no backend production Go changes"

# ---------------------------------------------------------------------------
# 24. Comments-only change to an instrumented backend file
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/comments-only"
  git checkout -q -b feature
  # The file is in the profile and declares functions, so it is in scope. Only a
  # comment changes, so it contributes no coverable lines.
  echo "// A trailing comment, no executable statement." >> backend/internal/demo/service.go
  git add -A && git commit -q -m "comment only"
  profile sqlite 3 5 1
  run_case "comments-only backend change is not a coverage failure" 0 ""

# ---------------------------------------------------------------------------
# 25. A package with nothing coverable is exempt, not a division by zero
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/zero-coverable"
  git checkout -q -b feature
  echo "// Only a comment." >> backend/internal/demo/service.go
  git add -A && git commit -q -m "comment only"
  # The file is in the profile, so it can appear in the report with no coverable
  # changed lines. min_changed_lines is 0 here, so only an explicit guard saves it.
  profile sqlite 3 5 1
  write_config 0 70
  run_case "a package with nothing coverable does not divide by zero" 0 ""

# ---------------------------------------------------------------------------
# 26. A negative threshold is rejected, not silently satisfied by anything
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/cfg-negative"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 12
  git add -A && git commit -q -m "uncovered change"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  write_config 0 -1
  run_case "a negative threshold is rejected" 1 "between 0 and 100"

# ---------------------------------------------------------------------------
# 27. A non-numeric threshold reports why, instead of a bare traceback
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/cfg-string"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 12
  git add -A && git commit -q -m "uncovered change"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  write_config 0 '"seventy"'
  run_case "a non-numeric threshold reports why" 1 "must be a number"

# ---------------------------------------------------------------------------
# 28. A negative min_changed_lines is rejected
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/cfg-minlines"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 12
  git add -A && git commit -q -m "uncovered change"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  write_config -5 70
  run_case "a negative min_changed_lines is rejected" 1 "non-negative integer"

# ---------------------------------------------------------------------------
# 29. The pre-nesting flat tag schema is rejected with a pointer to the fix
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/cfg-flat"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 12
  git add -A && git commit -q -m "uncovered change"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 0
  # Package paths directly under the tag, without the `packages` key. This parses
  # cleanly and would otherwise resolve to the global default while looking right.
  write_config 0 70 "  strict:
    backend/internal/demo: 95"
  run_case "the flat tag schema is rejected, not silently ignored" 1 "belong under"

# ---------------------------------------------------------------------------
# 30. An exempt package reports its counts and where its threshold came from
# ---------------------------------------------------------------------------
  new_repo "$TMPROOT/exempt-report"
  git checkout -q -b feature
  add_func backend/internal/demo/service.go Added 6
  git add -A && git commit -q -m "small change"
  total=$(wc -l < backend/internal/demo/service.go | tr -d " ")
  profile sqlite 6 "$total" 1
  write_config 500 70
  run_case "an exempt package reports counts and provenance" 0 "changed coverable lines covered, would need 70% (global default)"

echo
rm -rf "$TMPROOT"
echo "passed: $PASSED  failed: $FAILED"
[ "$FAILED" -eq 0 ]
