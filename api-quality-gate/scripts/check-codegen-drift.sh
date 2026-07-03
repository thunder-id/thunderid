#!/usr/bin/env bash
# check-codegen-drift.sh
#
# Deletes the "spec says X, handler does Y" failure class structurally: the Go
# server interfaces/models are generated FROM the linted OpenAPI spec, so if the
# committed generated code differs from a fresh generation, the build fails.
#
# Assumes a `make generate` target that runs oapi-codegen (or your generator).
# Requires the spec to already be lint-clean (run that job first).
set -euo pipefail

echo "Regenerating server code from OpenAPI spec..."
make generate

if ! git diff --quiet; then
  echo "\u2717 Generated code is out of sync with the OpenAPI spec." >&2
  echo "Run 'make generate' and commit the result. Diff:" >&2
  git --no-pager diff --stat >&2
  exit 1
fi
echo "\u2713 Generated code matches the spec."
