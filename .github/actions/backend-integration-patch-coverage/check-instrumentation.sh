#!/bin/bash
# Fails when a changed backend production Go file that can carry coverage is
# absent from every integration profile.
#
# Why this exists: diff-cover only reports on files it finds in a coverage
# report. A changed file missing from all reports is silently dropped from both
# the numerator and the denominator, so an unlinked or uninstrumented package
# would raise the percentage instead of failing the check.
#
# Distinguishing "absent because uninstrumented" from "absent because there is
# nothing to instrument" does not need AST analysis here. Go records coverage
# blocks only inside function bodies, so a file declaring no functions cannot
# appear in a profile no matter what. backend/ is gofmt-enforced by
# golangci-lint, so top-level declarations start at column zero and `^func `
# identifies them reliably. The one generated production file in scope,
# backend/internal/system/i18n/core/defaults.go, is exactly this case: 1300+
# lines of map literal and no functions.
#
# A second form of expected absence is a file outside the binary the integration
# suite exercises. Those tests drive the server, and Go only instruments packages
# linked into it, so such a file emits no records however it is written. That is
# a property of the import graph rather than a coverage gap, and it is listed in
# NOT_IN_SERVER_BINARY below with the reason. Entries are exact paths, not
# prefixes, so exempting a package root never quietly exempts its subpackages.
#
# Arguments:
#   $1 - file listing changed backend production Go paths, one per line
#   $2 - file listing every source path present in the integration profiles
#
# Exits non-zero and names the offending files when the check fails.

set -euo pipefail

CHANGED_LIST="${1:?changed-file list is required}"
INSTRUMENTED_LIST="${2:?instrumented-file list is required}"

# Files the server binary never links, so the integration suite cannot reach them.

NOT_IN_SERVER_BINARY=(
  "backend/pkg/thunderidengine/engine.go"
)

missing=()
exempt_unlinked=()

while IFS= read -r file; do
  [ -n "$file" ] || continue

  # A file with no function declarations has no coverable statements, so its
  # absence from the profiles is expected rather than a measurement gap.
  if ! grep -qE '^func ' "$file" 2>/dev/null; then
    continue
  fi

  # Absent because the server binary does not link it. Reported below rather than
  # skipped silently, so an exemption that outlives its reason stays visible.
  if printf '%s\n' "${NOT_IN_SERVER_BINARY[@]}" | grep -qxF "$file"; then
    exempt_unlinked+=("$file")
    continue
  fi

  if ! grep -qxF "$file" "$INSTRUMENTED_LIST"; then
    missing+=("$file")
  fi
done < "$CHANGED_LIST"

if [ "${#exempt_unlinked[@]}" -gt 0 ]; then
  {
    echo "ℹ️ Exempt from the instrumentation check — not linked into the server binary:"
    for file in "${exempt_unlinked[@]}"; do
      echo "- \`${file}\`"
    done
    echo
  } | tee -a "${GITHUB_STEP_SUMMARY:-/dev/null}"
fi

if [ "${#missing[@]}" -gt 0 ]; then
  {
    echo "❌ Backend integration patch coverage failed — uninstrumented or unlinked changed file."
    echo
    echo "These changed backend production Go files declare functions but appear in none of the"
    echo "integration coverage profiles, so their lines would be silently excluded from the"
    echo "percentage instead of counted as uncovered:"
    echo
    for file in "${missing[@]}"; do
      echo "- \`${file}\`"
    done
    echo
    echo "Either the package is not linked into the server binary, or it is excluded from"
    echo "coverage instrumentation (see backend/.excludecoverage)."
  } | tee -a "${GITHUB_STEP_SUMMARY:-/dev/null}"
  exit 1
fi
