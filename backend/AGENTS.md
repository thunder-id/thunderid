# Backend — Agent Guide

Read with the root [AGENTS.md](../AGENTS.md). The backend is a Go application under `backend/` (module `github.com/thunder-id/thunderid`): domain packages under `backend/internal/`, entry points under `backend/cmd/`, public packages under `backend/pkg/`.

For the canonical, deeper reference (flat package/file layout, export rules, logging), see the [Backend Development overview](../docs/content/community/contributing/contributing-code/backend-development/overview.mdx).

## When to Load Other Guides

- Touching `store.go`, schema, or queries → read [.agent/skills/db/SKILL.md](../.agent/skills/db/SKILL.md).
- Changing any interface → regenerate mocks with `make mockery`. CI's `verify_mocks` job fails if mocks are out of sync. Never hand-edit mock files.

## Logging

Use the `log` package from `internal/system` and pass the request `context.Context` as the first argument so entries carry the correlation (trace) ID. Avoid PII; mask sensitive values. Details in the overview doc.

## Test Selection

- **Inner loop**:
  - service / store / API handler change → `make test_unit` first.
  - DB or API-contract change → also add `make test_integration` (filter with `RUN="TestName"` or `PACKAGE="pkg/path"`).
- **Final gate**: `make test_unit` + `make test_integration`, or `make pr_checks` before opening a PR.
