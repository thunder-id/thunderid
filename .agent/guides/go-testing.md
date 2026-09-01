---
paths:
  - "backend/**/*_test.go"
  - "tests/integration/**/*.go"
---

# Go Testing

Two suites, different purposes. Unit tests live beside the code in `backend/internal/**/*_test.go`. The integration
suite is a **separate Go module** at `tests/integration/` with its own `go.mod`, roughly 35 domain packages, and a
shared `testutils/` harness.

## Commands

- `make test_unit` — backend unit tests.
- `make test_integration` — the integration module. Filter with `RUN="TestName"` or `PACKAGE="pkg/path"`.
- `make test` is these two and nothing else, so it is not full-repo validation.
- After changing any interface, `make mockery`. The `verify-mocks` CI job fails the build on drift and mock files are
  never hand-edited.

## The `testutils` harness

Before writing a fake, check `tests/integration/testutils/` for one that exists: mock OAuth, OIDC, Google OIDC, and
GitHub OAuth servers, a mock SMTP server, a mock notification server, a generic mock HTTP server, JWT and OAuth2
helpers, passkey and WebAuthn authenticator support, and shared models. Reimplementing one of these locally is the most
common avoidable duplication in this suite.

## Flaky tests are treated as critical, not cosmetic

A test that passes most of the time and fails unpredictably in CI wastes maintainer time and erodes trust in the whole
suite, so these patterns are defects rather than style points:

- **Encoding equivalence.** Mutating a base64, hex, or URL-encoded string in a position where the change does not alter
  the decoded bytes. Flipping the last character of a `base64.RawURLEncoding` string may decode identically depending on
  input length and padding alignment.
- **Map iteration order.** Go randomizes `range` order over a map per iteration. Never assert on it.
- **Time-dependent assertions.** Comparing against `time.Now()` or a fixed duration without tolerance, sleeping a fixed
  time and asserting state, or assuming wall-clock ordering across goroutines.
- **Port and resource conflicts.** Hardcoded ports, fixed temp paths, or shared resources that collide under parallel
  execution.
- **Goroutine races.** Shared mutable state touched from test goroutines without synchronization, or `t.Parallel()` tests
  closing over a loop variable.
- **Non-deterministic input.** `rand` without a fixed seed, or generated values that can collide (UUID-based names later
  asserted by position).
- **External service timing.** Assertions that depend on a server, database, or network call finishing within an
  implicit timeout.
- **Flaky cleanup.** `defer` cleanup racing an asynchronous operation, or teardown that assumes creation completed.

When you find one, describe the concrete failure scenario (the inputs or state that trigger it) and make it
deterministic rather than adding a retry or a longer sleep.

## Coverage

Per-package thresholds live in `.github/backend-coverage-thresholds.yml` and `codecov.yml`, and integration patch
coverage is computed by `.github/actions/backend-integration-patch-coverage/`. Read the thresholds there rather than
assuming a single repo-wide number.
