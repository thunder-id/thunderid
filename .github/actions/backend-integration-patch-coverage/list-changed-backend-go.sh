#!/bin/bash
# Lists the changed backend production Go files that can carry integration
# coverage, so the gate can tell "nothing to measure" from "something to measure
# but no data arrived".
#
# Scope is stated here rather than derived from codecov.yml, because codecov's
# flag paths answer a different question: which trees upload coverage at all.
# This check measures one specific thing, backend production Go executed by the
# integration suite, so its scope is written out explicitly and reviewably.
#
# Excluded, matching the policy:
#   backend/**/*_test.go   unit tests are not the thing being measured
#   backend/tests/**       mocks and helpers, also absent from every profile
#   everything outside backend/
#   deleted files          deleted code cannot be executed in the PR build
#
# Environment:
#   BASE_REF - diff base, e.g. origin/main or a merge-group base sha
#
# Prints one repo-relative path per line; prints nothing when the diff cannot
# affect backend integration coverage.

set -euo pipefail

if [ -z "${BASE_REF:-}" ]; then
  echo "❌ BASE_REF is not set." >&2
  exit 1
fi

# --diff-filter=d drops deletions: a deleted line cannot be executed, so it must
# not land in the denominator.
git diff --name-only --diff-filter=d "${BASE_REF}...HEAD" \
  | grep -E '^backend/.*\.go$' \
  | grep -v -E '_test\.go$' \
  | grep -v -E '^backend/tests/' \
  || true
