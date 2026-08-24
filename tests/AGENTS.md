# Tests — Agent Guide

Read with the root [AGENTS.md](../AGENTS.md). Two independent suites live here, with different runtimes and different
conventions:

| Directory | What it is |
|---|---|
| `tests/integration/` | A separate Go module (`go.mod` of its own) exercising the backend's HTTP API against a running server, with ~35 domain packages and a shared `testutils/` harness |
| `tests/e2e/` | A pnpm workspace (`@thunderid/e2e`) driving the real Console and Gate in a browser with Playwright |

Backend unit tests are not here; they sit beside the code in `backend/internal/**/*_test.go`.

## Neither Suite Is Fully Covered by the Default Gate

This is the trap worth knowing before you change anything:

- `make test_e2e` is **not** part of `make pr_checks`. Console or Gate UI changes need it run explicitly.
- Root `pnpm lint` and `pnpm test` are filtered with `--filter=!./tests/**`, so they skip this whole directory. Run lint
  and type-check from inside `tests/e2e`.
- `make test_integration` **is** in `pr_checks`. Filter it while iterating: `RUN="TestName"` or `PACKAGE="pkg/path"`.

## Guides

| Trigger | Read |
|---|---|
| Anything under `tests/integration/` | [.agent/guides/go-testing.md](../.agent/guides/go-testing.md) |
| Anything under `tests/e2e/` | [.agent/guides/e2e-page-objects.md](../.agent/guides/e2e-page-objects.md) |

Before your first edit, read every guide whose trigger matches a file you are about to change, and state in one line
which ones you loaded. If none match, say so.

A flaky test is treated as a critical defect here, not a style issue. Both guides carry the specific patterns that cause
non-determinism in this codebase.
